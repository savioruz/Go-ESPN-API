package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-espn-api/config"
	eventRepo "go-espn-api/internal/domains/event/repository"
	"go-espn-api/internal/ingest"

	"github.com/robfig/cron/v3"
)

// --- fakes -------------------------------------------------------------------

// fakeScoreboard records every IngestScoreboard call and tracks the peak number
// of concurrent in-flight calls so tests can assert the bounded worker pool.
type fakeScoreboard struct {
	mu       sync.Mutex
	calls    []workUnit
	inFlight int64
	maxSeen  int64
	delay    time.Duration
	// panicOn triggers a panic when the unit's date matches, to test isolation.
	panicOnDate string
}

func (f *fakeScoreboard) IngestScoreboard(_ context.Context, sport, league, date string) (ingest.IngestionResult, error) {
	cur := atomic.AddInt64(&f.inFlight, 1)
	for {
		prev := atomic.LoadInt64(&f.maxSeen)
		if cur <= prev || atomic.CompareAndSwapInt64(&f.maxSeen, prev, cur) {
			break
		}
	}
	defer atomic.AddInt64(&f.inFlight, -1)

	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	f.mu.Lock()
	f.calls = append(f.calls, workUnit{sport: sport, league: league, date: date})
	f.mu.Unlock()

	if f.panicOnDate != "" && date == f.panicOnDate {
		panic("boom")
	}

	return ingest.IngestionResult{Created: 1}, nil
}

func (f *fakeScoreboard) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.calls)
}

type fakeStuckLister struct {
	refs []eventRepo.StuckEventRef
	err  error
}

func (f *fakeStuckLister) StuckEvents(_ context.Context, _, _ int) ([]eventRepo.StuckEventRef, error) {
	return f.refs, f.err
}

// testScheduler builds a Scheduler wired only with the pieces a test needs.
func testScheduler(t *testing.T, cfg *config.Config, d deps) *Scheduler {
	t.Helper()

	return newScheduler(cfg, nil, d)
}

func cfgWith(leaguesRaw []string, concurrency int) *config.Config {
	cfg := &config.Config{}
	cfg.Services.ESPN.IngestLeagues = leaguesRaw
	cfg.Services.ESPN.IngestConcurrency = concurrency

	return cfg
}

// --- (a) INGEST_LEAGUES parsing ---------------------------------------------

