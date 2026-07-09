package repository_test

import (
	"context"
	"strings"
	"testing"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	sportrepo "go-espn-api/internal/domains/sport/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // register the postgres driver for sqlx.Open
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// recorderOtel is a minimal otel.Otel that records spans into an in-memory
// TracerProvider so tests can assert span attributes without a collector.
type recorderOtel struct {
	tp *sdktrace.TracerProvider
}

func (o recorderOtel) NewScope(ctx context.Context, scopeName, spanName string) (context.Context, otel.Scope) {
	ctx, span := o.tp.Tracer(scopeName).Start(ctx, spanName)

	return ctx, otel.NewScope(span)
}

// TestRepositorySpanCarriesQueryAttribute is a regression guard: repository
// spans must attach the executed SQL under constant.OtelQueryAttributeKey
// (matching the oil generic repo), otherwise queries don't show in traces.
//
// The DB is opened then immediately closed so the query fails instantly with no
// network — the attribute is set BEFORE the exec, so the span still records it.
func TestRepositorySpanCarriesQueryAttribute(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	db, err := sqlx.Open("postgres", "postgres://u:p@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Close so any query errors immediately (no connection attempt / timeout).
	_ = db.Close()

	conn := &postgres.Connection{Read: db, Write: db}
	repo := sportrepo.New(conn, recorderOtel{tp: tp})

	// Expected to error against the closed DB; we only care about the span.
	_, _ = repo.List(context.Background(), 1, 25)

	const wantSpan = constant.OtelRepositoryScopeName + ".sport.List"

	var query string

	for _, span := range recorder.Ended() {
		if span.Name() != wantSpan {
			continue
		}

		for _, attr := range span.Attributes() {
			if string(attr.Key) == constant.OtelQueryAttributeKey {
				query = attr.Value.AsString()
			}
		}
	}

	if !strings.Contains(query, "SELECT") || !strings.Contains(query, "FROM sports") {
		t.Fatalf("%s span missing %q attribute with the SQL; got %q",
			wantSpan, constant.OtelQueryAttributeKey, query)
	}
}
