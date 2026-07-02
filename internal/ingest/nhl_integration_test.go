package ingest_test

import (
	"context"
	"encoding/json"
	"testing"

	"go-espn-api/config"
	"go-espn-api/infras/nhl"
	nhlmocks "go-espn-api/infras/nhl/mocks"
	otelmocks "go-espn-api/infras/otel/mocks"
	nhlplayerRepo "go-espn-api/internal/domains/nhlplayer/repository"
	nhlstandingRepo "go-espn-api/internal/domains/nhlstanding/repository"
	nhlteamRepo "go-espn-api/internal/domains/nhlteam/repository"
	"go-espn-api/internal/ingest"

	"go.uber.org/mock/gomock"
)

// --- NHL test fixtures -------------------------------------------------------

// nhlTeamsJSON builds a Stats-API team feed with the given (id, tricode,
// fullName) triples.
func nhlTeamsJSON(teams ...[3]string) string {
	items := make([]map[string]any, 0, len(teams))
	for _, t := range teams {
		items = append(items, map[string]any{
			"id": t[0], "triCode": t[1], "fullName": t[2], "franchiseId": "1",
		})
	}
	b, _ := json.Marshal(map[string]any{"data": items})

	return string(b)
}

func nhlStandingsJSON(date string, rows ...map[string]any) string {
	b, _ := json.Marshal(map[string]any{"standingsDate": date, "standings": rows})

	return string(b)
}

func nhlRosterJSON(players ...map[string]any) string {
	// Put everyone under forwards for simplicity; the service concats all three.
	b, _ := json.Marshal(map[string]any{"forwards": players, "defensemen": []any{}, "goalies": []any{}})

	return string(b)
}

func nhlResp(data string) *nhl.Response {
	return &nhl.Response{Data: json.RawMessage(data), StatusCode: 200}
}

// --- tests -------------------------------------------------------------------

