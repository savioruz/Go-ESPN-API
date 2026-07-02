package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/espn"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	injuryModel "go-espn-api/internal/domains/injury/model"
	injuryRepo "go-espn-api/internal/domains/injury/repository"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	teamRepo "go-espn-api/internal/domains/team/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// InjuriesService ingests league injury reports (snapshot refresh) from ESPN.
type InjuriesService struct {
	espn     espn.ESPN
	db       *postgres.Connection
	otel     otel.Otel
	sports   sportRepo.Sport
	leagues  leagueRepo.League
	teams    teamRepo.Team
	injuries injuryRepo.Injury
}

// NewInjuriesService constructs an InjuriesService.
func NewInjuriesService(
	espnClient espn.ESPN,
	db *postgres.Connection,
	otl otel.Otel,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	teams teamRepo.Team,
	injuries injuryRepo.Injury,
) *InjuriesService {
	return &InjuriesService{espn: espnClient, db: db, otel: otl, sports: sports, leagues: leagues, teams: teams, injuries: injuries}
}

// IngestInjuries clears and re-inserts the league injury snapshot.
func (s *InjuriesService) IngestInjuries(ctx context.Context, sport, league string) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.Injuries")
	defer scope.End()

	scope.SetAttributes(map[string]any{"sport": sport, "league": league})

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		leagueID, err := resolveSportLeague(ctx, tx, s.sports, s.leagues, sport, league)
		if err != nil {
			return err
		}

		resp, err := s.espn.GetLeagueInjuries(ctx, sport, league)
		if err != nil {
			return fmt.Errorf("fetch injuries: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode injuries: %w", err)
		}

		items := mslice(root, "items")
		if len(items) == 0 {
			items = mslice(root, "injuries")
		}

		if len(items) == 0 {
			log.Info().Str("sport", sport).Str("league", league).Msg("no_injuries_found")

			return nil
		}

		// Injuries are a snapshot — clear prior entries before re-inserting.
		if err := s.injuries.DeleteByLeagueTx(ctx, tx, leagueID); err != nil {
			return err
		}

		for _, raw := range items {
			m, teamESPNID, ok := parseInjury(leagueID, asMap(raw))
			if !ok {
				result.Errors++

				continue
			}

			teamID, err := s.teams.IDByESPNInLeagueTx(ctx, tx, leagueID, teamESPNID)
			if err != nil {
				return err
			}

			m.TeamID = teamID

			if err := s.injuries.InsertTx(ctx, tx, m); err != nil {
				return err
			}

			result.Created++
		}

		return nil
	})
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("failed to ingest injuries: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "errors": result.Errors})

	return result, nil
}

// parseInjury mirrors _parse_injury. ok=false when the athlete has no name. The
// returned string is the team's ESPN id for later resolution.
func parseInjury(leagueID int64, item map[string]any) (injuryModel.Injury, string, bool) {
	athlete := mmap(item, "athlete")

	athleteName := mstrOr(athlete, "displayName", "fullName")
	if athleteName == "" {
		return injuryModel.Injury{}, "", false
	}

	rawStatus := mstr(item, "status")
	position := mstr(mmap(athlete, "position"), "abbreviation")

	return injuryModel.Injury{
		LeagueID:      leagueID,
		AthleteESPNID: mid(athlete, "id"),
		AthleteName:   athleteName,
		Position:      position,
		Status:        normalizeInjuryStatus(rawStatus),
		StatusDisplay: rawStatus,
		Description:   mstrOr(item, "description", "shortComment"),
		InjuryType:    mstr(item, "type"),
		RawData:       rawObj(item, "{}"),
	}, mid(mmap(item, "team"), "id"), true
}
