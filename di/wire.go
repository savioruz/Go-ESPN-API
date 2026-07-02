//go:build wireinject
// +build wireinject

package di

import (
	"github.com/google/wire"
	"go-espn-api/config"
	"go-espn-api/infras/espn"
	"go-espn-api/infras/jwt"
	"go-espn-api/infras/nhl"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/infras/redis"
	"go-espn-api/infras/s3"
	athleteRepo "go-espn-api/internal/domains/athlete/repository"
	athletestatsRepo "go-espn-api/internal/domains/athletestats/repository"
	athletestatsSvc "go-espn-api/internal/domains/athletestats/service"
	competitorRepo "go-espn-api/internal/domains/competitor/repository"
	eventRepo "go-espn-api/internal/domains/event/repository"
	eventSvc "go-espn-api/internal/domains/event/service"
	injuryRepo "go-espn-api/internal/domains/injury/repository"
	injurySvc "go-espn-api/internal/domains/injury/service"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	leagueSvc "go-espn-api/internal/domains/league/service"
	newsRepo "go-espn-api/internal/domains/news/repository"
	newsSvc "go-espn-api/internal/domains/news/service"
	nhlgameRepo "go-espn-api/internal/domains/nhlgame/repository"
	nhlgameSvc "go-espn-api/internal/domains/nhlgame/service"
	nhlgoaliestatsRepo "go-espn-api/internal/domains/nhlgoaliestats/repository"
	nhlgoaliestatsSvc "go-espn-api/internal/domains/nhlgoaliestats/service"
	nhlplayerRepo "go-espn-api/internal/domains/nhlplayer/repository"
	nhlplayerSvc "go-espn-api/internal/domains/nhlplayer/service"
	nhlskaterstatsRepo "go-espn-api/internal/domains/nhlskaterstats/repository"
	nhlskaterstatsSvc "go-espn-api/internal/domains/nhlskaterstats/service"
	nhlstandingRepo "go-espn-api/internal/domains/nhlstanding/repository"
	nhlstandingSvc "go-espn-api/internal/domains/nhlstanding/service"
	nhlteamRepo "go-espn-api/internal/domains/nhlteam/repository"
	nhlteamSvc "go-espn-api/internal/domains/nhlteam/service"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	sportSvc "go-espn-api/internal/domains/sport/service"
	teamRepo "go-espn-api/internal/domains/team/repository"
	teamSvc "go-espn-api/internal/domains/team/service"
	transactionRepo "go-espn-api/internal/domains/transaction/repository"
	transactionSvc "go-espn-api/internal/domains/transaction/service"
	venueRepo "go-espn-api/internal/domains/venue/repository"
	athletestatsHandler "go-espn-api/internal/handlers/athletestats"
	eventHandler "go-espn-api/internal/handlers/event"
	ingestHandler "go-espn-api/internal/handlers/ingest"
	injuryHandler "go-espn-api/internal/handlers/injury"
	leagueHandler "go-espn-api/internal/handlers/league"
	newsHandler "go-espn-api/internal/handlers/news"
	nhlgameHandler "go-espn-api/internal/handlers/nhlgame"
	nhlgoaliestatsHandler "go-espn-api/internal/handlers/nhlgoaliestats"
	nhlplayerHandler "go-espn-api/internal/handlers/nhlplayer"
	nhlskaterstatsHandler "go-espn-api/internal/handlers/nhlskaterstats"
	nhlstandingHandler "go-espn-api/internal/handlers/nhlstanding"
	nhlteamHandler "go-espn-api/internal/handlers/nhlteam"
	sportHandler "go-espn-api/internal/handlers/sport"
	teamHandler "go-espn-api/internal/handlers/team"
	transactionHandler "go-espn-api/internal/handlers/transaction"
	"go-espn-api/internal/ingest"
	"go-espn-api/internal/scheduler"
	"go-espn-api/permissions"
	"go-espn-api/shared/cache"
	"go-espn-api/transport/http"
	"go-espn-api/transport/http/middleware"
	"go-espn-api/transport/http/router"
)

var configurations = wire.NewSet(
	config.Get,
	permissions.Get,
)

