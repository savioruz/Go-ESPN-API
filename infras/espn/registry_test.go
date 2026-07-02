package espn

import "testing"

func TestRegistryCounts(t *testing.T) {
	if len(SportNames) != 17 {
		t.Fatalf("expected 17 sports, got %d", len(SportNames))
	}
	if len(LeagueInfo) != 139 {
		t.Fatalf("expected 139 leagues, got %d", len(LeagueInfo))
	}
}

func TestRegistryKnownEntries(t *testing.T) {
	if got := SportNames["basketball"]; got != "Basketball" {
		t.Fatalf("basketball => %q", got)
	}
	if got := SportNames["football"]; got != "Football" {
		t.Fatalf("football => %q", got)
	}

	cases := map[string]struct{ name, abbr string }{
		"nba":        {"National Basketball Association", "NBA"},
		"nfl":        {"National Football League", "NFL"},
		"fifa.world": {"FIFA World Cup", "WC"},
	}
	for slug, want := range cases {
		got, ok := LeagueInfo[slug]
		if !ok {
			t.Fatalf("missing league %q", slug)
		}
		if got.Name != want.name || got.Abbreviation != want.abbr {
			t.Fatalf("%s => %+v, want %+v", slug, got, want)
		}
	}
}
