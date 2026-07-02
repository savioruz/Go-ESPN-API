package main

import (
	"go-espn-api/config"
	"go-espn-api/di"
	"go-espn-api/shared/logger"

	"github.com/rs/zerolog/log"

	migration "go-espn-api/helper"
)

// @title Go-ESPN-API
// @version 1.0
// @description Go rewrite of the Django espn_service + nhl_service. A drop-in,
// @description DRF-compatible read API for ESPN sports data under /api/v1 and NHL
// @description data under /api/v1/nhl, plus POST /api/v1/ingest/* refresh hooks.
// @description All /api/v1 routes require the X-API-Key header.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description e.g. "Bearer {token}"
// @securityDefinitions.apikey X-API-Key
// @in header
// @name X-API-Key
// @description e.g. "{api_key}"
func main() {
	cfg := config.Get()

	logger.InitLogger()

	logger.SetLogLevel(cfg)

	if cfg.DB.Postgres.AutoMigrate {
		// Run migrations
		err := migration.Up(cfg)
		if err != nil {
			log.Error().Err(err).Msg("failed to run migrations")
		}
	}

	http, err := di.InitializeService()
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize service")

		return
	}

	http.Serve()
}
