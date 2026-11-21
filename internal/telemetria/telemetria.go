// Pacote telemetria: liga o OpenTelemetry (traces, métricas e logs) e manda tudo para o Collector via OTLP.
// O endereço vem das variáveis padrão do OTel (OTEL_EXPORTER_OTLP_ENDPOINT etc.).
package telemetria

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const Servico = "vida-da-bruna-api"

// Buckets de latência (segundos). O 0.3 existe de propósito: é o limite do SLO de latência (95% abaixo de 300 ms).
var bucketsLatencia = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.3, 0.5, 1, 2.5, 5, 10}

// Iniciar configura os três sinais. Devolve o logger da aplicação e a função de desligar (que esvazia os buffers).
func Iniciar(ctx context.Context) (*slog.Logger, func(context.Context) error, error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(Servico),
		semconv.ServiceVersion(versao()),
	))
	if err != nil {
		return nil, nil, err
	}

	// Traces
	expTrace, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(expTrace), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Métricas (exportadas a cada 10 s)
	expMetric, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(expMetric)),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http.server.request.duration"},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: bucketsLatencia}},
		)),
	)
	otel.SetMeterProvider(mp)

	// Logs: vão para o terminal (JSON) e para o Loki (via Collector), os dois com trace_id.
	expLog, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	lp := sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewBatchProcessor(expLog)))
	log := slog.New(duplo{
		comTrace{slog.NewJSONHandler(os.Stdout, nil)},
		otelslog.NewHandler(Servico, otelslog.WithLoggerProvider(lp)),
	})

	desligar := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}
	return log, desligar, nil
}

func versao() string {
	if v := os.Getenv("VERSAO"); v != "" {
		return v
	}
	return "dev"
}

// comTrace acrescenta trace_id e span_id aos logs do terminal quando a chamada veio de uma requisição rastreada.
type comTrace struct{ slog.Handler }

func (h comTrace) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}
func (h comTrace) WithAttrs(a []slog.Attr) slog.Handler { return comTrace{h.Handler.WithAttrs(a)} }
func (h comTrace) WithGroup(n string) slog.Handler      { return comTrace{h.Handler.WithGroup(n)} }

// duplo manda cada log para dois destinos.
type duplo [2]slog.Handler

func (d duplo) Enabled(ctx context.Context, l slog.Level) bool {
	return d[0].Enabled(ctx, l) || d[1].Enabled(ctx, l)
}
func (d duplo) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range d {
		if h.Enabled(ctx, r.Level) {
			errs = append(errs, h.Handle(ctx, r.Clone()))
		}
	}
	return errors.Join(errs...)
}
func (d duplo) WithAttrs(a []slog.Attr) slog.Handler {
	return duplo{d[0].WithAttrs(a), d[1].WithAttrs(a)}
}
func (d duplo) WithGroup(n string) slog.Handler { return duplo{d[0].WithGroup(n), d[1].WithGroup(n)} }
