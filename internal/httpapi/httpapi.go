// Pacote httpapi: rotas HTTP da API.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Pinger é o que a API precisa do banco para o /readyz. Uma interface pequena deixa testar sem Postgres.
type Pinger interface {
	Ping(ctx context.Context) error
}

// New monta o roteador com todas as rotas.
func New(log *slog.Logger, db Pinger) http.Handler {
	mux := http.NewServeMux()

	// livez: o processo está vivo? Não olha dependências (se olhasse, o orquestrador reiniciaria a API à toa quando o banco cai).
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		responder(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// readyz: pronta para receber tráfego? Aqui sim, checa o banco.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			log.Warn("banco indisponível", "err", err)
			responder(w, http.StatusServiceUnavailable, map[string]string{"status": "banco indisponível"})
			return
		}
		responder(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}
