package ingest

import (
	"strings"
	"time"
)

// Event status values (mirroring apps.espn.models.Event constants and the
// events.status DB column).
const (
	statusScheduled  = "scheduled"
	statusInProgress = "in_progress"
	statusFinal      = "final"
)

// Competitor home/away values.
const (
	homeAwayHome = "home"
	homeAwayAway = "away"
)

// dateLen is the length of an ISO date prefix ("2006-01-02").
const dateLen = 10

// isoLayouts covers the ISO-8601 shapes ESPN emits. Python's
// datetime.fromisoformat accepts an optional seconds component, whereas Go's
// time.RFC3339 requires seconds — ESPN scoreboard dates frequently omit them
// (e.g. "2024-12-15T20:00Z"), so both forms are tried.
var isoLayouts = []string{
	time.RFC3339,             // 2006-01-02T15:04:05Z07:00
	"2006-01-02T15:04Z07:00", // seconds omitted
}

// parseISO parses an ISO-8601 timestamp (ESPN uses a trailing Z), returning
// ok=false when the input is empty or unparseable. Mirrors the Python
// datetime.fromisoformat(date.replace("Z", "+00:00")).
func parseISO(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}

	normalized := strings.Replace(s, "Z", "+00:00", 1)
	for _, layout := range isoLayouts {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

// parseEventTime parses an ISO-8601 timestamp, falling back to the current time
// on failure (Python fromisoformat with a now() fallback).
func parseEventTime(s string) time.Time {
	if t, ok := parseISO(s); ok {
		return t
	}

	return time.Now()
}

// parseDTPtr parses an ISO-8601 timestamp into a *time.Time, returning nil for
// empty/invalid input (Python _parse_dt).
func parseDTPtr(s string) *time.Time {
	t, ok := parseISO(s)
	if !ok {
		return nil
	}

	return &t
}

// parseDateOnly parses the leading YYYY-MM-DD of an ISO date into a *time.Time
// (Python date.fromisoformat(raw[:10])). Returns nil on empty/invalid input.
func parseDateOnly(s string) *time.Time {
	if len(s) < dateLen {
		return nil
	}

	t, err := time.Parse("2006-01-02", s[:dateLen])
	if err != nil {
		return nil
	}

	return &t
}

// parseEventStatus maps an ESPN status object to (status, detail). It mirrors
// ScoreboardIngestionService._parse_event_status: a completed type is always
// final; otherwise the type.state is mapped pre→scheduled, in→in_progress,
// post→final (unknown → scheduled).
func parseEventStatus(statusData map[string]any) (string, string) {
	typeData := mmap(statusData, "type")

	state := mstr(typeData, "state")
	if state == "" {
		state = "pre"
	}

	completed := mbool(typeData, "completed", false)
	detail := mstr(typeData, "detail")

	if completed {
		if detail == "" {
			detail = "Final"
		}

		return statusFinal, detail
	}

	switch state {
	case "pre":
		return statusScheduled, detail
	case "in":
		return statusInProgress, detail
	case "post":
		return statusFinal, detail
	default:
		return statusScheduled, detail
	}
}

// homeAwayFallback normalises the competitor homeAway flag. When it is not a
// recognised "home"/"away" value, index 1 becomes home and everything else
// away (Python: home if idx == 1 else away).
func homeAwayFallback(raw string, idx int) string {
	if raw == homeAwayHome || raw == homeAwayAway {
		return raw
	}

	if idx == 1 {
		return homeAwayHome
	}

	return homeAwayAway
}

// injuryStatusMap mirrors InjuryIngestionService._STATUS_MAP.
var injuryStatusMap = map[string]string{
	"out":             "out",
	"doubtful":        "doubtful",
	"questionable":    "questionable",
	"injured reserve": "ir",
	"ir":              "ir",
	"day-to-day":      "day_to_day",
	"probable":        "day_to_day",
}

// normalizeInjuryStatus lower/trims the raw status and maps it, defaulting to
// "other" for anything unrecognised.
func normalizeInjuryStatus(raw string) string {
	if v, ok := injuryStatusMap[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return v
	}

	return "other"
}

// truncate returns s truncated to at most n bytes (headlines are capped at 500,
// matching Python headline[:500]). ESPN headlines are ASCII in practice.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n]
}
