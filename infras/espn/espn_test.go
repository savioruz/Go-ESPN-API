package espn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go-espn-api/infras/otel"
)

// --- noop otel plumbing so we can exercise the client without a collector ---

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

func newTestClient(base string, relays ...string) *espnImpl {
	urls := defaultBaseURLs()
	for d := range urls {
		urls[d] = base
	}

	return &espnImpl{
		otel:       noopOtel{},
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURLs:   urls,
		relays:     relays,
		userAgent:  defaultUserAgent,
		maxRetries: 3,
		backoffMin: time.Millisecond,
		backoffMax: 2 * time.Millisecond,
	}
}

func TestRetryOn5xxThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Get(context.Background(), "/x", DomainSite, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
	if string(resp.Data) != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", resp.Data)
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.Get(context.Background(), "/x", DomainSite, nil); err != nil {
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
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), "/x", DomainSite, nil)
	if !errors.Is(err, ErrClient) {
		t.Fatalf("expected ErrClient, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestNotFound(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), "/missing", DomainSite, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("404 should not retry, got %d calls", got)
	}
}

func TestClientErrorNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), "/bad", DomainSite, nil)
	if !errors.Is(err, ErrClient) {
		t.Fatalf("expected ErrClient, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("400 should not retry, got %d calls", got)
	}
}

func TestBackoffRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.backoffMin = time.Hour // force a long wait so cancellation wins
	c.backoffMax = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Get(ctx, "/x", DomainSite, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRelayRoundRobinAndFailover(t *testing.T) {
	// relayA always fails (500); relayB succeeds. First request starts at
	// relayA, fails over to relayB. Second request starts at relayB directly.
	var aHits, bHits int32

	relayA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&aHits, 1)
		if r.Header.Get(relayHeaderTarget) == "" || r.Header.Get(relayHeaderPath) == "" {
			t.Errorf("relay headers missing: target=%q path=%q", r.Header.Get(relayHeaderTarget), r.Header.Get(relayHeaderPath))
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer relayA.Close()

	relayB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&bHits, 1)
		_, _ = w.Write([]byte(`{"relay":"b"}`))
	}))
	defer relayB.Close()

	c := newTestClient("https://site.api.espn.com", relayA.URL, relayB.URL)

	// First request: cursor -> start at relayA (index 0), fails, then relayB.
	if _, err := c.Get(context.Background(), "/one", DomainSite, nil); err != nil {
		t.Fatalf("request 1 err: %v", err)
	}
	// Second request: start at relayB (index 1), succeeds immediately.
	if _, err := c.Get(context.Background(), "/two", DomainSite, nil); err != nil {
		t.Fatalf("request 2 err: %v", err)
	}

	if got := atomic.LoadInt32(&aHits); got != 1 {
		t.Fatalf("expected relayA hit once, got %d", got)
	}
	if got := atomic.LoadInt32(&bHits); got != 2 {
		t.Fatalf("expected relayB hit twice, got %d", got)
	}
}

func TestRelayExhaustionFallsBackDirect(t *testing.T) {
	var directHit int32
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&directHit, 1)
		_, _ = w.Write([]byte(`{"direct":true}`))
	}))
	defer direct.Close()

	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer relay.Close()

	c := newTestClient(direct.URL, relay.URL)
	if _, err := c.Get(context.Background(), "/x", DomainSite, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&directHit); got != 1 {
		t.Fatalf("expected direct fallback hit once, got %d", got)
	}
}

func TestBuildURLParams(t *testing.T) {
	c := newTestClient("https://example.com")
	got := c.buildURL(DomainSite, "/a/b", map[string]string{"limit": "5"})
	if got != "https://example.com/a/b?limit=5" {
		t.Fatalf("unexpected url: %s", got)
	}
}
