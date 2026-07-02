package apitest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"testing"

	"go-espn-api/config"
	"go-espn-api/infras/otel/mocks"
	"go-espn-api/infras/postgres"

	athletestatsRepo "go-espn-api/internal/domains/athletestats/repository"
	athletestatsSvc "go-espn-api/internal/domains/athletestats/service"
	injuryRepo "go-espn-api/internal/domains/injury/repository"
	injurySvc "go-espn-api/internal/domains/injury/service"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	leagueSvc "go-espn-api/internal/domains/league/service"
	newsRepo "go-espn-api/internal/domains/news/repository"
	newsSvc "go-espn-api/internal/domains/news/service"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	sportSvc "go-espn-api/internal/domains/sport/service"
	transactionRepo "go-espn-api/internal/domains/transaction/repository"
	transactionSvc "go-espn-api/internal/domains/transaction/service"

	athletestatsHandler "go-espn-api/internal/handlers/athletestats"
	injuryHandler "go-espn-api/internal/handlers/injury"
	leagueHandler "go-espn-api/internal/handlers/league"
	newsHandler "go-espn-api/internal/handlers/news"
	sportHandler "go-espn-api/internal/handlers/sport"
	transactionHandler "go-espn-api/internal/handlers/transaction"

	"go-espn-api/transport/http/middleware"
	"go-espn-api/transport/http/router"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test-secret-key"

func setupMux(t *testing.T) http.Handler {
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
		Sport:        sportHandler.New(sportSvc.New(sportRepo.New(conn, otl), otl), otl),
		League:       leagueHandler.New(leagueSvc.New(leagueRepo.New(conn, otl), otl), otl),
		News:         newsHandler.New(newsSvc.New(newsRepo.New(conn, otl), otl), otl),
		Injury:       injuryHandler.New(injurySvc.New(injuryRepo.New(conn, otl), otl), otl),
		Transaction:  transactionHandler.New(transactionSvc.New(transactionRepo.New(conn, otl), otl), otl),
		AthleteStats: athletestatsHandler.New(athletestatsSvc.New(athletestatsRepo.New(conn, otl), otl), otl),
	}

	cfg := &config.Config{}
	cfg.App.APIKey = testAPIKey
	cfg.Services.ESPN.Enabled = true
	auth := middleware.NewAuthRoleMiddleware(nil, otl, nil, cfg)

	rt := router.New(dh, cfg)

	mux := chi.NewRouter()
	mux.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.With(auth.APIKeyRequired).Group(func(rc chi.Router) {
		rt.SetupRoutes(rc)
	})

	return mux
}

func do(t *testing.T, mux http.Handler, path string, withKey bool) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest("GET", "http://api.test"+path, nil)
	if withKey {
		req.Header.Set("X-API-Key", testAPIKey)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	return w
}

func envelope(t *testing.T, body []byte) (count int, next, prev any, results []map[string]any) {
	t.Helper()

	var out struct {
		Count    int              `json:"count"`
		Next     any              `json:"next"`
		Previous any              `json:"previous"`
		Results  []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(body, &out))

	return out.Count, out.Next, out.Previous, out.Results
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	return ks
}

func TestSportsList(t *testing.T) {
	mux := setupMux(t)

	// Trailing slash and non-slash both resolve.
	for _, path := range []string{"/api/v1/sports/", "/api/v1/sports"} {
		w := do(t, mux, path, true)
		require.Equal(t, 200, w.Code, path)

		count, next, prev, results := envelope(t, w.Body.Bytes())
		assert.Equal(t, 2, count)
		assert.Nil(t, next)
		assert.Nil(t, prev)
		require.Len(t, results, 2)
		assert.Equal(t,
			[]string{"created_at", "id", "name", "slug", "updated_at"},
			keysOf(results[0]))
	}

	t.Logf("GET /api/v1/sports/ body:\n%s", do(t, mux, "/api/v1/sports/", true).Body.String())
}

func TestAPIKeyRequired(t *testing.T) {
	mux := setupMux(t)

	assert.Equal(t, http.StatusForbidden, do(t, mux, "/api/v1/sports/", false).Code)
	assert.Equal(t, http.StatusOK, do(t, mux, "/api/v1/sports/", true).Code)
	assert.Equal(t, http.StatusOK, do(t, mux, "/healthz", false).Code)
}

