package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/aovivo"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
)

// Máximo de itens observados numa conexão só.
const maxItens = 50

// item é uma coisa observada: uma coleção inteira ("eventos") ou um documento ("config/main").
type item struct {
	chave   string // como veio na URL, devolvido ao cliente em "w"
	colecao string
	id      string // vazio = coleção inteira
}

// streamMulti: GET /v1/stream?w=eventos&w=config/main
// Uma conexão SSE só para tudo o que a tela observa. Navegadores limitam 6 conexões HTTP/1.1 por origem:
// com um stream por coleção, o app travava. Cada evento é {"w": "<item>", "v": <dados>}.
func (a *api) streamMulti(w http.ResponseWriter, r *http.Request) {
	itens, ok := lerItens(w, r)
	if !ok {
		return
	}
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ctx := r.Context()

	// Uma assinatura por coleção (sem repetir), todas desaguando num canal só.
	avisos := make(chan aovivo.Mudanca, 16)
	vistas := map[string]bool{}
	for _, it := range itens {
		if vistas[it.colecao] {
			continue
		}
		vistas[it.colecao] = true
		c, cancelar := a.hub.Assinar(ctx, it.colecao)
		defer cancelar()
		go func() {
			for {
				select {
				case m := <-c:
					select {
					case avisos <- m:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	enviar := func(it item) error {
		lctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		v, err := a.lerItem(lctx, it)
		if err != nil {
			a.log.WarnContext(ctx, "stream: falha ao ler", "item", it.chave, "err", err)
			return nil // não derruba a conexão; o próximo aviso tenta de novo
		}
		b, _ := json.Marshal(map[string]any{"w": it.chave, "v": v})
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		return rc.Flush()
	}

	w.WriteHeader(http.StatusOK)
	for _, it := range itens {
		if enviar(it) != nil {
			return
		}
	}
	tic := time.NewTicker(batimento)
	defer tic.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-avisos:
			for _, it := range itens {
				// Reenvia só o que foi afetado: a coleção inteira, ou o documento que mudou.
				if it.colecao == m.Colecao && (it.id == "" || m.ID == "" || it.id == m.ID) {
					if enviar(it) != nil {
						return
					}
				}
			}
		case <-tic.C:
			if _, err := fmt.Fprint(w, ": vivo\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

func (a *api) lerItem(ctx context.Context, it item) (any, error) {
	if it.id == "" {
		lista, err := a.docs.Listar(ctx, it.colecao)
		if err != nil {
			return nil, err
		}
		saida := make([]map[string]any, 0, len(lista))
		for _, d := range lista {
			saida = append(saida, comID(d))
		}
		return saida, nil
	}
	d, err := a.docs.Obter(ctx, it.colecao, it.id)
	if errors.Is(err, docs.ErrNaoEncontrado) {
		return nil, nil // documento não existe: o cliente recebe null
	}
	if err != nil {
		return nil, err
	}
	return comID(d), nil
}

func lerItens(w http.ResponseWriter, r *http.Request) ([]item, bool) {
	brutos := r.URL.Query()["w"]
	if len(brutos) == 0 || len(brutos) > maxItens {
		responder(w, http.StatusBadRequest, map[string]string{"erro": fmt.Sprintf("informe de 1 a %d parâmetros w", maxItens)})
		return nil, false
	}
	itens := make([]item, 0, len(brutos))
	for _, b := range brutos {
		col, id, temID := strings.Cut(b, "/")
		if !nomeValido.MatchString(col) || (temID && !nomeValido.MatchString(id)) {
			responder(w, http.StatusBadRequest, map[string]string{"erro": "item inválido: " + b})
			return nil, false
		}
		itens = append(itens, item{chave: b, colecao: col, id: id})
	}
	return itens, true
}
