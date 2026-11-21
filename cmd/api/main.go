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

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/docs"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/httpapi"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/migrations"
	"github.com/BrunaFranca/vida-da-bruna-api/internal/telemetria"
)

func main() {
	if err := run(); err != nil {
		slog.Error("encerrando com erro", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Ctrl+C ou SIGTERM (o Docker/Kubernetes manda SIGTERM ao parar) cancelam o contexto.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log, desligarTelemetria, err := telemetria.Iniciar(ctx)
	if err != nil {
		return err
	}
	// Ao sair, esvazia os buffers de telemetria (senão os últimos spans/logs se perdem).
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = desligarTelemetria(c)
	}()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL não definida")
	}
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return err
	}
	// Cada query vira um span filho da requisição.
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
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
		Addr: ":" + env("PORT", "8080"),
		Handler: otelhttp.NewHandler(httpapi.New(log, db, docs.NovoRepo(db)), "api",
			// Health checks ficam fora dos traces e das métricas: são ruído e não entram no SLO.
			otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/livez" && r.URL.Path != "/readyz" }),
		),
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
