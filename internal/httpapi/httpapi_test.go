package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type bancoFalso struct{ err error }

func (b bancoFalso) Ping(context.Context) error { return b.err }

func TestSaude(t *testing.T) {
	casos := []struct {
		nome   string
		rota   string
		banco  error
		status int
	}{
		{"livez sempre ok", "/livez", nil, http.StatusOK},
		{"livez ok mesmo com banco fora", "/livez", errors.New("fora"), http.StatusOK},
		{"readyz ok com banco", "/readyz", nil, http.StatusOK},
		{"readyz 503 sem banco", "/readyz", errors.New("fora"), http.StatusServiceUnavailable},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := httptest.NewRecorder()
			New(log, bancoFalso{c.banco}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.rota, nil))
			if rec.Code != c.status {
				t.Fatalf("status = %d, quero %d", rec.Code, c.status)
			}
		})
	}
}
