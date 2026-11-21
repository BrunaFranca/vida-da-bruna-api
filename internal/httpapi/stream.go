package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/aovivo"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
)

// Assinante é o que as rotas de tempo real precisam do hub (aovivo.Hub implementa).
type Assinante interface {
	Assinar(ctx context.Context, col string) (<-chan aovivo.Mudanca, func())
}

// Intervalo do "estou vivo" na conexão SSE (evita proxies fecharem a conexão parada).
var batimento = 25 * time.Second

// streamColecao: GET /v1/{col}/stream. Manda a coleção inteira agora e de novo a cada mudança (o watchCollection do app).
func (a *api) streamColecao(w http.ResponseWriter, r *http.Request) {
	col, ok := caminho(w, r, false)
	if !ok {
		return
	}
	a.stream(w, r, col, func(ctx context.Context, _ aovivo.Mudanca) (any, bool, error) {
		lista, err := a.docs.Listar(ctx, col)
		if err != nil {
			return nil, false, err
		}
		saida := make([]map[string]any, 0, len(lista))
		for _, d := range lista {
			saida = append(saida, comID(d))
		}
		return saida, true, nil
	})
}

// streamDoc: GET /v1/{col}/{id}/stream. Manda o documento (ou null) agora e quando ele mudar (o watchDoc do app).
func (a *api) streamDoc(w http.ResponseWriter, r *http.Request) {
	col, ok := caminho(w, r, true)
	if !ok {
		return
	}
	id := r.PathValue("id")
	a.stream(w, r, col, func(ctx context.Context, m aovivo.Mudanca) (any, bool, error) {
		if m.ID != "" && m.ID != id {
			return nil, false, nil // mudou outro documento da coleção: nada a enviar
		}
		d, err := a.docs.Obter(ctx, col, id)
		if errors.Is(err, docs.ErrNaoEncontrado) {
			return nil, true, nil // documento não existe (ou foi apagado): envia null
		}
		if err != nil {
			return nil, false, err
		}
		return comID(d), true, nil
	})
}

// stream cuida da parte comum: cabeçalhos SSE, assinatura, envio inicial, batimento e encerramento.
// ler devolve (dados, enviar?, erro).
func (a *api) stream(w http.ResponseWriter, r *http.Request, col string,
	ler func(context.Context, aovivo.Mudanca) (any, bool, error),
) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // proxies (nginx) não devem segurar os eventos

	ctx := r.Context()
	avisos, cancelar := a.hub.Assinar(ctx, col)
	defer cancelar()

	enviar := func(m aovivo.Mudanca) error {
		lctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		dados, ok, err := ler(lctx, m)
		if err != nil {
			a.log.WarnContext(ctx, "stream: falha ao ler", "col", col, "err", err)
			// Avisa o cliente sem derrubar a conexão; o próximo aviso tenta de novo.
			_, err = fmt.Fprint(w, "event: erro\ndata: {}\n\n")
			if err == nil {
				err = rc.Flush()
			}
			return err
		}
		if !ok {
			return nil
		}
		b, _ := json.Marshal(dados)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		return rc.Flush()
	}

	w.WriteHeader(http.StatusOK)
	if err := enviar(aovivo.Mudanca{Colecao: col}); err != nil {
		return
	}
	tic := time.NewTicker(batimento)
	defer tic.Stop()
	for {
		select {
		case <-ctx.Done():
			return // cliente fechou
		case m := <-avisos:
			if enviar(m) != nil {
				return
			}
		case <-tic.C:
			if _, err := fmt.Fprint(w, ": vivo\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}
