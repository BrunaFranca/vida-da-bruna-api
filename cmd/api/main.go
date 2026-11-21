// Comando api: sobe o servidor HTTP da Vida da Bruna.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/httpapi"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("encerrando com erro", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	// Ctrl+C ou SIGTERM (o Docker/Kubernetes manda SIGTERM ao parar) cancelam o contexto.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL não definida")
	}
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// Migrations na subida: a API só começa a ouvir com o banco no esquema certo.
	if err := migrations.Aplicar(ctx, db); err != nil {
		return err
	}
	log.Info("migrations ok")

	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           httpapi.New(log, db, docs.NovoRepo(db)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	erros := make(chan error, 1)
	go func() {
		log.Info("ouvindo", "addr", srv.Addr)
		erros <- srv.ListenAndServe()
	}()

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
	}

	// Desligamento gracioso: para de aceitar conexões e espera as requisições em andamento.
	log.Info("desligando")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func env(chave, padrao string) string {
	if v := os.Getenv(chave); v != "" {
		return v
	}
	return padrao
}
