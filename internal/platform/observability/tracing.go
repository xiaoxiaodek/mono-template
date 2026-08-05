package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/vort-ads/vort-ads-template/internal/platform/config"
)

// NewTracerProvider configures an OTLP/HTTP trace pipeline. A nil tracer
// provider is returned when tracing is disabled so callers can skip wiring.
func NewTracerProvider(ctx context.Context, serviceName string, cfg config.TracingConfig) (*sdktrace.TracerProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.SamplingRatio > 0 && cfg.SamplingRatio < 1 {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRatio))
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("create tracer resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)
	// Register global defaults so libraries that resolve a "net/http" tracer
	// participate in the same pipeline and upstream trace context is propagated.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp, nil
}

// ShutdownTracer flushes and stops the tracer provider. It is a no-op for nil.
func ShutdownTracer(ctx context.Context, tp *sdktrace.TracerProvider) error {
	if tp == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return tp.Shutdown(shutdownCtx)
}
