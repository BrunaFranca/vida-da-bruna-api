// Pacote migrations: aplica os arquivos sql/NNNN_nome.sql em ordem, uma única vez cada.
package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var arquivos embed.FS

// Número qualquer, fixo: identifica a trava das migrations no Postgres.
const trava = 7_102_026

// Aplicar roda as migrations pendentes. É seguro chamar em toda subida da API.
func Aplicar(ctx context.Context, db *pgxpool.Pool) error {
	// Uma conexão só, para a trava e o trabalho ficarem na mesma sessão.
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Advisory lock: se duas réplicas subirem juntas, a segunda espera a primeira terminar.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", trava); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", trava) //nolint:errcheck // a trava cai junto com a sessão de qualquer jeito

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		versao text PRIMARY KEY,
		aplicada_em timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}

	nomes, err := fs.Glob(arquivos, "sql/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(nomes)

	for _, nome := range nomes {
		versao := strings.TrimSuffix(strings.TrimPrefix(nome, "sql/"), ".sql")
		var ja bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE versao = $1)", versao).Scan(&ja); err != nil {
			return err
		}
		if ja {
			continue
		}
		sql, err := arquivos.ReadFile(nome)
		if err != nil {
			return err
		}
		// Cada migration numa transação: ou entra inteira, ou não entra.
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", versao, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (versao) VALUES ($1)", versao); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
