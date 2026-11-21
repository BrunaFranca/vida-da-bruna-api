// Pacote docs: guarda e lê documentos no Postgres.
package docs

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNaoEncontrado: o documento pedido não existe.
var ErrNaoEncontrado = errors.New("documento não encontrado")

// Doc é um documento: um id e dados livres.
type Doc struct {
	ID    string
	Dados map[string]any
}

// Repo acessa a tabela docs.
type Repo struct{ db *pgxpool.Pool }

// NovoRepo cria o repositório.
func NovoRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

// Listar devolve todos os documentos de uma coleção, ordenados por id.
func (r *Repo) Listar(ctx context.Context, col string) ([]Doc, error) {
	rows, err := r.db.Query(ctx, "SELECT id, dados FROM docs WHERE colecao = $1 ORDER BY id", col)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Doc, error) {
		var d Doc
		err := row.Scan(&d.ID, &d.Dados)
		return d, err
	})
}

// Obter devolve um documento ou ErrNaoEncontrado.
func (r *Repo) Obter(ctx context.Context, col, id string) (Doc, error) {
	d := Doc{ID: id}
	err := r.db.QueryRow(ctx, "SELECT dados FROM docs WHERE colecao = $1 AND id = $2", col, id).Scan(&d.Dados)
	if errors.Is(err, pgx.ErrNoRows) {
		return Doc{}, ErrNaoEncontrado
	}
	return d, err
}

// Gravar cria ou substitui o documento inteiro (o "set" do app).
func (r *Repo) Gravar(ctx context.Context, col, id string, dados map[string]any) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO docs (colecao, id, dados) VALUES ($1, $2, $3)
		ON CONFLICT (colecao, id) DO UPDATE SET dados = EXCLUDED.dados, atualizado_em = now()`,
		col, id, dados)
	return err
}

// Mesclar junta os campos enviados aos que já existem (o "update" do app). Raso: um campo enviado substitui o antigo.
func (r *Repo) Mesclar(ctx context.Context, col, id string, dados map[string]any) error {
	tag, err := r.db.Exec(ctx,
		"UPDATE docs SET dados = dados || $3, atualizado_em = now() WHERE colecao = $1 AND id = $2",
		col, id, dados)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// Apagar remove o documento. Apagar o que não existe não é erro (idempotente).
func (r *Repo) Apagar(ctx context.Context, col, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM docs WHERE colecao = $1 AND id = $2", col, id)
	return err
}
