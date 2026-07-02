// Package router provides HTTP routing utilities.
package router

import (
	"go-espn-api/config"
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// DomainHandlers aggregates every domain HTTP handler. Wire fills it via struct
// injection.
type DomainHandlers struct {
	Sport        sportHandler.Handler
	League       leagueHandler.Handler
	News         newsHandler.Handler
	Injury       injuryHandler.Handler
	Transaction  transactionHandler.Handler
	AthleteStats athletestatsHandler.Handler
	Team         teamHandler.Handler
	Event        eventHandler.Handler
	Ingest       ingestHandler.Handler

	NHLTeam        nhlteamHandler.Handler
	NHLPlayer      nhlplayerHandler.Handler
	NHLGame        nhlgameHandler.Handler
	NHLStanding    nhlstandingHandler.Handler
	NHLSkaterStats nhlskaterstatsHandler.Handler
	NHLGoalieStats nhlgoaliestatsHandler.Handler
}

// Router owns the registered domain handlers and the runtime configuration used
// to gate which service route groups are mounted.
type Router struct {
	DomainHandlers DomainHandlers
	cfg            *config.Config
}

// SetupRoutes mounts the DRF-compatible domain routers under /api/v1. Trailing
// slashes are normalised so both `/api/v1/sports/` and `/api/v1/sports`
// resolve to the same handler (Django DefaultRouter emits trailing-slash URLs).
//
// The ESPN domain group is mounted only when SERVICES_ESPN_ENABLED is true; the
// NHL group is mounted under the `/nhl` subpath only when SERVICES_NHL_ENABLED
// is true. Both share the same /api/v1 APIKey + StripSlashes group.
func (r *Router) SetupRoutes(router chi.Router) {
	router.Route("/api/v1", func(rg chi.Router) {
		rg.Use(middleware.StripSlashes)

		if r.cfg.Services.ESPN.Enabled {
			r.DomainHandlers.Sport.Router(rg)
			r.DomainHandlers.League.Router(rg)
			r.DomainHandlers.News.Router(rg)
			r.DomainHandlers.Injury.Router(rg)
			r.DomainHandlers.Transaction.Router(rg)
			r.DomainHandlers.AthleteStats.Router(rg)
			r.DomainHandlers.Team.Router(rg)
			r.DomainHandlers.Event.Router(rg)
			r.DomainHandlers.Ingest.Router(rg)
		}

		if r.cfg.Services.NHL.Enabled {
			rg.Route("/nhl", func(nr chi.Router) {
				r.DomainHandlers.NHLTeam.Router(nr)
				r.DomainHandlers.NHLPlayer.Router(nr)
				r.DomainHandlers.NHLGame.Router(nr)
				r.DomainHandlers.NHLStanding.Router(nr)
				r.DomainHandlers.NHLSkaterStats.Router(nr)
				r.DomainHandlers.NHLGoalieStats.Router(nr)
			})
		}
	})
}

// New creates a Router with the provided domain handlers and configuration.
func New(domainHandlers DomainHandlers, cfg *config.Config) Router {
	return Router{
		DomainHandlers: domainHandlers,
		cfg:            cfg,
	}
}
