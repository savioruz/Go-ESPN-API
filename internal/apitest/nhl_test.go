package apitest

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"go-espn-api/config"
	"go-espn-api/infras/otel/mocks"
	"go-espn-api/infras/postgres"

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

	nhlteamDTO "go-espn-api/internal/domains/nhlteam/model/dto"
	nhlteamService "go-espn-api/internal/domains/nhlteam/service"
	sportDTO "go-espn-api/internal/domains/sport/model/dto"
	sportService "go-espn-api/internal/domains/sport/service"

	nhlgameHandler "go-espn-api/internal/handlers/nhlgame"
	nhlgoaliestatsHandler "go-espn-api/internal/handlers/nhlgoaliestats"
	nhlplayerHandler "go-espn-api/internal/handlers/nhlplayer"
	nhlskaterstatsHandler "go-espn-api/internal/handlers/nhlskaterstats"
	nhlstandingHandler "go-espn-api/internal/handlers/nhlstanding"
	nhlteamHandler "go-espn-api/internal/handlers/nhlteam"
	sportHandler "go-espn-api/internal/handlers/sport"

	"go-espn-api/transport/http/middleware"
	"go-espn-api/transport/http/router"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stub services (DB-free) for the gating test ---

type stubSport struct{}

func (stubSport) List(context.Context, int, int) ([]sportDTO.SportResponse, int, error) {
	return []sportDTO.SportResponse{}, 0, nil
}
func (stubSport) GetBySlug(context.Context, string) (*sportDTO.SportResponse, error) { return nil, nil }

type stubNHLTeam struct{}

func (stubNHLTeam) List(context.Context, nhlteamRepo.ListFilter, int, int) ([]nhlteamDTO.TeamResponse, int, error) {
	return []nhlteamDTO.TeamResponse{}, 0, nil
}
func (stubNHLTeam) GetByID(context.Context, int64) (*nhlteamDTO.TeamResponse, error) { return nil, nil }

var (
	_ sportService.Sport     = stubSport{}
	_ nhlteamService.NHLTeam = stubNHLTeam{}
)

// gatingMux builds a mux with stubbed Sport + NHLTeam handlers so route presence
// can be asserted without a database.
func gatingMux(espnEnabled, nhlEnabled bool) http.Handler {
	otl := mocks.NewOtel()

	dh := router.DomainHandlers{
		Sport:   sportHandler.New(stubSport{}, otl),
		NHLTeam: nhlteamHandler.New(stubNHLTeam{}, otl),
	}

	cfg := &config.Config{}
	cfg.App.APIKey = testAPIKey
	cfg.Services.ESPN.Enabled = espnEnabled
	cfg.Services.NHL.Enabled = nhlEnabled

	auth := middleware.NewAuthRoleMiddleware(nil, otl, nil, cfg)
	rt := router.New(dh, cfg)

	mux := chi.NewRouter()
	mux.With(auth.APIKeyRequired).Group(func(rc chi.Router) {
		rt.SetupRoutes(rc)
	})

	return mux
}

func TestNHLRouteGating(t *testing.T) {
	// NHL enabled, ESPN disabled: NHL serves, ESPN 404s.
	mux := gatingMux(false, true)
	assert.Equal(t, http.StatusOK, do(t, mux, "/api/v1/nhl/teams/", true).Code, "nhl on -> 200")
	assert.Equal(t, http.StatusNotFound, do(t, mux, "/api/v1/sports/", true).Code, "espn off -> 404")

	// ESPN enabled, NHL disabled: ESPN serves, NHL 404s.
	mux = gatingMux(true, false)
	assert.Equal(t, http.StatusOK, do(t, mux, "/api/v1/sports/", true).Code, "espn on -> 200")
	assert.Equal(t, http.StatusNotFound, do(t, mux, "/api/v1/nhl/teams/", true).Code, "nhl off -> 404")

	// Both enabled: both serve.
	mux = gatingMux(true, true)
	assert.Equal(t, http.StatusOK, do(t, mux, "/api/v1/sports/", true).Code)
	assert.Equal(t, http.StatusOK, do(t, mux, "/api/v1/nhl/teams/", true).Code)
}

// setupNHLMux builds a DB-backed mux with all six NHL domains mounted.
func setupNHLMux(t *testing.T) http.Handler {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set; skipping DB integration test")
	}

	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(t, err)

	conn := &postgres.Connection{Read: db, Write: db}
	otl := mocks.NewOtel()

	dh := router.DomainHandlers{
		NHLTeam:        nhlteamHandler.New(nhlteamSvc.New(nhlteamRepo.New(conn, otl), otl), otl),
		NHLPlayer:      nhlplayerHandler.New(nhlplayerSvc.New(nhlplayerRepo.New(conn, otl), otl), otl),
		NHLGame:        nhlgameHandler.New(nhlgameSvc.New(nhlgameRepo.New(conn, otl), otl), otl),
		NHLStanding:    nhlstandingHandler.New(nhlstandingSvc.New(nhlstandingRepo.New(conn, otl), otl), otl),
		NHLSkaterStats: nhlskaterstatsHandler.New(nhlskaterstatsSvc.New(nhlskaterstatsRepo.New(conn, otl), otl), otl),
		NHLGoalieStats: nhlgoaliestatsHandler.New(nhlgoaliestatsSvc.New(nhlgoaliestatsRepo.New(conn, otl), otl), otl),
	}

	cfg := &config.Config{}
	cfg.App.APIKey = testAPIKey
	cfg.Services.NHL.Enabled = true

	auth := middleware.NewAuthRoleMiddleware(nil, otl, nil, cfg)
	rt := router.New(dh, cfg)

	mux := chi.NewRouter()
	mux.With(auth.APIKeyRequired).Group(func(rc chi.Router) {
		rt.SetupRoutes(rc)
	})

	return mux
}

