package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
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
	competitorRepo "go-espn-api/internal/domains/competitor/repository"
	eventRepo "go-espn-api/internal/domains/event/repository"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	teamRepo "go-espn-api/internal/domains/team/repository"
	venueRepo "go-espn-api/internal/domains/venue/repository"
	"go-espn-api/internal/ingest"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

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
      "venue": {"id": "349", "fullName": "TD Garden", "address": {"city": "Boston", "state": "MA", "country": "USA"}, "indoor": true, "capacity": 19156},
      "competitors": [
        {"id": "c1", "homeAway": "home", "winner": true, "score": "110",
         "team": {"id": "2", "abbreviation": "BOS", "displayName": "Boston Celtics", "shortDisplayName": "Celtics", "name": "Celtics", "location": "Boston", "logo": "http://logo/bos.png"}},
        {"id": "c2", "homeAway": "away", "winner": false, "score": "104",
         "team": {"id": "13", "abbreviation": "LAL", "displayName": "Los Angeles Lakers", "shortDisplayName": "Lakers", "name": "Lakers", "location": "Los Angeles", "logo": "http://logo/lal.png"}}
      ]
    }]
  }]
}`

func startPostgres(t *testing.T) *postgres.Connection {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	name := fmt.Sprintf("espn-sched-test-%d", time.Now().UnixNano())
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

	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	portOut, err := exec.Command("docker", "port", name, "5432/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("docker port: %v: %s", err, portOut)
	}
	hostPort := strings.TrimSpace(string(portOut))
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		hostPort = hostPort[idx+1:]
	}

	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/espn?sslmode=disable", hostPort)

	var db *sqlx.DB
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		db, err = sqlx.Connect("postgres", dsn)
		if err == nil && db.Ping() == nil {
			break
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

func countRows(t *testing.T, db *postgres.Connection, query string) int {
	t.Helper()

	var n int
	if err := db.Read.Get(&n, query); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}

	return n
}

// buildScoreboardScheduler wires a Scheduler with a real ScoreboardService (over
// the docker DB) and a mock ESPN client, plus the real event repo for unstick.
func buildScoreboardScheduler(t *testing.T, db *postgres.Connection, cfg *config.Config) (*Scheduler, *espnmocks.MockESPN) {
	t.Helper()

	ctrl := gomock.NewController(t)
	espnMock := espnmocks.NewMockESPN(ctrl)
	otl := otelmocks.NewOtel()

	events := eventRepo.New(db, otl)
	sb := ingest.NewScoreboardService(
		espnMock, db, otl,
		sportRepo.New(db, otl), leagueRepo.New(db, otl),
		venueRepo.New(db, otl), teamRepo.New(db, otl),
		events, competitorRepo.New(db, otl),
	)

	s := newScheduler(cfg, otl, deps{scoreboard: sb, events: events})

	return s, espnMock
}

func TestScoreboardsTickLandsRows(t *testing.T) {
	db := startPostgres(t)
	cfg := &config.Config{}
	cfg.Services.ESPN.IngestLeagues = []string{"basketball:nba"}
	cfg.Services.ESPN.IngestConcurrency = 2

	s, espnMock := buildScoreboardScheduler(t, db, cfg)
	espnMock.EXPECT().
		GetScoreboard(gomock.Any(), "basketball", "nba", gomock.Any(), gomock.Any()).
		Return(resp(scoreboardJSON), nil).
		AnyTimes()

	sum := s.runScoreboards(context.Background())

	// 1 league * 2 dates = 2 units. Same event (by espn_id) so one create, one
	// update; no errors.
	if sum.errors != 0 {
		t.Fatalf("summary errors = %d, want 0", sum.errors)
	}
	if sum.created+sum.updated != 2 {
		t.Fatalf("created+updated = %d, want 2 (today+yesterday)", sum.created+sum.updated)
	}
	if sum.created != 1 || sum.updated != 1 {
		t.Fatalf("summary = %+v, want created=1 updated=1", sum)
	}

	if got := countRows(t, db, "SELECT COUNT(*) FROM events"); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	if got := countRows(t, db, "SELECT COUNT(*) FROM competitors"); got != 2 {
		t.Fatalf("competitors = %d, want 2", got)
	}
}

func TestUnstickReingestsSeededStuckEvent(t *testing.T) {
	db := startPostgres(t)
	cfg := &config.Config{}
	cfg.Services.ESPN.IngestConcurrency = 2

	s, espnMock := buildScoreboardScheduler(t, db, cfg)
	espnMock.EXPECT().
		GetScoreboard(gomock.Any(), "basketball", "nba", gomock.Any(), gomock.Any()).
		Return(resp(scoreboardJSON), nil).
		AnyTimes()

	// Seed a stuck event: same league/espn_id as the fixture but status scheduled
	// and a date a few days in the past so StuckEvents picks it up.
	stuckDate := time.Now().UTC().AddDate(0, 0, -3).Truncate(time.Hour)
	seedStuckEvent(t, db, stuckDate)

	// Sanity: the repo query finds it.
	refs, err := eventRepo.New(db, otelmocks.NewOtel()).StuckEvents(context.Background(), 7, 200)
	if err != nil {
		t.Fatalf("StuckEvents: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("stuck refs = %d, want 1", len(refs))
	}

	sum := s.runUnstick(context.Background())
	if sum.errors != 0 {
		t.Fatalf("unstick errors = %d, want 0", sum.errors)
	}

	// The re-ingest must have flipped the seeded event to final.
	var status string
	if err := db.Read.QueryRow(`SELECT status FROM events WHERE espn_id = '401585183'`).Scan(&status); err != nil {
		t.Fatalf("event status: %v", err)
	}
	if status != "final" {
		t.Fatalf("status = %q, want final (unstick should re-ingest)", status)
	}

	// After the flip, no more stuck events.
	refs2, _ := eventRepo.New(db, otelmocks.NewOtel()).StuckEvents(context.Background(), 7, 200)
	if len(refs2) != 0 {
		t.Fatalf("stuck refs after unstick = %d, want 0", len(refs2))
	}
}

// seedStuckEvent inserts the sport/league/event needed for the unstick test,
// with the event marked scheduled and dated in the past.
func seedStuckEvent(t *testing.T, db *postgres.Connection, date time.Time) {
	t.Helper()

	var sportID int64
	if err := db.Write.QueryRow(
		`INSERT INTO sports (slug, name) VALUES ('basketball', 'Basketball') RETURNING id`,
	).Scan(&sportID); err != nil {
		t.Fatalf("seed sport: %v", err)
	}

	var leagueID int64
	if err := db.Write.QueryRow(
		`INSERT INTO leagues (sport_id, slug, name, abbreviation) VALUES ($1, 'nba', 'NBA', 'NBA') RETURNING id`,
		sportID,
	).Scan(&leagueID); err != nil {
		t.Fatalf("seed league: %v", err)
	}

	if _, err := db.Write.Exec(
		`INSERT INTO events (league_id, espn_id, uid, date, name, short_name, season_year, season_type, season_slug, status, status_detail)
		 VALUES ($1, '401585183', 's:40~l:46~e:401585183', $2, 'seed', 'seed', 2024, 2, 'regular-season', 'scheduled', 'Scheduled')`,
		leagueID, date,
	); err != nil {
		t.Fatalf("seed event: %v", err)
	}
}
