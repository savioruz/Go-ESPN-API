package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/espn"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	teamModel "go-espn-api/internal/domains/team/model"
	teamRepo "go-espn-api/internal/domains/team/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// TeamsService ingests team data from ESPN.
type TeamsService struct {
	espn    espn.ESPN
	db      *postgres.Connection
	otel    otel.Otel
	sports  sportRepo.Sport
	leagues leagueRepo.League
	teams   teamRepo.Team
}

// NewTeamsService constructs a TeamsService.
func NewTeamsService(
	espnClient espn.ESPN,
	db *postgres.Connection,
	otl otel.Otel,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	teams teamRepo.Team,
) *TeamsService {
	return &TeamsService{espn: espnClient, db: db, otel: otl, sports: sports, leagues: leagues, teams: teams}
}

// IngestTeams fetches and upserts all teams for a sport and league.
func (s *TeamsService) IngestTeams(ctx context.Context, sport, league string) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.Teams")
	defer scope.End()

	scope.SetAttributes(map[string]any{"sport": sport, "league": league})

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		leagueID, err := resolveSportLeague(ctx, tx, s.sports, s.leagues, sport, league)
		if err != nil {
			return err
		}

		resp, err := s.espn.GetTeams(ctx, sport, league, 0)
		if err != nil {
			return fmt.Errorf("fetch teams: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode teams: %w", err)
		}

		teams := teamsFromResponse(root)
		if len(teams) == 0 {
			log.Warn().Str("sport", sport).Str("league", league).Msg("no_teams_found")

			return nil
		}

		for _, raw := range teams {
			m, espnID := parseTeam(leagueID, asMap(raw))
			if espnID == "" {
				result.Errors++

				continue
			}

			created, err := s.teams.UpsertTx(ctx, tx, m)
			if err != nil {
				return err
			}

			if created {
				result.Created++
			} else {
				result.Updated++
			}
		}

		return nil
	})
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("failed to ingest teams: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}

// teamsFromResponse navigates data["sports"][0]["leagues"][0]["teams"].
func teamsFromResponse(root map[string]any) []any {
	sports := mslice(root, "sports")
	if len(sports) == 0 {
		return nil
	}

	leagues := mslice(asMap(sports[0]), "leagues")
	if len(leagues) == 0 {
		return nil
	}

	return mslice(asMap(leagues[0]), "teams")
}

// parseTeam mirrors _parse_team_data; team_info is data["team"] when present,
// otherwise the item itself. Returns the model and its espn id ("" when blank).
func parseTeam(leagueID int64, teamData map[string]any) (teamModel.Team, string) {
	info := teamData
	if nested := mmap(teamData, "team"); nested != nil {
		info = nested
	}

	espnID := mid(info, "id")

	return teamModel.Team{
		LeagueID:         leagueID,
		ESPNID:           espnID,
		UID:              mstr(info, "uid"),
		Slug:             mstr(info, "slug"),
		Abbreviation:     mstr(info, "abbreviation"),
		DisplayName:      mstr(info, "displayName"),
		ShortDisplayName: mstr(info, "shortDisplayName"),
		Name:             mstr(info, "name"),
		Nickname:         mstr(info, "nickname"),
		Location:         mstr(info, "location"),
		Color:            mstr(info, "color"),
		AlternateColor:   mstr(info, "alternateColor"),
		IsActive:         mbool(info, "isActive", true),
		IsAllStar:        mbool(info, "isAllStar", false),
		Logos:            rawObj(info["logos"], "[]"),
		Links:            rawObj(info["links"], "[]"),
		RawData:          rawObj(info, "{}"),
	}, espnID
}
