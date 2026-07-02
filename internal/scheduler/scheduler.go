// Package scheduler runs the ESPN ingestion jobs in-process on fixed intervals,
// replacing the Python Celery-beat + worker fan-out. Each job walks the
// configured (sport, league) units through a bounded worker pool so concurrent
// DB writes stay within the Postgres connection pool (the "too many clients"
// fix). Jobs are panic-safe and error-isolated: a failure or panic in one unit
// is logged and counted but never aborts the rest of the job, and a panic in a
// job never kills the scheduler — matching Celery's per-task isolation.
package scheduler

import (
	"context"
	"time"

	"go-espn-api/config"
	"go-espn-api/infras/otel"
	eventRepo "go-espn-api/internal/domains/event/repository"
	"go-espn-api/internal/ingest"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	// jobTimeout bounds a single job tick. It mirrors Celery's task time limit
	// (CELERY_TASK_TIME_LIMIT = 30 min) generously; the real safeguard is the
	// bounded fan-out plus the scheduler's base context, which is cancelled on
	// shutdown.
	jobTimeout = 25 * time.Minute

	// News fetch limit per league, matching refresh_news_task's default.
	newsLimit = 50

	// unstick lookback window / cap, matching unstick_scoreboards_task defaults.
	unstickLookbackDays = 7
	unstickMaxEvents    = 200

	// bucketsPerEvent is the number of ESPN date buckets a stuck event maps to
	// (its UTC date AND the day before, for ET-bucketing self-heal).
	bucketsPerEvent = 2
)

// Job intervals — ported 1:1 from CELERY_BEAT_SCHEDULE in base.py.
const (
	scoreboardsInterval  = 5 * time.Minute
	unstickInterval      = 10 * time.Minute
	newsInterval         = 30 * time.Minute
	injuriesInterval     = 4 * time.Hour
	transactionsInterval = 6 * time.Hour
	teamsInterval        = 7 * 24 * time.Hour
)

// NHL job intervals. Django had no beat schedule for NHL, so these are chosen to
// match how each dataset changes: the team list is near-static (weekly), rosters
// churn day-to-day (daily), and standings move after every game night (hourly).
const (
	nhlTeamsInterval     = 7 * 24 * time.Hour
	nhlRostersInterval   = 24 * time.Hour
	nhlStandingsInterval = 1 * time.Hour
)

// --- narrow ingest interfaces (so jobs are unit-testable with fakes) ---------

type scoreboardIngestor interface {
	IngestScoreboard(ctx context.Context, sport, league, date string) (ingest.IngestionResult, error)
}

type teamsIngestor interface {
	IngestTeams(ctx context.Context, sport, league string) (ingest.IngestionResult, error)
}

type newsIngestor interface {
	IngestNews(ctx context.Context, sport, league string, limit int) (ingest.IngestionResult, error)
}

type injuriesIngestor interface {
	IngestInjuries(ctx context.Context, sport, league string) (ingest.IngestionResult, error)
}

type transactionsIngestor interface {
	IngestTransactions(ctx context.Context, sport, league string) (ingest.IngestionResult, error)
}

type stuckEventLister interface {
	StuckEvents(ctx context.Context, lookbackDays, limit int) ([]eventRepo.StuckEventRef, error)
}

// nhlSyncer is the shared shape of the three NHL ingest jobs: a single
// no-argument sync returning a result. SyncTeams/SyncRosters/SyncStandings all
// satisfy it.
type nhlSyncer interface {
	sync(ctx context.Context) (ingest.IngestionResult, error)
}

// nhlSyncFunc adapts a plain sync method to nhlSyncer.
type nhlSyncFunc func(ctx context.Context) (ingest.IngestionResult, error)

func (f nhlSyncFunc) sync(ctx context.Context) (ingest.IngestionResult, error) { return f(ctx) }