func TestNHLSyncTeamsDeactivateReactivate(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	nhlMock := nhlmocks.NewMockNHL(ctrl)
	otl := otelmocks.NewOtel()

	// First feed: TOR + BOS. Second feed: only TOR (BOS dropped) + updated name.
	gomock.InOrder(
		nhlMock.EXPECT().GetTeams(gomock.Any()).Return(nhlResp(nhlTeamsJSON([3]string{"10", "TOR", "Toronto Maple Leafs"}, [3]string{"6", "BOS", "Boston Bruins"})), nil),
		nhlMock.EXPECT().GetTeams(gomock.Any()).Return(nhlResp(nhlTeamsJSON([3]string{"10", "TOR", "Toronto St Pats"})), nil),
	)

	svc := ingest.NewNHLTeamsService(nhlMock, db, otl, nhlteamRepo.New(db, otl))
	ctx := context.Background()

	res, err := svc.SyncTeams(ctx)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if res.Created != 2 || res.Updated != 0 {
		t.Fatalf("first result = %+v, want created=2", res)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_teams WHERE is_active = TRUE"); got != 2 {
		t.Fatalf("active teams = %d, want 2", got)
	}

	// Second sync: BOS deactivated, TOR updated (not duplicated).
	res2, err := svc.SyncTeams(ctx)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if res2.Updated != 1 || res2.Created != 0 {
		t.Fatalf("second result = %+v, want updated=1", res2)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_teams"); got != 2 {
		t.Fatalf("total teams = %d, want 2 (no dup)", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_teams WHERE is_active = TRUE"); got != 1 {
		t.Fatalf("active teams after re-sync = %d, want 1 (only TOR)", got)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_teams WHERE abbreviation = 'BOS' AND is_active = FALSE"); got != 1 {
		t.Fatalf("BOS should be deactivated")
	}

	var fullName string
	if err := db.Read.QueryRow(`SELECT full_name FROM nhl_teams WHERE team_id = '10'`).Scan(&fullName); err != nil {
		t.Fatalf("tor full_name: %v", err)
	}
	if fullName != "Toronto St Pats" {
		t.Fatalf("full_name = %q, want updated", fullName)
	}
}

func TestNHLSyncStandingsIdempotent(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	nhlMock := nhlmocks.NewMockNHL(ctrl)
	otl := otelmocks.NewOtel()

	// Seed a team so standings can resolve it.
	nhlMock.EXPECT().GetTeams(gomock.Any()).Return(nhlResp(nhlTeamsJSON([3]string{"10", "TOR", "Toronto Maple Leafs"})), nil)
	teamSvc := ingest.NewNHLTeamsService(nhlMock, db, otl, nhlteamRepo.New(db, otl))
	if _, err := teamSvc.SyncTeams(context.Background()); err != nil {
		t.Fatalf("seed teams: %v", err)
	}

	torRow := map[string]any{
		"teamAbbrev":  map[string]any{"default": "TOR"},
		"gamesPlayed": 10, "wins": 7, "losses": 2, "otLosses": 1, "points": 15, "pointPctg": 0.75,
		"divisionName": "Atlantic", "conferenceName": "Eastern",
	}
	// A row whose team is unknown must be skipped (counted as error), not fail.
	unknownRow := map[string]any{"teamAbbrev": "ZZZ", "wins": 1}

	gomock.InOrder(
		nhlMock.EXPECT().GetStandings(gomock.Any()).Return(nhlResp(nhlStandingsJSON("2024-12-15T00:00:00Z", torRow, unknownRow)), nil),
		nhlMock.EXPECT().GetStandings(gomock.Any()).Return(nhlResp(nhlStandingsJSON("2024-12-15T00:00:00Z", torRow)), nil),
	)

	svc := ingest.NewNHLStandingsService(nhlMock, db, otl, nhlteamRepo.New(db, otl), nhlstandingRepo.New(db, otl))
	ctx := context.Background()

	res, err := svc.SyncStandings(ctx)
	if err != nil {
		t.Fatalf("first standings: %v", err)
	}
	if res.Created != 1 || res.Errors != 1 {
		t.Fatalf("first result = %+v, want created=1 errors=1 (unknown skipped)", res)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_standings"); got != 1 {
		t.Fatalf("standings rows = %d, want 1", got)
	}

	// Re-run: upsert by (team_id, date) => updated, not duplicated.
	res2, err := svc.SyncStandings(ctx)
	if err != nil {
		t.Fatalf("second standings: %v", err)
	}
	if res2.Updated != 1 || res2.Created != 0 {
		t.Fatalf("second result = %+v, want updated=1", res2)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_standings"); got != 1 {
		t.Fatalf("standings after re-run = %d, want 1 (idempotent)", got)
	}

	var wins int
	if err := db.Read.QueryRow(`SELECT wins FROM nhl_standings`).Scan(&wins); err != nil {
		t.Fatalf("standing wins: %v", err)
	}
	if wins != 7 {
		t.Fatalf("wins = %d, want 7", wins)
	}
}

func TestNHLSyncRostersLinksPlayersToTeam(t *testing.T) {
	db := startPostgres(t)
	ctrl := gomock.NewController(t)
	nhlMock := nhlmocks.NewMockNHL(ctrl)
	otl := otelmocks.NewOtel()

	// Seed one active team.
	nhlMock.EXPECT().GetTeams(gomock.Any()).Return(nhlResp(nhlTeamsJSON([3]string{"10", "TOR", "Toronto Maple Leafs"})), nil)
	teamSvc := ingest.NewNHLTeamsService(nhlMock, db, otl, nhlteamRepo.New(db, otl))
	if _, err := teamSvc.SyncTeams(context.Background()); err != nil {
		t.Fatalf("seed teams: %v", err)
	}

	player := map[string]any{
		"id":            8479318,
		"firstName":     map[string]any{"default": "Auston"},
		"lastName":      map[string]any{"default": "Matthews"},
		"sweaterNumber": 34, "positionCode": "C", "headshot": "http://x/34.png",
	}

	cfg := &config.Config{}
	cfg.Services.NHL.IngestConcurrency = 4

	nhlMock.EXPECT().GetRoster(gomock.Any(), "TOR").Return(nhlResp(nhlRosterJSON(player)), nil).Times(2)

	svc := ingest.NewNHLRostersService(nhlMock, db, otl, cfg, nhlteamRepo.New(db, otl), nhlplayerRepo.New(db, otl))
	ctx := context.Background()

	res, err := svc.SyncRosters(ctx)
	if err != nil {
		t.Fatalf("first rosters: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("first result = %+v, want created=1", res)
	}

	// Player linked to the seeded team via current_team_id.
	var linked int
	if err := db.Read.QueryRow(`
		SELECT COUNT(*) FROM nhl_players p JOIN nhl_teams t ON t.id = p.current_team_id
		WHERE p.player_id = '8479318' AND t.team_id = '10'`).Scan(&linked); err != nil {
		t.Fatalf("link query: %v", err)
	}
	if linked != 1 {
		t.Fatalf("linked players = %d, want 1", linked)
	}

	// Re-run: upsert by player_id => updated, not duplicated.
	res2, err := svc.SyncRosters(ctx)
	if err != nil {
		t.Fatalf("second rosters: %v", err)
	}
	if res2.Updated != 1 || res2.Created != 0 {
		t.Fatalf("second result = %+v, want updated=1", res2)
	}
	if got := count(t, db, "SELECT COUNT(*) FROM nhl_players"); got != 1 {
		t.Fatalf("players after re-run = %d, want 1 (no dup)", got)
	}
}
