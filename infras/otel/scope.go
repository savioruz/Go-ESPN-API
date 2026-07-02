package otel

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Scope defines the interface for OpenTelemetry span operations.
type Scope interface {
	End()
	TraceError(err error)
	TraceIfError(err error)
	AddEvent(name string)
	SetAttribute(key string, value any)
	SetAttributes(attributes map[string]any)
}

type scopeImpl struct {
	span oteltrace.Span
}

func (s *scopeImpl) End() {
	s.span.End()
}

func (s *scopeImpl) TraceError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

func (s *scopeImpl) TraceIfError(err error) {
	if err != nil {
		s.TraceError(err)
	}
}

func (s *scopeImpl) AddEvent(name string) {
	s.span.AddEvent(name)
}

func (s *scopeImpl) SetAttribute(key string, value any) {
	switch val := value.(type) {
	case bool:
		s.span.SetAttributes(attribute.Bool(key, val))
	case string:
		s.span.SetAttributes(attribute.String(key, val))
	case int:
		s.span.SetAttributes(attribute.Int(key, val))
	case []string:
		s.span.SetAttributes(attribute.StringSlice(key, val))
	default:
		s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", val)))
	}
}

func (s *scopeImpl) SetAttributes(attributes map[string]any) {
	for key, value := range attributes {
		s.SetAttribute(key, value)
	}
}

// NewScope creates a new Scope from an OpenTelemetry span.
func NewScope(span oteltrace.Span) Scope {
	return &scopeImpl{
		span: span,
	}
}
