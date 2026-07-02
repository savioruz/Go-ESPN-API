// Command worker runs the ESPN ingestion scheduler (the in-process replacement
// for Celery-beat + worker). It loads config, and if no services are enabled it
// idles cleanly; otherwise it builds the ingest worker via Wire and runs the
// cron jobs until SIGINT/SIGTERM.
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go-espn-api/config"
	"go-espn-api/di"
	"go-espn-api/shared/logger"

	"github.com/rs/zerolog/log"
)

func main() {
	once := flag.Bool("once", false, "run every enabled ingest job a single time, then exit (back-fill / cutover)")

	flag.Parse()

	cfg := config.Get()

	logger.InitLogger()
	logger.SetLogLevel(cfg)

	// Distinguish the worker's OTel service name from the HTTP app. otel.New
	// reads App.Name, so set it before the DI graph builds the tracer.
	cfg.App.Name = "espn_worker"

	// If nothing is enabled, don't touch the DB or ESPN client. In --once mode
	// there's nothing to do; otherwise idle so the process stays up (e.g. in a
	// pod) without crashing.
	if !cfg.Services.ESPN.Enabled && !cfg.Services.NHL.Enabled {
		log.Info().Msg("no services enabled — nothing to ingest")

		if !*once {
			idle()
		}

		return
	}

	worker, err := di.InitializeWorker()
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize worker")

		return
	}

	if *once {
		worker.RunOnce()

		return
	}

	worker.Run()
}

// idle blocks until an interrupt so a services-disabled worker exits cleanly on
// signal instead of busy-looping or crashing.
func idle() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown signal received, exiting")
}
