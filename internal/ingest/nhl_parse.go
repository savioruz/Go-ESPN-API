package ingest

import (
	"strings"
	"time"

	nhlplayerModel "go-espn-api/internal/domains/nhlplayer/model"
	nhlstandingModel "go-espn-api/internal/domains/nhlstanding/model"
	nhlteamModel "go-espn-api/internal/domains/nhlteam/model"
)

// The NHL feeds decode into map[string]any trees navigated with the shared
// lenient accessors (asMap/mslice/mstr/…). These helpers add the NHL-specific
// shapes the Python ingestion relied on: nested {"default": "..."} localized
// strings, triCode/rawTricode fallback, and the last-word team name.

// lastWord returns the final whitespace-separated token of s (Python
// name.split()[-1]), or "" when s is blank.
func lastWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}

	return fields[len(fields)-1]
}

// defaultStr extracts m[key]["default"] as a string (the NHL localized-string
// shape, e.g. firstName: {"default": "Connor"}). Returns "" when absent.
func defaultStr(m map[string]any, key string) string {
	return mstr(mmap(m, key), "default")
}

// nhlTeamAbbrev reads teamAbbrev, tolerating both the dict form
// ({"default": "TOR"}) and a flat string ("TOR"). Returns "" when absent.
func nhlTeamAbbrev(row map[string]any) string {
	switch v := row["teamAbbrev"].(type) {
	case map[string]any:
		return mstr(v, "default")
	case string:
		return v
	default:
		return ""
	}
}

// mfloat returns m[key] as a float64, defaulting to def when absent/not numeric.
func mfloat(m map[string]any, key string, def float64) float64 {
	f, ok := m[key].(float64)
	if !ok {
		return def
	}

	return f
}

// parseStandingsDate reads standingsDate and slices its leading YYYY-MM-DD
// (Python date_str[:10]). It falls back to today (UTC) when absent/unparseable.
func parseStandingsDate(root map[string]any) time.Time {
	raw := mstr(root, "standingsDate")
	if len(raw) >= dateLen {
		if t, err := time.Parse("2006-01-02", raw[:dateLen]); err == nil {
			return t
		}
	}

	now := time.Now().UTC()

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// parseNHLTeam mirrors sync_teams' per-team mapping. abbreviation prefers
// triCode then rawTricode; name is the last word of fullName; the team is
// always marked active (the caller deactivates all first). Returns the model
// and its team_id ("" when blank).
func parseNHLTeam(t map[string]any) (nhlteamModel.NHLTeam, string) {
	teamID := mid(t, "id")
	fullName := mstr(t, "fullName")

	return nhlteamModel.NHLTeam{
		TeamID:       teamID,
		Abbreviation: mstrOr(t, "triCode", "rawTricode"),
		Name:         lastWord(fullName),
		FullName:     fullName,
		FranchiseID:  mid(t, "franchiseId"),
		IsActive:     true,
		RawData:      rawObj(t, "{}"),
	}, teamID
}

// parseNHLPlayer mirrors sync_team_rosters' per-player mapping. first/last names
// come from the {"default": ...} localized shape; full_name is their trimmed
// join; sweater number coerces a numeric to its text form. Returns the model
// and its player_id ("" when blank).
func parseNHLPlayer(p map[string]any, teamID int64) (nhlplayerModel.NHLPlayer, string) {
	playerID := mid(p, "id")
	first := defaultStr(p, "firstName")
	last := defaultStr(p, "lastName")
	tid := teamID

	return nhlplayerModel.NHLPlayer{
		PlayerID:      playerID,
		FirstName:     first,
		LastName:      last,
		FullName:      strings.TrimSpace(first + " " + last),
		SweaterNumber: mid(p, "sweaterNumber"),
		Position:      mstr(p, "positionCode"),
		CurrentTeamID: &tid,
		IsActive:      true,
		HeadshotURL:   mstr(p, "headshot"),
		RawData:       rawObj(p, "{}"),
	}, playerID
}

// parseNHLStanding mirrors sync_standings' per-row mapping for an already
// resolved team id and sync date.
func parseNHLStanding(row map[string]any, teamID int64, date time.Time) nhlstandingModel.NHLStanding {
	pct := mfloat(row, "pointPctg", 0.0)

	return nhlstandingModel.NHLStanding{
		TeamID:         teamID,
		Date:           date,
		GamesPlayed:    mint(row, "gamesPlayed", 0),
		Wins:           mint(row, "wins", 0),
		Losses:         mint(row, "losses", 0),
		OTLosses:       mint(row, "otLosses", 0),
		Points:         mint(row, "points", 0),
		PointPct:       &pct,
		DivisionName:   mstr(row, "divisionName"),
		ConferenceName: mstr(row, "conferenceName"),
		RawData:        rawObj(row, "{}"),
	}
}