// deps bundles the ingest layer the jobs drive. The concrete *ingest.* services
// satisfy these interfaces; tests inject fakes.
type deps struct {
	scoreboard   scoreboardIngestor
	teams        teamsIngestor
	news         newsIngestor
	injuries     injuriesIngestor
	transactions transactionsIngestor
	events       stuckEventLister

	// NHL jobs (nil when the NHL service is not wired).
	nhlTeams     nhlSyncer
	nhlRosters   nhlSyncer
	nhlStandings nhlSyncer
}

// Scheduler owns the ingest jobs and the parsed run configuration.
type Scheduler struct {
	otel        otel.Otel
	deps        deps
	leagues     []config.IngestLeague
	concurrency int
}

// New constructs a Scheduler from config and the concrete ingest services. It
// parses INGEST_LEAGUES and clamps the concurrency to a sane minimum.
func New(
	cfg *config.Config,
	otl otel.Otel,
	scoreboard *ingest.ScoreboardService,
	teams *ingest.TeamsService,
	news *ingest.NewsService,
	injuries *ingest.InjuriesService,
	transactions *ingest.TransactionsService,
	events eventRepo.Event,
	nhlTeams *ingest.NHLTeamsService,
	nhlRosters *ingest.NHLRostersService,
	nhlStandings *ingest.NHLStandingsService,
) *Scheduler {
	return newScheduler(cfg, otl, deps{
		scoreboard:   scoreboard,
		teams:        teams,
		news:         news,
		injuries:     injuries,
		transactions: transactions,
		events:       events,
		nhlTeams:     nhlSyncFunc(nhlTeams.SyncTeams),
		nhlRosters:   nhlSyncFunc(nhlRosters.SyncRosters),
		nhlStandings: nhlSyncFunc(nhlStandings.SyncStandings),
	})
}

// newScheduler is the shared constructor used by New and by tests (which inject
// fake deps).
func newScheduler(cfg *config.Config, otl otel.Otel, d deps) *Scheduler {
	concurrency := cfg.Services.ESPN.IngestConcurrency
	if concurrency < 1 {
		concurrency = 1
	}

	return &Scheduler{
		otel:        otl,
		deps:        d,
		leagues:     config.ParseIngestLeagues(cfg.Services.ESPN.IngestLeagues),
		concurrency: concurrency,
	}
}

// Register wires the six ESPN jobs onto the given cron with their base context.
// Jobs are NOT run immediately on boot (matching beat); cron fires them on the
// first interval boundary. Each job body is wrapped so a panic is recovered and
// logged instead of killing the scheduler.
func (s *Scheduler) Register(ctx context.Context, c *cron.Cron) {
	s.schedule(ctx, c, "scoreboards", scoreboardsInterval, s.runScoreboards)
	s.schedule(ctx, c, "unstick", unstickInterval, s.runUnstick)
	s.schedule(ctx, c, "news", newsInterval, s.runNews)
	s.schedule(ctx, c, "injuries", injuriesInterval, s.runInjuries)
	s.schedule(ctx, c, "transactions", transactionsInterval, s.runTransactions)
	s.schedule(ctx, c, "teams", teamsInterval, s.runTeams)
}

// RegisterNHL wires the three NHL jobs onto the given cron with their base
// context. Like Register, jobs are not run on boot; cron fires them on the first
// interval boundary, and each body is panic-isolated. These are single-league
// jobs (no league fan-out); rosters fans out over active teams inside the
// service, bounded by SERVICES_NHL_INGEST_CONCURRENCY.
func (s *Scheduler) RegisterNHL(ctx context.Context, c *cron.Cron) {
	s.schedule(ctx, c, "nhl_teams", nhlTeamsInterval, s.runNHLTeams)
	s.schedule(ctx, c, "nhl_rosters", nhlRostersInterval, s.runNHLRosters)
	s.schedule(ctx, c, "nhl_standings", nhlStandingsInterval, s.runNHLStandings)
}

