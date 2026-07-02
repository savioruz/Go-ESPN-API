package nhl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-espn-api/infras/otel"
)

type noopScope struct{}

func (noopScope) End()                         {}
func (noopScope) TraceError(error)             {}
func (noopScope) TraceIfError(error)           {}
func (noopScope) AddEvent(string)              {}
func (noopScope) SetAttribute(string, any)     {}
func (noopScope) SetAttributes(map[string]any) {}

type noopOtel struct{}

func (noopOtel) NewScope(ctx context.Context, _, _ string) (context.Context, otel.Scope) {
	return ctx, noopScope{}
}

func newTestClient(base string) *nhlImpl {
	return &nhlImpl{
		otel:        noopOtel{},
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		webBaseURL:  base,
		statBaseURL: base,
		maxRetries:  3,
		backoffMin:  time.Millisecond,
		backoffMax:  2 * time.Millisecond,
	}
}

func TestGetStandingsSuccess(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"standings":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.GetStandings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/v1/standings/now" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if string(resp.Data) != `{"standings":[]}` {
		t.Fatalf("unexpected body %s", resp.Data)
	}
}

func TestRetryThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GetTeams(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestGiveUpAfterN(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetRoster(context.Background(), "TOR")
	if !errors.Is(err, ErrClient) {
		t.Fatalf("expected ErrClient, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestSkaterSummaryParams(t *testing.T) {
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GetSkaterSummary(context.Background(), "20232024", -1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rawQuery, "seasonId%3D20232024") || !strings.Contains(rawQuery, "limit=-1") {
		t.Fatalf("unexpected query %q", rawQuery)
	}
}

func TestBackoffRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.backoffMin = time.Hour
	c.backoffMax = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetStandings(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