var infrastructures = wire.NewSet(
	postgres.New,
	otel.New,
	redis.New,
	s3.New,
	jwt.New,
	espn.New,
)

// ingestServices wires the ESPN write-side ingestion services, the extra repos
// they need (venue, competitor, athlete), and the ingest HTTP handler.
var ingestServices = wire.NewSet(
	venueRepo.New,
	competitorRepo.New,
	athleteRepo.New,
	ingest.NewScoreboardService,
	ingest.NewTeamsService,
	ingest.NewNewsService,
	ingest.NewInjuriesService,
	ingest.NewTransactionsService,
	ingest.NewAthleteStatsService,
	ingestHandler.New,
)

var middlewares = wire.NewSet(
	middleware.NewAppMiddleware,
	middleware.NewAuthRoleMiddleware,
)

var sharedHelpers = wire.NewSet(
	cache.NewRedisCache,
)

var domains = wire.NewSet(
	sportRepo.New,
	sportSvc.New,
	sportHandler.New,
	leagueRepo.New,
	leagueSvc.New,
	leagueHandler.New,
	newsRepo.New,
	newsSvc.New,
	newsHandler.New,
	injuryRepo.New,
	injurySvc.New,
	injuryHandler.New,
	transactionRepo.New,
	transactionSvc.New,
	transactionHandler.New,
	athletestatsRepo.New,
	athletestatsSvc.New,
	athletestatsHandler.New,
	teamRepo.New,
	teamSvc.New,
	teamHandler.New,
	eventRepo.New,
	eventSvc.New,
	eventHandler.New,
	nhlteamRepo.New,
	nhlteamSvc.New,
	nhlteamHandler.New,
	nhlplayerRepo.New,
	nhlplayerSvc.New,
	nhlplayerHandler.New,
	nhlgameRepo.New,
	nhlgameSvc.New,
	nhlgameHandler.New,
	nhlstandingRepo.New,
	nhlstandingSvc.New,
	nhlstandingHandler.New,
	nhlskaterstatsRepo.New,
	nhlskaterstatsSvc.New,
	nhlskaterstatsHandler.New,
	nhlgoaliestatsRepo.New,
	nhlgoaliestatsSvc.New,
	nhlgoaliestatsHandler.New,
)

var routing = wire.NewSet(
	wire.Struct(new(router.DomainHandlers), "*"),
	router.New,
)

func InitializeService() (*http.HTTP, error) {
	wire.Build(
		configurations,
		infrastructures,
		middlewares,
		sharedHelpers,
		domains,
		ingestServices,
		routing,
		http.New,
	)

	return &http.HTTP{}, nil
}

// workerInfrastructures is the trimmed infra the worker needs: config, otel,
// postgres, and the ESPN client. No redis/s3/jwt — the worker only writes to
// Postgres via the ingest services.
var workerInfrastructures = wire.NewSet(
	config.Get,
	otel.New,
	postgres.New,
	espn.New,
	nhl.New,
)

// workerIngest wires the repos and ingest services the scheduler drives (no
// HTTP handler, no athlete-stats service). Includes both the ESPN and NHL
// write-side ingestion.
var workerIngest = wire.NewSet(
	sportRepo.New,
	leagueRepo.New,
	venueRepo.New,
	teamRepo.New,
	eventRepo.New,
	competitorRepo.New,
	newsRepo.New,
	injuryRepo.New,
	transactionRepo.New,
	nhlteamRepo.New,
	nhlplayerRepo.New,
	nhlstandingRepo.New,
	ingest.NewScoreboardService,
	ingest.NewTeamsService,
	ingest.NewNewsService,
	ingest.NewInjuriesService,
	ingest.NewTransactionsService,
	ingest.NewNHLTeamsService,
	ingest.NewNHLRostersService,
	ingest.NewNHLStandingsService,
)

// InitializeWorker builds the ESPN ingestion worker (scheduler + resources).
func InitializeWorker() (*scheduler.Worker, error) {
	wire.Build(
		workerInfrastructures,
		workerIngest,
		scheduler.New,
		wire.Struct(new(scheduler.Worker), "*"),
	)

	return &scheduler.Worker{}, nil
}