func TestNHLSkaterStatsNestedSerialization(t *testing.T) {
	mux := setupNHLMux(t)

	w := do(t, mux, "/api/v1/nhl/skater-stats/", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	require.GreaterOrEqual(t, count, 1)
	require.NotEmpty(t, results)

	// Level 1: skater-stats field set.
	assert.Equal(t,
		[]string{"assists", "games_played", "goals", "id", "player", "plus_minus", "points", "raw_data", "season"},
		keysOf(results[0]))

	// Level 2: nested player.
	player, ok := results[0]["player"].(map[string]any)
	require.True(t, ok, "player must be a nested object")
	assert.Equal(t,
		[]string{"current_team", "first_name", "full_name", "headshot_url", "id", "is_active", "last_name", "player_id", "position", "sweater_number"},
		keysOf(player))

	// Level 3: nested current_team inside the player.
	team, ok := player["current_team"].(map[string]any)
	require.True(t, ok, "current_team must be a nested object")
	assert.Equal(t,
		[]string{"abbreviation", "franchise_id", "full_name", "id", "is_active", "name", "raw_data", "team_id"},
		keysOf(team))

	pretty, _ := json.MarshalIndent(results[0], "", "  ")
	t.Logf("GET /api/v1/nhl/skater-stats/ row:\n%s", pretty)
}

func TestNHLTeamsExcludeInactive(t *testing.T) {
	mux := setupNHLMux(t)

	w := do(t, mux, "/api/v1/nhl/teams/", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 1, count, "inactive team must be excluded")
	require.Len(t, results, 1)
	assert.Equal(t, "BOS", results[0]["abbreviation"])
	assert.Equal(t,
		[]string{"abbreviation", "franchise_id", "full_name", "id", "is_active", "name", "raw_data", "team_id"},
		keysOf(results[0]))
	// raw_data JSONB passthrough (object, never null).
	_, ok := results[0]["raw_data"].(map[string]any)
	assert.True(t, ok)
}

func TestNHLGamesFilterAndNestedTeams(t *testing.T) {
	mux := setupNHLMux(t)

	w := do(t, mux, "/api/v1/nhl/games/?season=20232024&status=final", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 1, count, "exact season+status filter")
	require.Len(t, results, 1)

	home, ok := results[0]["home_team"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "BOS", home["abbreviation"])

	away, ok := results[0]["away_team"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ARI", away["abbreviation"])

	// date is a DateTimeField -> ISO-8601 ...Z.
	assert.Equal(t, "2024-01-15T00:00:00Z", results[0]["date"])
}

func TestNHLDetailAndNotFound(t *testing.T) {
	mux := setupNHLMux(t)

	// Existing detail.
	w := do(t, mux, "/api/v1/nhl/teams/1/", true)
	require.Equal(t, 200, w.Code)

	var team map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &team))
	assert.Equal(t, "BOS", team["abbreviation"])

	// Missing id -> 404 {"error":"Not found."}.
	nf := do(t, mux, "/api/v1/nhl/teams/9999/", true)
	assert.Equal(t, http.StatusNotFound, nf.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(nf.Body.Bytes(), &body))
	assert.Equal(t, "Not found.", body["error"])
}

func TestNHLStandingsAndGoalieNullFloats(t *testing.T) {
	mux := setupNHLMux(t)

	// Standings: DateField -> YYYY-MM-DD, nullable point_pct.
	w := do(t, mux, "/api/v1/nhl/standings/", true)
	require.Equal(t, 200, w.Code)

	_, _, _, results := envelope(t, w.Body.Bytes())
	require.NotEmpty(t, results)
	assert.Equal(t, "2024-01-15", results[0]["date"])

	// Goalie stats: nullable save_pct/gaa present as numbers.
	g := do(t, mux, "/api/v1/nhl/goalie-stats/", true)
	require.Equal(t, 200, g.Code)

	_, _, _, grows := envelope(t, g.Body.Bytes())
	require.NotEmpty(t, grows)
	assert.Equal(t,
		[]string{"gaa", "games_played", "id", "losses", "player", "raw_data", "save_pct", "season", "shutouts", "wins"},
		keysOf(grows[0]))
}