func TestParseIngestLeagues(t *testing.T) {
	// Custom string, including whitespace and a malformed entry that must be
	// dropped (mirrors _parse_ingest_leagues).
	got := config.ParseIngestLeagues([]string{" soccer:fifa.world ", "football:nfl", "garbage", "basketball: nba ", ":x", "y:"})
	want := []config.IngestLeague{
		{Sport: "soccer", League: "fifa.world"},
		{Sport: "football", League: "nfl"},
		{Sport: "basketball", League: "nba"},
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d pairs, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pair %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseIngestLeaguesDefault(t *testing.T) {
	// The default list is a struct-tag literal; verify the parser handles it and
	// yields the expected 12 base.py pairs.
	def := []string{
		"soccer:fifa.world", "soccer:eng.1", "soccer:esp.1", "soccer:ger.1",
		"soccer:ita.1", "soccer:fra.1", "soccer:usa.1", "soccer:eng.2",
		"soccer:uefa.champions", "soccer:uefa.europa", "football:nfl", "basketball:nba",
	}
	got := config.ParseIngestLeagues(def)
	if len(got) != 12 {
		t.Fatalf("default parsed %d pairs, want 12", len(got))
	}
	if got[0] != (config.IngestLeague{Sport: "soccer", League: "fifa.world"}) {
		t.Fatalf("first pair = %+v", got[0])
	}
	if got[11] != (config.IngestLeague{Sport: "basketball", League: "nba"}) {
		t.Fatalf("last pair = %+v", got[11])
	}
}

// --- (b) scoreboards fan-out count + bounded concurrency --------------------

func TestScoreboardsFanOutCountAndConcurrency(t *testing.T) {
	const concurrency = 2
	leagues := []string{"soccer:fifa.world", "football:nfl", "basketball:nba"}
	fs := &fakeScoreboard{delay: 20 * time.Millisecond}

	s := testScheduler(t, cfgWith(leagues, concurrency), deps{scoreboard: fs})
	s.runScoreboards(context.Background())

	// Fan-out is exactly len(leagues)*2 (today + yesterday).
	if got, want := fs.callCount(), len(leagues)*2; got != want {
		t.Fatalf("scoreboard calls = %d, want %d", got, want)
	}

	// Two distinct dates must have been used.
	dates := map[string]struct{}{}
	fs.mu.Lock()
	for _, c := range fs.calls {
		dates[c.date] = struct{}{}
	}
	fs.mu.Unlock()
	if len(dates) != 2 {
		t.Fatalf("distinct dates = %d, want 2", len(dates))
	}

	// Bounded concurrency: never more than N in flight.
	if peak := atomic.LoadInt64(&fs.maxSeen); peak > concurrency {
		t.Fatalf("peak concurrency = %d, want <= %d", peak, concurrency)
	}
}

// --- (c) unstick bucket dedup ------------------------------------------------

func TestUnstickBucketDedup(t *testing.T) {
	// Two stuck games in the same league/day => one bucket per date, plus date-1.
	day := time.Date(2024, 12, 15, 2, 0, 0, 0, time.UTC)
	lister := &fakeStuckLister{refs: []eventRepo.StuckEventRef{
		{SportSlug: "basketball", LeagueSlug: "nba", Date: day},
		{SportSlug: "basketball", LeagueSlug: "nba", Date: day.Add(3 * time.Hour)}, // same UTC day
	}}
	fs := &fakeScoreboard{}

	s := testScheduler(t, cfgWith(nil, 4), deps{scoreboard: fs, events: lister})
	s.runUnstick(context.Background())

	// Expect exactly 2 buckets: 20241215 and 20241214 (date-1), deduped across
	// the two events.
	got := map[string]struct{}{}
	fs.mu.Lock()
	for _, c := range fs.calls {
		got[c.date] = struct{}{}
	}
	fs.mu.Unlock()

	if len(fs.calls) != 2 {
		t.Fatalf("unstick made %d ingest calls, want 2 (deduped): %+v", len(fs.calls), fs.calls)
	}
	for _, want := range []string{"20241215", "20241214"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("missing bucket %s, got %+v", want, got)
		}
	}
}

// --- (d) panic in one unit doesn't abort the job ----------------------------

func TestPanicIsolation(t *testing.T) {
	leagues := []string{"soccer:fifa.world", "football:nfl", "basketball:nba"}
	// Panic on the "yesterday" bucket only; today's units for all leagues must
	// still complete.
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("20060102")
	fs := &fakeScoreboard{panicOnDate: yesterday}

	s := testScheduler(t, cfgWith(leagues, 4), deps{scoreboard: fs})

	// Must not panic out of the job.
	s.runScoreboards(context.Background())

	// All 6 units were attempted (3 panicked, 3 succeeded) — call recorded before
	// the panic in the fake.
	if got := fs.callCount(); got != len(leagues)*2 {
		t.Fatalf("attempted %d units, want %d", got, len(leagues)*2)
	}
}

// --- registration: exactly 6 jobs -------------------------------------------

func TestRegisterCreatesSixJobs(t *testing.T) {
	s := testScheduler(t, cfgWith([]string{"basketball:nba"}, 4), deps{})
	c := cron.New()
	s.Register(context.Background(), c)

	if got := len(c.Entries()); got != 6 {
		t.Fatalf("registered %d cron entries, want 6", got)
	}
}

// --- NHL registration + job execution ---------------------------------------

func TestRegisterNHLCreatesThreeJobs(t *testing.T) {
	s := testScheduler(t, cfgWith(nil, 4), deps{})
	c := cron.New()
	s.RegisterNHL(context.Background(), c)

	if got := len(c.Entries()); got != 3 {
		t.Fatalf("registered %d NHL cron entries, want 3", got)
	}
}

func TestRegisterESPNAndNHLCreatesNineJobs(t *testing.T) {
	s := testScheduler(t, cfgWith([]string{"basketball:nba"}, 4), deps{})
	c := cron.New()
	s.Register(context.Background(), c)
	s.RegisterNHL(context.Background(), c)

	if got := len(c.Entries()); got != 9 {
		t.Fatalf("registered %d cron entries, want 9 (6 ESPN + 3 NHL)", got)
	}
}

// fakeNHLSyncer records that it ran and returns a fixed result/error.
type fakeNHLSyncer struct {
	calls  int64
	result ingest.IngestionResult
	err    error
}

func (f *fakeNHLSyncer) sync(context.Context) (ingest.IngestionResult, error) {
	atomic.AddInt64(&f.calls, 1)

	return f.result, f.err
}

func TestRunNHLJobTalliesResult(t *testing.T) {
	teams := &fakeNHLSyncer{result: ingest.IngestionResult{Created: 3, Updated: 1, Errors: 2}}
	s := testScheduler(t, cfgWith(nil, 4), deps{nhlTeams: teams})

	sum := s.runNHLTeams(context.Background())
	if atomic.LoadInt64(&teams.calls) != 1 {
		t.Fatalf("nhl teams sync called %d times, want 1", teams.calls)
	}
	if sum.created != 3 || sum.updated != 1 || sum.errors != 2 {
		t.Fatalf("summary = %+v, want created=3 updated=1 errors=2", sum)
	}
}

func TestRunNHLJobCountsError(t *testing.T) {
	rosters := &fakeNHLSyncer{err: errFake}
	s := testScheduler(t, cfgWith(nil, 4), deps{nhlRosters: rosters})

	sum := s.runNHLRosters(context.Background())
	if sum.errors != 1 {
		t.Fatalf("summary errors = %d, want 1", sum.errors)
	}
}

var errFake = fmtError("boom")

type fmtError string

func (e fmtError) Error() string { return string(e) }
