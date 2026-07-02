package ingest

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseEventStatus(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		wantStatus string
		wantDetail string
	}{
		{
			name:       "completed wins over state",
			status:     `{"type":{"state":"in","completed":true,"detail":"Final/OT"}}`,
			wantStatus: statusFinal,
			wantDetail: "Final/OT",
		},
		{
			name:       "completed without detail defaults to Final",
			status:     `{"type":{"state":"post","completed":true}}`,
			wantStatus: statusFinal,
			wantDetail: "Final",
		},
		{
			name:       "pre maps to scheduled",
			status:     `{"type":{"state":"pre","completed":false,"detail":""}}`,
			wantStatus: statusScheduled,
			wantDetail: "",
		},
		{
			name:       "in maps to in_progress",
			status:     `{"type":{"state":"in","completed":false,"detail":"1st Quarter"}}`,
			wantStatus: statusInProgress,
			wantDetail: "1st Quarter",
		},
		{
			name:       "post (not completed) maps to final",
			status:     `{"type":{"state":"post","completed":false}}`,
			wantStatus: statusFinal,
			wantDetail: "",
		},
		{
			name:       "unknown state defaults to scheduled",
			status:     `{"type":{"state":"weird"}}`,
			wantStatus: statusScheduled,
			wantDetail: "",
		},
		{
			name:       "missing type defaults to scheduled",
			status:     `{}`,
			wantStatus: statusScheduled,
			wantDetail: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal([]byte(tc.status), &m); err != nil {
				t.Fatalf("bad fixture: %v", err)
			}

			gotStatus, gotDetail := parseEventStatus(m)
			if gotStatus != tc.wantStatus || gotDetail != tc.wantDetail {
				t.Fatalf("got (%q,%q), want (%q,%q)", gotStatus, gotDetail, tc.wantStatus, tc.wantDetail)
			}
		})
	}
}

func TestHomeAwayFallback(t *testing.T) {
	cases := []struct {
		raw  string
		idx  int
		want string
	}{
		{"home", 0, "home"},
		{"away", 1, "away"},
		{"", 0, "away"},
		{"", 1, "home"},
		{"unknown", 1, "home"},
		{"unknown", 2, "away"},
	}

	for _, tc := range cases {
		if got := homeAwayFallback(tc.raw, tc.idx); got != tc.want {
			t.Fatalf("homeAwayFallback(%q,%d) = %q, want %q", tc.raw, tc.idx, got, tc.want)
		}
	}
}

func TestNormalizeInjuryStatus(t *testing.T) {
	cases := map[string]string{
		"Out":             "out",
		"  DOUBTFUL ":     "doubtful",
		"Questionable":    "questionable",
		"Injured Reserve": "ir",
		"IR":              "ir",
		"Day-To-Day":      "day_to_day",
		"Probable":        "day_to_day",
		"Suspended":       "other",
		"":                "other",
	}

	for raw, want := range cases {
		if got := normalizeInjuryStatus(raw); got != want {
			t.Fatalf("normalizeInjuryStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseArticleESPNIDAndHeadline(t *testing.T) {
	// dataSourceIdentifier wins over id; headline truncated to 500.
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}

	item := map[string]any{
		"dataSourceIdentifier": "ds-123",
		"id":                   float64(999),
		"headline":             string(long),
	}

	m, ok := parseArticle(7, item)
	if !ok {
		t.Fatal("expected ok")
	}

	if m.ESPNID != "ds-123" {
		t.Fatalf("espn_id = %q, want ds-123", m.ESPNID)
	}

	if len(m.Headline) != 500 {
		t.Fatalf("headline len = %d, want 500", len(m.Headline))
	}

	// Falls back to id when dataSourceIdentifier absent; title when no headline.
	m2, ok := parseArticle(7, map[string]any{"id": float64(42), "title": "Hello"})
	if !ok || m2.ESPNID != "42" || m2.Headline != "Hello" {
		t.Fatalf("fallback parse wrong: ok=%v espn=%q headline=%q", ok, m2.ESPNID, m2.Headline)
	}

	// Missing headline => not ok (counted as error upstream).
	if _, ok := parseArticle(7, map[string]any{"id": float64(1)}); ok {
		t.Fatal("expected not ok when headline missing")
	}
}

func TestParseEventTime(t *testing.T) {
	got := parseEventTime("2024-12-15T20:00Z")
	want := time.Date(2024, 12, 15, 20, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseEventTime = %v, want %v", got, want)
	}

	// Invalid falls back to ~now.
	fb := parseEventTime("not-a-date")
	if time.Since(fb) > time.Minute {
		t.Fatalf("fallback not near now: %v", fb)
	}
}

func TestParseDTPtrAndDateOnly(t *testing.T) {
	if parseDTPtr("") != nil {
		t.Fatal("empty should be nil")
	}

	if parseDTPtr("garbage") != nil {
		t.Fatal("invalid should be nil")
	}

	dt := parseDTPtr("2024-01-02T03:04:05Z")
	if dt == nil || !dt.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("parseDTPtr wrong: %v", dt)
	}

	d := parseDateOnly("2024-06-07T00:00:00Z")
	if d == nil || d.Year() != 2024 || d.Month() != 6 || d.Day() != 7 {
		t.Fatalf("parseDateOnly wrong: %v", d)
	}

	if parseDateOnly("short") != nil {
		t.Fatal("short should be nil")
	}
}

func TestLeagueDefaultsFallback(t *testing.T) {
	// Unknown slug falls back to title-case name and upper[:10] abbreviation.
	name, abbr := leagueDefaults("some-made-up-league-slug")
	if name != "Some Made Up League Slug" {
		t.Fatalf("name = %q", name)
	}

	if abbr != "SOME-MADE-" {
		t.Fatalf("abbr = %q, want SOME-MADE-", abbr)
	}

	// Known slug uses the registry.
	n2, a2 := leagueDefaults("nba")
	if n2 != "National Basketball Association" || a2 != "NBA" {
		t.Fatalf("registry lookup wrong: %q %q", n2, a2)
	}
}
