package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
)

type bancoFalso struct{ err error }

func (b bancoFalso) Ping(context.Context) error { return b.err }

// memoria é um repositório em memória, só para testar as rotas sem Postgres.
type memoria map[string]map[string]any

func (m memoria) chave(col, id string) string { return col + "/" + id }

func (m memoria) Listar(_ context.Context, col string) ([]docs.Doc, error) {
	var out []docs.Doc
	for k, v := range m {
		if strings.HasPrefix(k, col+"/") {
			out = append(out, docs.Doc{ID: strings.TrimPrefix(k, col+"/"), Dados: v})
		}
	}
	return out, nil
}

func (m memoria) Obter(_ context.Context, col, id string) (docs.Doc, error) {
	v, ok := m[m.chave(col, id)]
	if !ok {
		return docs.Doc{}, docs.ErrNaoEncontrado
	}
	return docs.Doc{ID: id, Dados: v}, nil
}

func (m memoria) Gravar(_ context.Context, col, id string, d map[string]any) error {
	m[m.chave(col, id)] = d
	return nil
}

func (m memoria) Mesclar(_ context.Context, col, id string, d map[string]any) error {
	v, ok := m[m.chave(col, id)]
	if !ok {
		return docs.ErrNaoEncontrado
	}
	for k, x := range d {
		v[k] = x
	}
	return nil
}

func (m memoria) Apagar(_ context.Context, col, id string) error {
	delete(m, m.chave(col, id))
	return nil
}

var silencioso = slog.New(slog.NewTextHandler(io.Discard, nil))

func pedir(h http.Handler, metodo, rota, corpo string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(metodo, rota, strings.NewReader(corpo)))
	return rec
}

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
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := pedir(New(silencioso, bancoFalso{c.banco}, memoria{}), http.MethodGet, c.rota, "")
			if rec.Code != c.status {
				t.Fatalf("status = %d, quero %d", rec.Code, c.status)
			}
		})
	}
}

func TestDocumentos(t *testing.T) {
	h := New(silencioso, bancoFalso{}, memoria{})

	passos := []struct {
		metodo, rota, corpo string
		status              int
	}{
		{"GET", "/v1/eventos/x", "", 404},
		{"PUT", "/v1/eventos/x", `{"titulo":"Piano","feito":false,"id":"ignorado"}`, 204},
		{"PATCH", "/v1/eventos/x", `{"feito":true}`, 204},
		{"PATCH", "/v1/eventos/nao-existe", `{"feito":true}`, 404},
		{"PUT", "/v1/eventos/x", `[1,2]`, 400},
		{"PUT", "/v1/eventos/x", `nem json`, 400},
		{"GET", "/v1/eventos!/x", "", 400},
		{"DELETE", "/v1/eventos/y", "", 204},
	}
	for _, p := range passos {
		if rec := pedir(h, p.metodo, p.rota, p.corpo); rec.Code != p.status {
			t.Fatalf("%s %s: status = %d, quero %d (%s)", p.metodo, p.rota, rec.Code, p.status, rec.Body)
		}
	}

	rec := pedir(h, "GET", "/v1/eventos/x", "")
	var doc map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &doc)
	if doc["id"] != "x" || doc["titulo"] != "Piano" || doc["feito"] != true {
		t.Fatalf("documento = %v", doc)
	}

	rec = pedir(h, "POST", "/v1/eventos", `{"titulo":"Leitura"}`)
	var criado map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &criado)
	if rec.Code != 201 || len(criado["id"]) != 20 {
		t.Fatalf("criar: %d %v", rec.Code, criado)
	}

	rec = pedir(h, "GET", "/v1/eventos", "")
	var lista []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &lista)
	if len(lista) != 2 {
		t.Fatalf("listar: %v", lista)
	}
}
