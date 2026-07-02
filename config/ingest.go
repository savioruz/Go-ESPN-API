package config

import "strings"

// IngestLeague is a single (sport, league) slug pair the periodic ingest jobs
// fan out over.
type IngestLeague struct {
	Sport  string
	League string
}

// ParseIngestLeagues converts raw "sport:league" strings into IngestLeague
// pairs. It mirrors base.py's _parse_ingest_leagues exactly: split on the first
// colon, trim whitespace on both halves, and drop any entry missing either
// half.
func ParseIngestLeagues(raw []string) []IngestLeague {
	pairs := make([]IngestLeague, 0, len(raw))

	for _, item := range raw {
		sport, league, _ := strings.Cut(item, ":")
		sport = strings.TrimSpace(sport)
		league = strings.TrimSpace(league)

		if sport != "" && league != "" {
			pairs = append(pairs, IngestLeague{Sport: sport, League: league})
		}
	}

	return pairs
}
