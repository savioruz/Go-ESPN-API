package ingest

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLastWord(t *testing.T) {
	cases := map[string]string{
		"Toronto Maple Leafs": "Leafs",
		"Canadiens":           "Canadiens",
		"  St. Louis  Blues ": "Blues",
		"":                    "",
		"   ":                 "",
	}

	for in, want := range cases {
		if got := lastWord(in); got != want {
			t.Fatalf("lastWord(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseNHLTeamTricodeFallback(t *testing.T) {
	// triCode wins when present.
	m, id := parseNHLTeam(map[string]any{
		"id": float64(10), "fullName": "Toronto Maple Leafs",
		"triCode": "TOR", "rawTricode": "TOR2", "franchiseId": float64(5),
	})
	if id != "10" {
		t.Fatalf("team id = %q, want 10", id)
	}
	if m.Abbreviation != "TOR" {
		t.Fatalf("abbrev = %q, want TOR (triCode wins)", m.Abbreviation)
	}
	if m.Name != "Leafs" {
		t.Fatalf("name = %q, want Leafs", m.Name)
	}
	if m.FullName != "Toronto Maple Leafs" {
		t.Fatalf("full_name = %q", m.FullName)
	}
	if m.FranchiseID != "5" {
		t.Fatalf("franchise_id = %q, want 5", m.FranchiseID)
	}
	if !m.IsActive {
		t.Fatal("team must be active")
	}

	// Falls back to rawTricode when triCode blank/absent.
	m2, _ := parseNHLTeam(map[string]any{"id": float64(2), "fullName": "Boston Bruins", "rawTricode": "BOS"})
	if m2.Abbreviation != "BOS" {
		t.Fatalf("abbrev = %q, want BOS (rawTricode fallback)", m2.Abbreviation)
	}

	// Blank id => empty id (caller counts as error).
	_, id3 := parseNHLTeam(map[string]any{"fullName": "No Id"})
	if id3 != "" {
		t.Fatalf("id = %q, want empty", id3)
	}
}

func TestParseNHLPlayerDefaultExtraction(t *testing.T) {
	p := map[string]any{
		"id":            float64(8478402),
		"firstName":     map[string]any{"default": "Connor"},
		"lastName":      map[string]any{"default": "McDavid"},
		"sweaterNumber": float64(97),
		"positionCode":  "C",
		"headshot":      "http://x/mcdavid.png",
	}

	m, id := parseNHLPlayer(p, 42)
	if id != "8478402" {
		t.Fatalf("player id = %q", id)
	}
	if m.FirstName != "Connor" || m.LastName != "McDavid" {
		t.Fatalf("names = %q/%q", m.FirstName, m.LastName)
	}
	if m.FullName != "Connor McDavid" {
		t.Fatalf("full_name = %q", m.FullName)
	}
	if m.SweaterNumber != "97" {
		t.Fatalf("sweater = %q, want 97", m.SweaterNumber)
	}
	if m.Position != "C" {
		t.Fatalf("position = %q", m.Position)
	}
	if m.CurrentTeamID == nil || *m.CurrentTeamID != 42 {
		t.Fatalf("current_team_id = %v, want 42", m.CurrentTeamID)
	}
	if !m.IsActive {
		t.Fatal("player must be active")
	}

	// Missing localized names => empty, full_name trims to "".
	m2, _ := parseNHLPlayer(map[string]any{"id": float64(1)}, 1)
	if m2.FirstName != "" || m2.LastName != "" || m2.FullName != "" {
		t.Fatalf("empty-name parse wrong: %q/%q/%q", m2.FirstName, m2.LastName, m2.FullName)
	}
}

func TestNHLTeamAbbrevDictOrString(t *testing.T) {
	if got := nhlTeamAbbrev(map[string]any{"teamAbbrev": map[string]any{"default": "TOR"}}); got != "TOR" {
		t.Fatalf("dict form = %q, want TOR", got)
	}
	if got := nhlTeamAbbrev(map[string]any{"teamAbbrev": "BOS"}); got != "BOS" {
		t.Fatalf("string form = %q, want BOS", got)
	}
	if got := nhlTeamAbbrev(map[string]any{}); got != "" {
		t.Fatalf("absent = %q, want empty", got)
	}
}

func TestParseStandingsDateSlice(t *testing.T) {
	// Full datetime string sliced to YYYY-MM-DD.
	root := map[string]any{"standingsDate": "2024-12-15T00:00:00Z"}
	got := parseStandingsDate(root)
	want := time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseStandingsDate = %v, want %v", got, want)
	}

	// Plain date.
	if got := parseStandingsDate(map[string]any{"standingsDate": "2024-01-02"}); !got.Equal(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("plain date = %v", got)
	}

	// Missing => today (UTC midnight), within a day of now.
	fb := parseStandingsDate(map[string]any{})
	now := time.Now().UTC()
	if fb.Year() != now.Year() || fb.YearDay() != now.YearDay() {
		t.Fatalf("fallback date = %v, want today", fb)
	}
}

func TestParseNHLStanding(t *testing.T) {
	var row map[string]any
	if err := json.Unmarshal([]byte(`{
		"gamesPlayed": 30, "wins": 20, "losses": 8, "otLosses": 2,
		"points": 42, "pointPctg": 0.7, "divisionName": "Atlantic",
		"conferenceName": "Eastern"}`), &row); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	date := time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC)
	m := parseNHLStanding(row, 7, date)
	if m.TeamID != 7 || !m.Date.Equal(date) {
		t.Fatalf("team/date = %d/%v", m.TeamID, m.Date)
	}
	if m.GamesPlayed != 30 || m.Wins != 20 || m.Losses != 8 || m.OTLosses != 2 || m.Points != 42 {
		t.Fatalf("record fields wrong: %+v", m)
	}
	if m.PointPct == nil || *m.PointPct != 0.7 {
		t.Fatalf("point_pct = %v, want 0.7", m.PointPct)
	}
	if m.DivisionName != "Atlantic" || m.ConferenceName != "Eastern" {
		t.Fatalf("div/conf = %q/%q", m.DivisionName, m.ConferenceName)
	}

	// Missing pointPctg defaults to 0.0 (Django default), not nil.
	m2 := parseNHLStanding(map[string]any{}, 1, date)
	if m2.PointPct == nil || *m2.PointPct != 0.0 {
		t.Fatalf("default point_pct = %v, want 0.0", m2.PointPct)
	}
}