func TestNewsListVsDetailFieldSets(t *testing.T) {
	mux := setupMux(t)

	// List filtered by league=nba -> 2 items, list serializer field set.
	w := do(t, mux, "/api/v1/news/?league=nba", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 2, count)
	require.Len(t, results, 2)

	wantList := []string{
		"description", "espn_id", "headline", "id", "league_slug",
		"published", "sport_slug", "thumbnail", "type",
	}
	assert.Equal(t, wantList, keysOf(results[0]))

	// Thumbnail computed from images[].url or images[].href.
	thumbs := map[string]any{}
	for _, r := range results {
		thumbs[r["espn_id"].(string)] = r["thumbnail"]
	}
	assert.Equal(t, "https://img/1.jpg", thumbs["n1"])         // url wins
	assert.Equal(t, "https://img/only-href.jpg", thumbs["n2"]) // falls back to href

	// Detail serializer field set (fetch the nba url-thumbnail article id).
	id := int(results[0]["id"].(float64))
	// pick n1 specifically
	for _, r := range results {
		if r["espn_id"] == "n1" {
			id = int(r["id"].(float64))
		}
	}

	dw := do(t, mux, "/api/v1/news/"+strconv.Itoa(id)+"/", true)
	require.Equal(t, 200, dw.Code)

	var detail map[string]any
	require.NoError(t, json.Unmarshal(dw.Body.Bytes(), &detail))

	wantDetail := []string{
		"categories", "created_at", "description", "espn_id", "headline",
		"id", "images", "last_modified", "league_slug", "links", "published",
		"sport_slug", "thumbnail", "type", "updated_at",
	}
	assert.Equal(t, wantDetail, keysOf(detail))

	t.Logf("GET /api/v1/news/%d/ body:\n%s", id, dw.Body.String())
}

func TestLeaguesNestedSportAndFilter(t *testing.T) {
	mux := setupMux(t)

	// Filter by sport=basketball -> only NBA, nested full SportSerializer.
	w := do(t, mux, "/api/v1/leagues/?sport=basketball", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 1, count)
	require.Len(t, results, 1)

	assert.Equal(t,
		[]string{"abbreviation", "created_at", "id", "name", "slug", "sport", "updated_at"},
		keysOf(results[0]))

	sport, ok := results[0]["sport"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t,
		[]string{"created_at", "id", "name", "slug", "updated_at"},
		keysOf(sport))
	assert.Equal(t, "basketball", sport["slug"])

	t.Logf("GET /api/v1/leagues/?sport=basketball body:\n%s", w.Body.String())
}

func TestNewsOrphanNullSlugs(t *testing.T) {
	mux := setupMux(t)

	// Unfiltered news -> 4 items incl. one with no league (null slugs).
	w := do(t, mux, "/api/v1/news/", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 4, count)

	var orphan map[string]any
	for _, r := range results {
		if r["espn_id"] == "n4" {
			orphan = r
		}
	}
	require.NotNil(t, orphan)
	assert.Nil(t, orphan["league_slug"])
	assert.Nil(t, orphan["sport_slug"])
	assert.Nil(t, orphan["thumbnail"])
}

func TestInjuriesTeamFilter(t *testing.T) {
	mux := setupMux(t)

	// team=lal (lowercase) must iexact-match abbreviation LAL.
	w := do(t, mux, "/api/v1/injuries/?team=lal", true)
	require.Equal(t, 200, w.Code)

	count, _, _, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 1, count)
	require.Len(t, results, 1)
	assert.Equal(t, "LeBron James", results[0]["athlete_name"])
	assert.Equal(t, "LAL", results[0]["team_abbreviation"])
}

func TestAthleteStatsPagination(t *testing.T) {
	mux := setupMux(t)

	// 32 rows total, page size 25.
	w := do(t, mux, "/api/v1/athlete-stats/", true)
	require.Equal(t, 200, w.Code)

	count, next, prev, results := envelope(t, w.Body.Bytes())
	assert.Equal(t, 32, count)
	assert.Len(t, results, 25)
	require.NotNil(t, next)
	assert.Contains(t, next.(string), "page=2")
	assert.Nil(t, prev)

	// Page 2 -> 7 rows, previous set, next null.
	w2 := do(t, mux, "/api/v1/athlete-stats/?page=2", true)
	count2, next2, prev2, results2 := envelope(t, w2.Body.Bytes())
	assert.Equal(t, 32, count2)
	assert.Len(t, results2, 7)
	assert.Nil(t, next2)
	require.NotNil(t, prev2)
}
