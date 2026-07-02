package ingest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go-espn-api/config"
	"go-espn-api/infras/espn"
	espnmocks "go-espn-api/infras/espn/mocks"
	otelmocks "go-espn-api/infras/otel/mocks"
	"go-espn-api/infras/postgres"
	athleteRepo "go-espn-api/internal/domains/athlete/repository"
	athletestatsRepo "go-espn-api/internal/domains/athletestats/repository"
	competitorRepo "go-espn-api/internal/domains/competitor/repository"
	eventRepo "go-espn-api/internal/domains/event/repository"
	injuryRepo "go-espn-api/internal/domains/injury/repository"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	newsRepo "go-espn-api/internal/domains/news/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	teamRepo "go-espn-api/internal/domains/team/repository"
	transactionRepo "go-espn-api/internal/domains/transaction/repository"
	venueRepo "go-espn-api/internal/domains/venue/repository"
	ingestHandler "go-espn-api/internal/handlers/ingest"
	"go-espn-api/internal/ingest"
	httpmw "go-espn-api/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

// --- test fixtures -----------------------------------------------------------

const scoreboardJSON = `{
  "events": [{
    "id": "401585183",
    "uid": "s:40~l:46~e:401585183",
    "date": "2024-12-15T20:00Z",
    "name": "Los Angeles Lakers at Boston Celtics",
    "shortName": "LAL @ BOS",
    "season": {"year": 2024, "type": 2, "slug": "regular-season"},
    "week": {"number": 12},
    "status": {"displayClock": "0.0", "period": 4, "type": {"state": "post", "completed": true, "detail": "Final"}},
    "links": [{"href": "http://x"}],
    "competitions": [{
      "attendance": 19156,
      "broadcasts": [{"market": "national", "names": ["ESPN"]}],
      "venue": {"id": "349", "fullName": "TD Garden", "address": {"city": "Boston", "state": "MA", "country": "USA"}, "indoor": true, "capacity": 19156},
      "competitors": [
        {"id": "c1", "homeAway": "home", "winner": true, "score": "110",
         "team": {"id": "2", "abbreviation": "BOS", "displayName": "Boston Celtics", "shortDisplayName": "Celtics", "name": "Celtics", "location": "Boston", "logo": "http://logo/bos.png"},
         "linescores": [{"value": 30}], "records": [{"summary": "20-5"}]},
        {"id": "c2", "homeAway": "away", "winner": false, "score": "104",
         "team": {"id": "13", "abbreviation": "LAL", "displayName": "Los Angeles Lakers", "shortDisplayName": "Lakers", "name": "Lakers", "location": "Los Angeles", "logo": "http://logo/lal.png"}}
      ]
    }]
  }]
}`

func teamsJSON(displayName string) string {
	return fmt.Sprintf(`{"sports":[{"leagues":[{"teams":[
		{"team":{"id":"25","uid":"s:40~t:25","slug":"golden-state-warriors","abbreviation":"GSW","displayName":%q,"shortDisplayName":"Warriors","name":"Warriors","location":"Golden State","color":"1D428A","isActive":true,"logos":[{"href":"http://logo/gsw.png"}]}}
	]}]}]}`, displayName)
}

func injuriesJSON(names ...string) string {
	items := make([]string, 0, len(names))
	for i, n := range names {
		items = append(items, fmt.Sprintf(`{"athlete":{"id":"%d","displayName":%q,"position":{"abbreviation":"PG"}},"status":"Out","type":"Knee","description":"sprain","team":{"id":"2"}}`, 100+i, n))
	}

	return `{"items":[` + strings.Join(items, ",") + `]}`
}

// --- docker postgres harness -------------------------------------------------

func startPostgres(t *testing.T) *postgres.Connection {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	name := fmt.Sprintf("espn-ingest-test-%d", time.Now().UnixNano())
	runArgs := []string{
		"run", "-d", "--rm", "--name", name,
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_DB=espn",
		"-p", "127.0.0.1:0:5432",
		"postgres:16-alpine",
	}
	if out, err := exec.Command("docker", runArgs...).CombinedOutput(); err != nil {
		t.Skipf("cannot start postgres container: %v: %s", err, out)
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})

	portOut, err := exec.Command("docker", "port", name, "5432/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("docker port: %v: %s", err, portOut)
	}
	// e.g. "127.0.0.1:55001"
	hostPort := strings.TrimSpace(string(portOut))
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		hostPort = hostPort[idx+1:]
	}

	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/espn?sslmode=disable", hostPort)

	var db *sqlx.DB
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		db, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			if pingErr := db.Ping(); pingErr == nil {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if db == nil || db.Ping() != nil {
		t.Fatalf("postgres never became ready: %v", err)
	}

	applyMigrations(t, db)

	return &postgres.Connection{Read: db, Write: db}
}

func applyMigrations(t *testing.T, db *sqlx.DB) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "postgres", "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(files)

	for _, f := range files {
		content, rerr := os.ReadFile(f)
		if rerr != nil {
			t.Fatalf("read %s: %v", f, rerr)
		}
		if _, eerr := db.Exec(string(content)); eerr != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), eerr)
		}
	}
}

