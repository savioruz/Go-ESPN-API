package ingest

import (
	"context"
	"strings"

	"go-espn-api/infras/espn"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"

	"github.com/jmoiron/sqlx"
)

// maxAbbreviationLen caps the fallback league abbreviation (Python slug[:10]).
const maxAbbreviationLen = 10

// resolveSportLeague get-or-creates the Sport and League rows for the given
// slugs and returns the league id. It mirrors get_or_create_sport_and_league,
// sourcing display names/abbreviations from the ESPN registry with the same
// fallbacks (title-cased slug / upper[:10]).
func resolveSportLeague(
	ctx context.Context,
	tx *sqlx.Tx,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	sportSlug, leagueSlug string,
) (int64, error) {
	sportName, ok := espn.SportNames[sportSlug]
	if !ok {
		sportName = titleCase(strings.ReplaceAll(sportSlug, "-", " "))
	}

	sportID, err := sports.GetOrCreateTx(ctx, tx, sportSlug, sportName)
	if err != nil {
		return 0, err
	}

	leagueName, leagueAbbr := leagueDefaults(leagueSlug)

	leagueID, err := leagues.GetOrCreateTx(ctx, tx, sportID, leagueSlug, leagueName, leagueAbbr)
	if err != nil {
		return 0, err
	}

	return leagueID, nil
}

// leagueDefaults returns the league name/abbreviation from the registry, or the
// Python fallbacks (title-cased slug, upper-cased slug truncated to 10).
func leagueDefaults(leagueSlug string) (string, string) {
	if meta, ok := espn.LeagueInfo[leagueSlug]; ok {
		return meta.Name, meta.Abbreviation
	}

	name := titleCase(strings.ReplaceAll(leagueSlug, "-", " "))

	abbr := strings.ToUpper(leagueSlug)
	if len(abbr) > maxAbbreviationLen {
		abbr = abbr[:maxAbbreviationLen]
	}

	return name, abbr
}

// titleCase capitalises the first letter of each space-separated word, matching
// Python str.title() closely enough for the slug fallbacks (ASCII slugs).
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}

	return strings.Join(words, " ")
}
