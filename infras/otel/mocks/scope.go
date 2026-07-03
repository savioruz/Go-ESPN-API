package mocks

import "go-espn-api/infras/otel"

type scopeImpl struct {
}

// AddEvent implements otel.Scope.
func (s *scopeImpl) AddEvent(_ string) {

}

// End implements otel.Scope.
func (s *scopeImpl) End() {

}

// SetAttribute implements otel.Scope.
func (s *scopeImpl) SetAttribute(_ string, _ any) {

}

// SetAttributes implements otel.Scope.
func (s *scopeImpl) SetAttributes(_ map[string]any) {

}

// TraceError implements otel.Scope.
func (s *scopeImpl) TraceError(_ error) {

}

// TraceIfError implements otel.Scope.
func (s *scopeImpl) TraceIfError(_ error) {

}

// NewScope creates a new mock Scope.
func NewScope() otel.Scope {
	return &scopeImpl{}
}
