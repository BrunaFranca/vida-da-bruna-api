package docs_test

// Testes de integração: rodam contra um Postgres de verdade.
// Local: suba o banco (docker compose up -d db) e defina
//   TEST_DATABASE_URL=postgres://vida:vida-local@localhost:5432/vida?sslmode=disable
// Sem a variável, os testes são pulados. No CI ela sempre existe.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/migrations"
)

func repoDeTeste(t *testing.T) (*docs.Repo, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := migrations.Aplicar(ctx, db); err != nil {
		t.Fatal(err)
	}
	// Coleção única por teste: os testes não se atrapalham e não sujam dados de outros.
	col := "teste_" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), "DELETE FROM docs WHERE colecao = $1", col) })
	return docs.NovoRepo(db), col
}

func TestCicloDeVida(t *testing.T) {
	r, col := repoDeTeste(t)
	ctx := context.Background()

	if _, err := r.Obter(ctx, col, "a"); !errors.Is(err, docs.ErrNaoEncontrado) {
		t.Fatalf("antes de gravar: erro = %v, quero ErrNaoEncontrado", err)
	}

	if err := r.Gravar(ctx, col, "a", map[string]any{"titulo": "Piano", "feito": false}); err != nil {
		t.Fatal(err)
	}
	if err := r.Mesclar(ctx, col, "a", map[string]any{"feito": true}); err != nil {
		t.Fatal(err)
	}
	d, err := r.Obter(ctx, col, "a")
	if err != nil {
		t.Fatal(err)
	}
	if d.Dados["titulo"] != "Piano" || d.Dados["feito"] != true {
		t.Fatalf("mesclar deveria manter titulo e trocar feito: %v", d.Dados)
	}

	// Gravar de novo substitui tudo: o campo "feito" some.
	if err := r.Gravar(ctx, col, "a", map[string]any{"titulo": "Leitura"}); err != nil {
		t.Fatal(err)
	}
	d, _ = r.Obter(ctx, col, "a")
	if _, tem := d.Dados["feito"]; tem || d.Dados["titulo"] != "Leitura" {
		t.Fatalf("gravar deveria substituir o documento: %v", d.Dados)
	}

	_ = r.Gravar(ctx, col, "b", map[string]any{})
	lista, err := r.Listar(ctx, col)
	if err != nil || len(lista) != 2 || lista[0].ID != "a" || lista[1].ID != "b" {
		t.Fatalf("listar = %v, %v", lista, err)
	}

	if err := r.Apagar(ctx, col, "a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Apagar(ctx, col, "a"); err != nil {
		t.Fatalf("apagar duas vezes não deveria dar erro: %v", err)
	}
	if err := r.Mesclar(ctx, col, "a", map[string]any{"x": 1}); !errors.Is(err, docs.ErrNaoEncontrado) {
		t.Fatalf("mesclar inexistente: erro = %v, quero ErrNaoEncontrado", err)
	}
}

func TestMigrationsIdempotentes(t *testing.T) {
	_, _ = repoDeTeste(t) // já aplicou uma vez
	db, _ := pgxpool.New(context.Background(), os.Getenv("TEST_DATABASE_URL"))
	defer db.Close()
	if err := migrations.Aplicar(context.Background(), db); err != nil {
		t.Fatalf("aplicar de novo deveria ser no-op: %v", err)
	}
}
