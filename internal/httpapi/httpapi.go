// Pacote httpapi: rotas HTTP da API.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
)

// Pinger é o que a API precisa do banco para o /readyz. Uma interface pequena deixa testar sem Postgres.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Documentos é o que as rotas /v1 precisam do repositório (docs.Repo implementa).
type Documentos interface {
	Listar(ctx context.Context, col string) ([]docs.Doc, error)
	Obter(ctx context.Context, col, id string) (docs.Doc, error)
	Gravar(ctx context.Context, col, id string, dados map[string]any) error
	Mesclar(ctx context.Context, col, id string, dados map[string]any) error
	Apagar(ctx context.Context, col, id string) error
}

// Nomes de coleção e id: só letras, números, _ e -. Evita lixo e surpresas em logs e URLs.
var nomeValido = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

const limiteCorpo = 1 << 20 // 1 MiB por documento

type api struct {
	log  *slog.Logger
	db   Pinger
	docs Documentos
}

// New monta o roteador com todas as rotas.
func New(log *slog.Logger, db Pinger, d Documentos) http.Handler {
	a := &api{log: log, db: db, docs: d}
	mux := http.NewServeMux()

	// livez: o processo está vivo? Não olha dependências (se olhasse, o orquestrador reiniciaria a API à toa quando o banco cai).
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		responder(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// readyz: pronta para receber tráfego? Aqui sim, checa o banco.
	mux.HandleFunc("GET /readyz", a.readyz)

	mux.HandleFunc("GET /v1/{col}", comTimeout(a.listar))
	mux.HandleFunc("POST /v1/{col}", comTimeout(a.criar))
	mux.HandleFunc("GET /v1/{col}/{id}", comTimeout(a.obter))
	mux.HandleFunc("PUT /v1/{col}/{id}", comTimeout(a.gravar))
	mux.HandleFunc("PATCH /v1/{col}/{id}", comTimeout(a.mesclar))
	mux.HandleFunc("DELETE /v1/{col}/{id}", comTimeout(a.apagar))
	return mux
}

// comTimeout limita cada requisição a 3 s. Com o banco fora, a API responde 500 rápido em vez de ficar pendurada.
// Fica dentro de cada rota (e não em volta do mux) para o mux marcar a rota na requisição original,
// que é a que o OpenTelemetry lê para o label http_route.
func comTimeout(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		h(w, r.WithContext(ctx))
	}
}

func (a *api) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil {
		a.log.WarnContext(r.Context(), "banco indisponível", "err", err)
		responder(w, http.StatusServiceUnavailable, map[string]string{"status": "banco indisponível"})
		return
	}
	responder(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) listar(w http.ResponseWriter, r *http.Request) {
	col, ok := caminho(w, r, false)
	if !ok {
		return
	}
	lista, err := a.docs.Listar(r.Context(), col)
	if err != nil {
		a.falha(w, r, err)
		return
	}
	saida := make([]map[string]any, 0, len(lista))
	for _, d := range lista {
		saida = append(saida, comID(d))
	}
	responder(w, http.StatusOK, saida)
}

func (a *api) obter(w http.ResponseWriter, r *http.Request) {
	col, ok := caminho(w, r, true)
	if !ok {
		return
	}
	d, err := a.docs.Obter(r.Context(), col, r.PathValue("id"))
	if err != nil {
		a.falha(w, r, err)
		return
	}
	responder(w, http.StatusOK, comID(d))
}

func (a *api) criar(w http.ResponseWriter, r *http.Request) {
	col, ok := caminho(w, r, false)
	if !ok {
		return
	}
	dados, ok := lerCorpo(w, r)
	if !ok {
		return
	}
	id := novoID()
	if err := a.docs.Gravar(r.Context(), col, id, dados); err != nil {
		a.falha(w, r, err)
		return
	}
	responder(w, http.StatusCreated, map[string]string{"id": id})
}

func (a *api) gravar(w http.ResponseWriter, r *http.Request) {
	a.escrever(w, r, a.docs.Gravar)
}

func (a *api) mesclar(w http.ResponseWriter, r *http.Request) {
	a.escrever(w, r, a.docs.Mesclar)
}

func (a *api) escrever(w http.ResponseWriter, r *http.Request, op func(context.Context, string, string, map[string]any) error) {
	col, ok := caminho(w, r, true)
	if !ok {
		return
	}
	dados, ok := lerCorpo(w, r)
	if !ok {
		return
	}
	if err := op(r.Context(), col, r.PathValue("id"), dados); err != nil {
		a.falha(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) apagar(w http.ResponseWriter, r *http.Request) {
	col, ok := caminho(w, r, true)
	if !ok {
		return
	}
	if err := a.docs.Apagar(r.Context(), col, r.PathValue("id")); err != nil {
		a.falha(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- utilitários ----------

// caminho valida {col} (e {id}, se comID) e já responde 400 quando inválido.
func caminho(w http.ResponseWriter, r *http.Request, comID bool) (string, bool) {
	col := r.PathValue("col")
	if !nomeValido.MatchString(col) || (comID && !nomeValido.MatchString(r.PathValue("id"))) {
		responder(w, http.StatusBadRequest, map[string]string{"erro": "coleção ou id inválido"})
		return "", false
	}
	return col, true
}

// lerCorpo exige um objeto JSON de até 1 MiB. O campo "id" é ignorado: o id vem da URL.
func lerCorpo(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var dados map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limiteCorpo)).Decode(&dados); err != nil || dados == nil {
		responder(w, http.StatusBadRequest, map[string]string{"erro": "o corpo precisa ser um objeto JSON (até 1 MiB)"})
		return nil, false
	}
	delete(dados, "id")
	return dados, true
}

func (a *api) falha(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, docs.ErrNaoEncontrado) {
		responder(w, http.StatusNotFound, map[string]string{"erro": "não encontrado"})
		return
	}
	a.log.ErrorContext(r.Context(), "erro no banco", "err", err)
	responder(w, http.StatusInternalServerError, map[string]string{"erro": "erro interno"})
}

// comID devolve o documento no formato do app: os dados com o id junto.
func comID(d docs.Doc) map[string]any {
	saida := make(map[string]any, len(d.Dados)+1)
	for k, v := range d.Dados {
		saida[k] = v
	}
	saida["id"] = d.ID
	return saida
}

// novoID gera 20 caracteres aleatórios (a-z, 0-9), no mesmo estilo dos ids do app.
func novoID() string {
	const letras = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letras[int(b[i])%len(letras)]
	}
	return string(b)
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}
