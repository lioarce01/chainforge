// Package otel provides OpenTelemetry tracing wrappers for chainforge interfaces.
// Import this package only when you need distributed tracing; pkg/core has zero
// OTel dependencies.
package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/lioarce01/chainforge"

// InitTracerProvider initialises an OTLP gRPC tracer provider and registers
// it as the global OTel tracer provider. Call the returned shutdown function
// on application exit to flush and close the exporter.
//
//	tp, shutdown, err := otel.InitTracerProvider(ctx, "localhost:4317", "chainforge", "0.3.0")
//	defer shutdown(context.Background())
func InitTracerProvider(ctx context.Context, otlpEndpoint, serviceName, serviceVersion string) (trace.TracerProvider, func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel: create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel: create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("otel: tracer provider shutdown: %w", err)
		}
		return nil
	}

	return tp, shutdown, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer() trace.Tracer {
	return otel.Tracer(instrumentationName)
}