// RunOnceESPN runs each ESPN ingest job a single time, in dependency order
// (teams first so competitors link to existing teams), then returns. Intended
// for back-fill / cutover, not the periodic cron. `unstick` is skipped — it is a
// self-heal for an already-populated system, not initial ingestion.
func (s *Scheduler) RunOnceESPN(ctx context.Context) {
	s.runJob(ctx, "teams", s.runTeams)
	s.runJob(ctx, "scoreboards", s.runScoreboards)
	s.runJob(ctx, "news", s.runNews)
	s.runJob(ctx, "injuries", s.runInjuries)
	s.runJob(ctx, "transactions", s.runTransactions)
}

// RunOnceNHL runs each NHL ingest job a single time (teams → rosters →
// standings, so players and standings resolve their team), then returns.
func (s *Scheduler) RunOnceNHL(ctx context.Context) {
	s.runJob(ctx, "nhl_teams", s.runNHLTeams)
	s.runJob(ctx, "nhl_rosters", s.runNHLRosters)
	s.runJob(ctx, "nhl_standings", s.runNHLStandings)
}

func (s *Scheduler) schedule(ctx context.Context, c *cron.Cron, name string, every time.Duration, run func(context.Context) jobSummary) {
	c.Schedule(cron.Every(every), cron.FuncJob(func() {
		s.runJob(ctx, name, run)
	}))
}

// runJob applies a per-job timeout and recovers panics so a single misbehaving
// tick can never bring down the scheduler.
func (s *Scheduler) runJob(ctx context.Context, name string, run func(context.Context) jobSummary) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("job", name).Interface("panic", r).Msg("scheduler job panicked")
		}
	}()

	if ctx.Err() != nil {
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	log.Info().Str("job", name).Msg("scheduler job started")
	run(jobCtx)
}

// --- jobs --------------------------------------------------------------------

// workUnit is one ingest invocation (a league, optionally scoped to a date).
type workUnit struct {
	sport  string
	league string
	date   string
}

// runScoreboards re-ingests every configured league for today and yesterday
// (UTC). Yesterday covers ESPN's ET date bucketing and late finishers, so live
// games advance to `final`. Fan-out is exactly len(leagues)*2 units.
func (s *Scheduler) runScoreboards(ctx context.Context) jobSummary {
	now := time.Now().UTC()
	dates := []string{now.Format("20060102"), now.AddDate(0, 0, -1).Format("20060102")}

	units := make([]workUnit, 0, len(s.leagues)*len(dates))
	for _, lg := range s.leagues {
		for _, d := range dates {
			units = append(units, workUnit{sport: lg.Sport, league: lg.League, date: d})
		}
	}

	return s.fanOut(ctx, "scoreboards", units, func(ctx context.Context, u workUnit) (ingest.IngestionResult, error) {
		return s.deps.scoreboard.IngestScoreboard(ctx, u.sport, u.league, u.date)
	})
}

// runUnstick re-ingests scoreboards for games past kickoff yet still stuck
// scheduled/in_progress (their ESPN date bucket fell outside the rolling
// today+yesterday window). Each stuck event contributes two buckets — its UTC
// date AND the day before (ET bucketing self-heal) — deduped so multiple stuck
// games in the same league/day enqueue a single re-ingest.
func (s *Scheduler) runUnstick(ctx context.Context) jobSummary {
	refs, err := s.deps.events.StuckEvents(ctx, unstickLookbackDays, unstickMaxEvents)
	if err != nil {
		log.Error().Err(err).Str("job", "unstick").Msg("failed to query stuck events")

		return jobSummary{errors: 1}
	}

	seen := make(map[workUnit]struct{}, len(refs)*bucketsPerEvent)
	units := make([]workUnit, 0, len(refs)*bucketsPerEvent)

	for _, r := range refs {
		d := r.Date.UTC()
		for _, day := range []time.Time{d, d.AddDate(0, 0, -1)} {
			u := workUnit{sport: r.SportSlug, league: r.LeagueSlug, date: day.Format("20060102")}
			if _, ok := seen[u]; ok {
				continue
			}

			seen[u] = struct{}{}
			units = append(units, u)
		}
	}

	log.Info().Str("job", "unstick").Int("stuck_events", len(refs)).Int("buckets", len(units)).Msg("unstick fan-out")

	return s.fanOut(ctx, "unstick", units, func(ctx context.Context, u workUnit) (ingest.IngestionResult, error) {
		return s.deps.scoreboard.IngestScoreboard(ctx, u.sport, u.league, u.date)
	})
}