func resp(data string) *espn.Response {
	return &espn.Response{Data: json.RawMessage(data), StatusCode: 200}
}

func count(t *testing.T, db *postgres.Connection, query string, args ...any) int {
	t.Helper()

	var n int
	if err := db.Read.Get(&n, query, args...); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}

	return n
}

// --- tests -------------------------------------------------------------------

func TestScoreboardIngestAndIdempotency(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	espnMock := espnmocks.NewMockESPN(ctrl)
	otl := otelmocks.NewOtel()

	espnMock.EXPECT().
		GetScoreboard(gomock.Any(), "basketball", "nba", "20241215", gomock.Any()).
		Return(resp(scoreboardJSON), nil).
		AnyTimes()

	svc := ingest.NewScoreboardService(
		espnMock, db, otl,
		sportRepo.New(db, otl), leagueRepo.New(db, otl),
		venueRepo.New(db, otl), teamRepo.New(db, otl),
		eventRepo.New(db, otl), competitorRepo.New(db, otl),
	)

	ctx := context.Background()

	// First ingest — everything created.
	res, err := svc.IngestScoreboard(ctx, "basketball", "nba", "20241215")
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if res.Created != 1 || res.Updated != 0 || res.Errors != 0 {
		t.Fatalf("first result = %+v, want created=1", res)
	}

	if got := count(t, db, "SELECT COUNT(*) FROM sports"); got != 1 {
		t.Fatalf("sports = %d, want 1", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM leagues"); got != 1 {
		t.Fatalf("leagues = %d, want 1", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM venues"); got != 1 {
		t.Fatalf("venues = %d, want 1", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM events"); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM competitors"); got != 2 {
		t.Fatalf("competitors = %d, want 2", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM teams"); got != 2 {
		t.Fatalf("teams = %d, want 2", got)
	}

	// Status mapping: completed -> final.
	var status, detail string
	if err := db.Read.QueryRow(`SELECT status, status_detail FROM events WHERE espn_id = '401585183'`).Scan(&status, &detail); err != nil {
		t.Fatalf("event status: %v", err)
	}
	if status != "final" || detail != "Final" {
		t.Fatalf("status/detail = %q/%q, want final/Final", status, detail)
	}

	// Home competitor: BOS (team espn 2), home, score 110, winner true.
	var homeAway, score string
	var winner bool
	err = db.Read.QueryRow(`
		SELECT c.home_away, c.score, c.winner
		FROM competitors c JOIN teams t ON t.id = c.team_id
		WHERE t.espn_id = '2'`).Scan(&homeAway, &score, &winner)
	if err != nil {
		t.Fatalf("home competitor: %v", err)
	}
	if homeAway != "home" || score != "110" || !winner {
		t.Fatalf("home competitor = %q/%q/%v, want home/110/true", homeAway, score, winner)
	}

	// Away competitor: LAL (team espn 13), away, 104.
	err = db.Read.QueryRow(`
		SELECT c.home_away, c.score
		FROM competitors c JOIN teams t ON t.id = c.team_id
		WHERE t.espn_id = '13'`).Scan(&homeAway, &score)
	if err != nil {
		t.Fatalf("away competitor: %v", err)
	}
	if homeAway != "away" || score != "104" {
		t.Fatalf("away competitor = %q/%q, want away/104", homeAway, score)
	}

	// Second ingest — idempotent: updated, not duplicated.
	res2, err := svc.IngestScoreboard(ctx, "basketball", "nba", "20241215")
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if res2.Created != 0 || res2.Updated != 1 {
		t.Fatalf("second result = %+v, want updated=1", res2)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM events"); got != 1 {
		t.Fatalf("events after re-ingest = %d, want 1", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM competitors"); got != 2 {
		t.Fatalf("competitors after re-ingest = %d, want 2", got)
	}
}

func TestTeamsIngestUpdatesExisting(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	espnMock := espnmocks.NewMockESPN(ctrl)
	otl := otelmocks.NewOtel()

	gomock.InOrder(
		espnMock.EXPECT().GetTeams(gomock.Any(), "basketball", "nba", gomock.Any()).Return(resp(teamsJSON("Golden State Warriors")), nil),
		espnMock.EXPECT().GetTeams(gomock.Any(), "basketball", "nba", gomock.Any()).Return(resp(teamsJSON("GS Warriors (updated)")), nil),
	)

	svc := ingest.NewTeamsService(
		espnMock, db, otl,
		sportRepo.New(db, otl), leagueRepo.New(db, otl), teamRepo.New(db, otl),
	)
	ctx := context.Background()

	res, err := svc.IngestTeams(ctx, "basketball", "nba")
	if err != nil {
		t.Fatalf("first teams ingest: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("first teams result = %+v, want created=1", res)
	}

	res2, err := svc.IngestTeams(ctx, "basketball", "nba")
	if err != nil {
		t.Fatalf("second teams ingest: %v", err)
	}
	if res2.Updated != 1 || res2.Created != 0 {
		t.Fatalf("second teams result = %+v, want updated=1", res2)
	}

	if got := count(t, db, "SELECT COUNT(*) FROM teams WHERE espn_id = '25'"); got != 1 {
		t.Fatalf("team rows = %d, want 1 (no dup)", got)
	}

	var displayName string
	if err := db.Read.QueryRow(`SELECT display_name FROM teams WHERE espn_id = '25'`).Scan(&displayName); err != nil {
		t.Fatalf("team display_name: %v", err)
	}
	if displayName != "GS Warriors (updated)" {
		t.Fatalf("display_name = %q, want updated value", displayName)
	}
}

func TestInjuriesSnapshotReplacesOldRows(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	espnMock := espnmocks.NewMockESPN(ctrl)
	otl := otelmocks.NewOtel()

	gomock.InOrder(
		espnMock.EXPECT().GetLeagueInjuries(gomock.Any(), "basketball", "nba").Return(resp(injuriesJSON("A", "B", "C")), nil),
		espnMock.EXPECT().GetLeagueInjuries(gomock.Any(), "basketball", "nba").Return(resp(injuriesJSON("A")), nil),
	)

	svc := ingest.NewInjuriesService(
		espnMock, db, otl,
		sportRepo.New(db, otl), leagueRepo.New(db, otl),
		teamRepo.New(db, otl), injuryRepo.New(db, otl),
	)
	ctx := context.Background()

	res, err := svc.IngestInjuries(ctx, "basketball", "nba")
	if err != nil {
		t.Fatalf("first injuries ingest: %v", err)
	}
	if res.Created != 3 {
		t.Fatalf("first injuries result = %+v, want created=3", res)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM injuries"); got != 3 {
		t.Fatalf("injuries after first = %d, want 3", got)
	}

	res2, err := svc.IngestInjuries(ctx, "basketball", "nba")
	if err != nil {
		t.Fatalf("second injuries ingest: %v", err)
	}
	if res2.Created != 1 {
		t.Fatalf("second injuries result = %+v, want created=1", res2)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM injuries"); got != 1 {
		t.Fatalf("injuries after snapshot refresh = %d, want 1", got)
	}
}

func TestScoreboardHandlerViaRouter(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	espnMock := espnmocks.NewMockESPN(ctrl)
	otl := otelmocks.NewOtel()

	espnMock.EXPECT().
		GetScoreboard(gomock.Any(), "basketball", "nba", gomock.Any(), gomock.Any()).
		Return(resp(scoreboardJSON), nil).
		AnyTimes()

	sports := sportRepo.New(db, otl)
	leagues := leagueRepo.New(db, otl)
	teams := teamRepo.New(db, otl)

	h := ingestHandler.New(
		ingest.NewScoreboardService(espnMock, db, otl, sports, leagues, venueRepo.New(db, otl), teams, eventRepo.New(db, otl), competitorRepo.New(db, otl)),
		ingest.NewTeamsService(espnMock, db, otl, sports, leagues, teams),
		ingest.NewNewsService(espnMock, db, otl, sports, leagues, newsRepo.New(db, otl)),
		ingest.NewInjuriesService(espnMock, db, otl, sports, leagues, teams, injuryRepo.New(db, otl)),
		ingest.NewTransactionsService(espnMock, db, otl, sports, leagues, teams, transactionRepo.New(db, otl)),
		otl,
	)
	// Ensure athlete-stats service still constructs (no HTTP route).
	_ = ingest.NewAthleteStatsService(espnMock, db, otl, sports, leagues, athleteRepo.New(db, otl), athletestatsRepo.New(db, otl))

	const apiKey = "secret-key"
	cfg := &config.Config{}
	cfg.App.APIKey = apiKey
	authMW := httpmw.NewAuthRoleMiddleware(nil, otl, nil, cfg)

	r := chi.NewRouter()
	r.Use(middleware.StripSlashes)
	r.With(authMW.APIKeyRequired).Group(func(rg chi.Router) {
		h.Router(rg)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Happy path with trailing slash.
	body := strings.NewReader(`{"sport":"BASKETBALL","league":"NBA","date":"20241215"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest/scoreboard/", body)
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", httpResp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(httpResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, k := range []string{"created", "updated", "errors", "total_processed", "details"} {
		if _, ok := payload[k]; !ok {
			t.Fatalf("response missing key %q: %v", k, payload)
		}
	}
	t.Logf("scoreboard response: %v", payload)

	// Bad date -> 400.
	badReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest/scoreboard/", strings.NewReader(`{"sport":"basketball","league":"nba","date":"2024"}`))
	badReq.Header.Set("X-API-Key", apiKey)
	badResp, err := http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatalf("bad request: %v", err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad date status = %d, want 400", badResp.StatusCode)
	}

	// Missing API key -> rejected (403).
	noKeyReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest/scoreboard/", strings.NewReader(`{"sport":"basketball","league":"nba"}`))
	noKeyResp, err := http.DefaultClient.Do(noKeyReq)
	if err != nil {
		t.Fatalf("no-key request: %v", err)
	}
	noKeyResp.Body.Close()
	if noKeyResp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing key status = %d, want 403", noKeyResp.StatusCode)
	}
}
