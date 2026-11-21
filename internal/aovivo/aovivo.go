// Pacote aovivo: escuta o canal "docs" do Postgres (LISTEN/NOTIFY) e avisa quem está assinando cada coleção.
package aovivo

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Mudanca diz qual documento mudou.
type Mudanca struct {
	Colecao string `json:"colecao"`
	ID      string `json:"id"`
}

// Hub distribui as mudanças para as assinaturas abertas.
type Hub struct {
	mu        sync.Mutex
	assinadas map[string]map[chan Mudanca]struct{} // coleção → canais
	abertas   metric.Int64UpDownCounter
}

// NovoHub cria o hub. Chame Escutar numa goroutine para ligá-lo ao Postgres.
func NovoHub() *Hub {
	abertas, _ := otel.Meter("aovivo").Int64UpDownCounter("api.streams.abertos",
		metric.WithDescription("Conexões de tempo real (SSE) abertas"))
	return &Hub{assinadas: map[string]map[chan Mudanca]struct{}{}, abertas: abertas}
}

// Assinar devolve um canal que recebe as mudanças da coleção e a função para cancelar.
// O canal tem buffer 1: se chegarem várias mudanças juntas, o cliente recebe um aviso só e relê tudo (coalescência).
func (h *Hub) Assinar(ctx context.Context, col string) (<-chan Mudanca, func()) {
	c := make(chan Mudanca, 1)
	h.mu.Lock()
	if h.assinadas[col] == nil {
		h.assinadas[col] = map[chan Mudanca]struct{}{}
	}
	h.assinadas[col][c] = struct{}{}
	h.mu.Unlock()
	h.abertas.Add(ctx, 1)

	var once sync.Once
	return c, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.assinadas[col], c)
			h.mu.Unlock()
			h.abertas.Add(context.Background(), -1)
		})
	}
}

func (h *Hub) publicar(m Mudanca) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.assinadas[m.Colecao] {
		select {
		case c <- m:
		default: // já tem aviso pendente: o cliente vai reler de qualquer jeito
		}
	}
}

// Escutar mantém uma conexão dedicada com LISTEN docs e se reconecta sozinho se o banco cair.
func (h *Hub) Escutar(ctx context.Context, db *pgxpool.Pool, log *slog.Logger) {
	espera := time.Second
	for ctx.Err() == nil {
		err := h.escutarUmaVez(ctx, db)
		if ctx.Err() != nil {
			return
		}
		log.WarnContext(ctx, "LISTEN caiu; tentando de novo", "err", err, "em", espera)
		select {
		case <-time.After(espera):
		case <-ctx.Done():
			return
		}
		espera = min(espera*2, 30*time.Second) // backoff exponencial
	}
}

func (h *Hub) escutarUmaVez(ctx context.Context, db *pgxpool.Pool) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN docs"); err != nil {
		return err
	}
	// Ao reconectar, avisa todo mundo: pode ter havido mudança enquanto estávamos fora.
	h.avisarTodos()
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var m Mudanca
		if json.Unmarshal([]byte(n.Payload), &m) == nil {
			h.publicar(m)
		}
	}
}

func (h *Hub) avisarTodos() {
	h.mu.Lock()
	cols := make([]string, 0, len(h.assinadas))
	for col := range h.assinadas {
		cols = append(cols, col)
	}
	h.mu.Unlock()
	for _, col := range cols {
		h.publicar(Mudanca{Colecao: col})
	}
}
