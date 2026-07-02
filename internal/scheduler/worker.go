package scheduler

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-espn-api/config"
	"go-espn-api/infras/postgres"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// defaultDrainSeconds is the fallback in-flight job drain budget when the
// server shutdown periods are unconfigured.
const defaultDrainSeconds = 30

// Worker is the top-level ESPN ingestion process: it owns the cron, the
// scheduler, and the resources that must be released on shutdown. It is built
// by di.InitializeWorker.
type Worker struct {
	Cfg       *config.Config
	Scheduler *Scheduler
	DB        *postgres.Connection
}

// Run registers the ESPN jobs (only when the ESPN service is enabled), starts
// the cron, and blocks until SIGINT/SIGTERM. On shutdown it stops the cron and
// waits — up to a bounded drain timeout — for in-flight jobs to finish before
// closing the DB connections. Jobs are never run immediately on boot (matching
// Celery-beat).
func (w *Worker) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := cron.New()

	espnEnabled := w.Cfg.Services.ESPN.Enabled
	nhlEnabled := w.Cfg.Services.NHL.Enabled

	before := len(c.Entries())
	if espnEnabled {
		w.Scheduler.Register(ctx, c)
	}

	espnJobs := len(c.Entries()) - before
	if nhlEnabled {
		w.Scheduler.RegisterNHL(ctx, c)
	}

	nhlJobs := len(c.Entries()) - before - espnJobs

	if espnEnabled || nhlEnabled {
		log.Info().
			Bool("espn", espnEnabled).
			Bool("nhl", nhlEnabled).
			Int("espn_jobs", espnJobs).
			Int("nhl_jobs", nhlJobs).
			Int("jobs", len(c.Entries())).
			Msg("registered ingest jobs")
	} else {
		log.Info().Msg("no services enabled — no jobs registered, worker idling")
	}

	c.Start()
	log.Info().Msg("scheduler started")

	// Block until interrupt.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown signal received, stopping scheduler")

	// Stop scheduling new runs and cancel the base context so in-flight jobs
	// wind down; then wait for them to drain within the bounded timeout.
	cronCtx := c.Stop()

	cancel()

	drain := w.drainTimeout()
	select {
	case <-cronCtx.Done():
		log.Info().Msg("in-flight jobs drained")
	case <-time.After(drain):
		log.Warn().Dur("timeout", drain).Msg("drain timeout elapsed, forcing shutdown")
	}

	w.closeDB()

	log.Info().Msg("worker shutdown complete")
}

// RunOnce runs every enabled ingest job a single time, closes the DB, and
// returns — the back-fill / cutover entrypoint (`worker --once`). No cron, no
// signal wait. ESPN runs before NHL; within each, teams run first so dependent
// rows (competitors, players, standings) resolve their team.
func (w *Worker) RunOnce() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	espnEnabled := w.Cfg.Services.ESPN.Enabled
	nhlEnabled := w.Cfg.Services.NHL.Enabled

	if !espnEnabled && !nhlEnabled {
		log.Info().Msg("no services enabled — nothing to ingest")

		return
	}

	log.Info().Bool("espn", espnEnabled).Bool("nhl", nhlEnabled).Msg("one-shot ingest starting")

	if espnEnabled {
		w.Scheduler.RunOnceESPN(ctx)
	}

	if nhlEnabled {
		w.Scheduler.RunOnceNHL(ctx)
	}

	w.closeDB()
	log.Info().Msg("one-shot ingest complete")
}

// drainTimeout reuses the HTTP server's graceful-shutdown budget (grace +
// cleanup) as the bound for waiting on in-flight jobs, defaulting to 30s if
// unconfigured.
func (w *Worker) drainTimeout() time.Duration {
	sd := w.Cfg.Server.Shutdown

	seconds := sd.GracePeriodSeconds + sd.CleanupPeriodSeconds
	if seconds <= 0 {
		return defaultDrainSeconds * time.Second
	}

	return time.Duration(seconds) * time.Second
}

func (w *Worker) closeDB() {
	if w.DB == nil {
		return
	}

	if w.DB.Read != nil {
		if err := w.DB.Read.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close read DB connection")
		}
	}

	if w.DB.Write != nil && w.DB.Write != w.DB.Read {
		if err := w.DB.Write.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close write DB connection")
		}
	}
}
