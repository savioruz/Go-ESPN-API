package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-espn-api/config"
	"go-espn-api/infras/otel"

	"github.com/go-chi/chi/v5"
	oteltrace "go.opentelemetry.io/otel/trace"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// sdkOtel is a minimal otel.Otel backed by a real SDK TracerProvider. It mirrors
// infras/otel.otelImpl.NewScope so the child-span trace-id inheritance is actually
// exercised (otelImpl itself is unexported and cannot be built from this package).
type sdkOtel struct {
	tp *sdktrace.TracerProvider
}

func (o sdkOtel) NewScope(ctx context.Context, scopeName, spanName string) (context.Context, otel.Scope) {
	ctx, span := o.tp.Tracer(scopeName).Start(ctx, spanName)

	return ctx, otel.NewScope(span)
}

// TestTracingPropagationContinuesTraceparent verifies the Tracing middleware
// extracts an incoming W3C traceparent header and continues the caller's trace
// rather than starting an orphan root trace.
func TestTracingPropagationContinuesTraceparent(t *testing.T) {
	// Register the W3C propagator (as infras/otel.New does in production).
	otelapi.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	const wantTraceID = "0af7651916cd43dd8448eb211c80319c"

	mw := &appMiddleware{
		otel:   sdkOtel{tp: sdktrace.NewTracerProvider()},
		config: &config.Config{},
	}
	mw.config.App.Name = "test"

	var gotTraceID string

	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTraceID = oteltrace.SpanContextFromContext(r.Context()).TraceID().String()
	})

	// Use a real chi router so the RouteContext (and Routes) is populated before
	// the Tracing middleware runs.
	router := chi.NewRouter()
	router.Use(mw.Tracing)
	router.Get("/ping", handler)

	req := httptest.NewRequest(http.MethodGet, "/ping", http.NoBody)
	req.Header.Set("traceparent", "00-"+wantTraceID+"-b7ad6b7169203331-01")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if gotTraceID != wantTraceID {
		t.Fatalf("expected handler span to continue caller trace %s, got %s", wantTraceID, gotTraceID)
	}
}