func (s *Scheduler) runNews(ctx context.Context) jobSummary {
	return s.fanOut(ctx, "news", s.leagueUnits(), func(ctx context.Context, u workUnit) (ingest.IngestionResult, error) {
		return s.deps.news.IngestNews(ctx, u.sport, u.league, newsLimit)
	})
}

func (s *Scheduler) runInjuries(ctx context.Context) jobSummary {
	return s.fanOut(ctx, "injuries", s.leagueUnits(), func(ctx context.Context, u workUnit) (ingest.IngestionResult, error) {
		return s.deps.injuries.IngestInjuries(ctx, u.sport, u.league)
	})
}

func (s *Scheduler) runTransactions(ctx context.Context) jobSummary {
	return s.fanOut(ctx, "transactions", s.leagueUnits(), func(ctx context.Context, u workUnit) (ingest.IngestionResult, error) {
		return s.deps.transactions.IngestTransactions(ctx, u.sport, u.league)
	})
}

func (s *Scheduler) runTeams(ctx context.Context) jobSummary {
	return s.fanOut(ctx, "teams", s.leagueUnits(), func(ctx context.Context, u workUnit) (ingest.IngestionResult, error) {
		return s.deps.teams.IngestTeams(ctx, u.sport, u.league)
	})
}

// runNHLTeams / runNHLRosters / runNHLStandings are single-shot jobs: they call
// one NHL sync and tally its result. Roster fan-out (over active teams) happens
// inside the service, so there is no league fan-out here.
func (s *Scheduler) runNHLTeams(ctx context.Context) jobSummary {
	return s.runNHLJob(ctx, "nhl_teams", s.deps.nhlTeams)
}

func (s *Scheduler) runNHLRosters(ctx context.Context) jobSummary {
	return s.runNHLJob(ctx, "nhl_rosters", s.deps.nhlRosters)
}

func (s *Scheduler) runNHLStandings(ctx context.Context) jobSummary {
	return s.runNHLJob(ctx, "nhl_standings", s.deps.nhlStandings)
}

// runNHLJob runs one NHL sync and logs a per-job summary. A returned error is
// counted (not propagated); the surrounding runJob already recovers panics.
func (s *Scheduler) runNHLJob(ctx context.Context, name string, syncer nhlSyncer) jobSummary {
	var sum jobSummary

	if syncer == nil {
		return sum
	}

	res, err := syncer.sync(ctx)
	if err != nil {
		sum.errors = 1

		log.Error().Err(err).Str("job", name).Msg("nhl ingest failed")

		return sum
	}

	sum.created = int64(res.Created)
	sum.updated = int64(res.Updated)
	sum.errors = int64(res.Errors)

	log.Info().
		Str("job", name).
		Int64("created", sum.created).
		Int64("updated", sum.updated).
		Int64("errors", sum.errors).
		Msg("scheduler job completed")

	return sum
}

// leagueUnits returns one work unit per configured league (no date scope).
func (s *Scheduler) leagueUnits() []workUnit {
	units := make([]workUnit, 0, len(s.leagues))
	for _, lg := range s.leagues {
		units = append(units, workUnit{sport: lg.Sport, league: lg.League})
	}

	return units
}
