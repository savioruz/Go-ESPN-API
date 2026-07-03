// Package mocks provides mock implementations for otel testing.
//
//nolint:revive
package mocks

import (
	"context"
	"go-espn-api/infras/otel"
)

type otelImpl struct {
}

// NewScope implements otel.Otel.
func (o *otelImpl) NewScope(ctx context.Context, _, _ string) (context.Context, otel.Scope) {
	return ctx, NewScope()
}

// NewOtel creates a new mock Otel instance.
func NewOtel() otel.Otel {
	return &otelImpl{}
}
