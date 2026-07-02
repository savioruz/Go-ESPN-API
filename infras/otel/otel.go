// Package otel provides OpenTelemetry tracing utilities.
//
//nolint:revive
package otel

import (
	"context"
	"go-espn-api/config"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc/credentials/insecure"
)

// Otel defines the interface for OpenTelemetry tracing operations.
type Otel interface {
	NewScope(ctx context.Context, scopeName, spanName string) (context.Context, Scope)
}

type otelImpl struct {
	TracerProvider *trace.TracerProvider
}

func (o *otelImpl) NewScope(ctx context.Context, scopeName, spanName string) (context.Context, Scope) {
	ctx, span := o.TracerProvider.Tracer(scopeName).Start(ctx, spanName)

	return ctx, NewScope(span)
}

// New creates a new Otel instance with the provided configuration.
func New(config *config.Config) Otel {
	ctx := context.Background()

	endpoint := config.External.Otel.Endpoint

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create OTLP exporter")
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.App.Name),
		)),
	)

	// Set tracer provider global
	otel.SetTracerProvider(traceProvider)

	otelInstance := &otelImpl{
		TracerProvider: traceProvider,
	}

	return otelInstance
}
