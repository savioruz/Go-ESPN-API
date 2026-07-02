package scheduler

import (
	"context"
	"sync"
	"testing"

	"go-espn-api/internal/ingest"
)

// --- fakes that record call order across the different ingest interfaces -----

// callRecorder captures, thread-safely, the order and per-job count of ingest
// calls so RunOnce tests can assert both "each job ran" and "teams ran first".
type callRecorder struct {
	mu    sync.Mutex
	order []string
	count map[string]int
}

func (r *callRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == nil {
		r.count = map[string]int{}
	}

	r.order = append(r.order, name)
	r.count[name]++
}

func (r *callRecorder) counts() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(map[string]int, len(r.count))
	for k, v := range r.count {
		out[k] = v
	}

	return out
}

func (r *callRecorder) first() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.order) == 0 {
		return ""
	}

	return r.order[0]
}

func (r *callRecorder) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.order)
}

type recTeams struct{ rec *callRecorder }

func (f recTeams) IngestTeams(_ context.Context, _, _ string) (ingest.IngestionResult, error) {
	f.rec.record("teams")

	return ingest.IngestionResult{}, nil
}

type recScoreboard struct{ rec *callRecorder }

func (f recScoreboard) IngestScoreboard(_ context.Context, _, _, _ string) (ingest.IngestionResult, error) {
	f.rec.record("scoreboards")

	return ingest.IngestionResult{}, nil
}

type recNews struct{ rec *callRecorder }

func (f recNews) IngestNews(_ context.Context, _, _ string, _ int) (ingest.IngestionResult, error) {
	f.rec.record("news")

	return ingest.IngestionResult{}, nil
}

type recInjuries struct{ rec *callRecorder }

func (f recInjuries) IngestInjuries(_ context.Context, _, _ string) (ingest.IngestionResult, error) {
	f.rec.record("injuries")

	return ingest.IngestionResult{}, nil
}

type recTransactions struct{ rec *callRecorder }

func (f recTransactions) IngestTransactions(_ context.Context, _, _ string) (ingest.IngestionResult, error) {
	f.rec.record("transactions")

	return ingest.IngestionResult{}, nil
}

// espnRecDeps wires all five ESPN ingest fakes onto a shared recorder.
func espnRecDeps(rec *callRecorder) deps {
	return deps{
		teams:        recTeams{rec},
		scoreboard:   recScoreboard{rec},
		news:         recNews{rec},
		injuries:     recInjuries{rec},
		transactions: recTransactions{rec},
	}
}

// --- RunOnceESPN -------------------------------------------------------------

func TestRunOnceESPNRunsEachJobTeamsFirst(t *testing.T) {
	rec := &callRecorder{}
	s := testScheduler(t, cfgWith([]string{"basketball:nba"}, 4), espnRecDeps(rec))

	s.RunOnceESPN(context.Background())

	// One league: teams/news/injuries/transactions run once each; scoreboards
	// runs twice (today + yesterday). unstick is skipped in one-shot mode.
	want := map[string]int{"teams": 1, "scoreboards": 2, "news": 1, "injuries": 1, "transactions": 1}

	got := rec.counts()
	for name, n := range want {
		if got[name] != n {
			t.Fatalf("%s ran %d times, want %d (all: %+v)", name, got[name], n, got)
		}
	}

	if _, ran := got["unstick"]; ran {
		t.Fatalf("unstick must be skipped in one-shot ingest, but it ran: %+v", got)
	}

	// Jobs run sequentially, so teams (which competitors depend on) must complete
	// before any scoreboard ingest.
	if first := rec.first(); first != "teams" {
		t.Fatalf("first job ran = %q, want teams", first)
	}
}

// --- RunOnceNHL --------------------------------------------------------------

func TestRunOnceNHLRunsThreeSyncs(t *testing.T) {
	teams := &fakeNHLSyncer{}
	rosters := &fakeNHLSyncer{}
	standings := &fakeNHLSyncer{}

	s := testScheduler(t, cfgWith(nil, 4), deps{
		nhlTeams:     teams,
		nhlRosters:   rosters,
		nhlStandings: standings,
	})

	s.RunOnceNHL(context.Background())

	if teams.calls != 1 || rosters.calls != 1 || standings.calls != 1 {
		t.Fatalf("nhl syncs: teams=%d rosters=%d standings=%d, want 1 each", teams.calls, rosters.calls, standings.calls)
	}
}

// --- Worker.RunOnce gating ---------------------------------------------------

func TestWorkerRunOnceRunsEnabledServicesOnly(t *testing.T) {
	// ESPN enabled, NHL disabled: only the ESPN jobs run.
	rec := &callRecorder{}
	nhl := &fakeNHLSyncer{}

	cfg := cfgWith([]string{"basketball:nba"}, 4)
	cfg.Services.ESPN.Enabled = true
	cfg.Services.NHL.Enabled = false

	d := espnRecDeps(rec)
	d.nhlTeams = nhl
	d.nhlRosters = nhl
	d.nhlStandings = nhl

	w := &Worker{Cfg: cfg, Scheduler: testScheduler(t, cfg, d), DB: nil}
	w.RunOnce()

	if got := rec.counts()["teams"]; got != 1 {
		t.Fatalf("ESPN teams ran %d times, want 1", got)
	}

	if nhl.calls != 0 {
		t.Fatalf("NHL syncs ran %d times with NHL disabled, want 0", nhl.calls)
	}
}

func TestWorkerRunOnceNoServicesDoesNothing(t *testing.T) {
	rec := &callRecorder{}

	cfg := cfgWith([]string{"basketball:nba"}, 4)
	cfg.Services.ESPN.Enabled = false
	cfg.Services.NHL.Enabled = false

	w := &Worker{Cfg: cfg, Scheduler: testScheduler(t, cfg, espnRecDeps(rec)), DB: nil}
	w.RunOnce()

	if got := rec.total(); got != 0 {
		t.Fatalf("worker with no services enabled ran %d ingest jobs, want 0", got)
	}
}
