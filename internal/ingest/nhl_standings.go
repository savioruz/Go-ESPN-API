package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/nhl"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	nhlstandingRepo "go-espn-api/internal/domains/nhlstanding/repository"
	nhlteamRepo "go-espn-api/internal/domains/nhlteam/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// NHLStandingsService ingests NHL standings from the Web API. It ports
// sync_standings: each row is resolved to a team by abbreviation and upserted by
// (team_id, date), all within a single transaction.
type NHLStandingsService struct {
	nhl       nhl.NHL
	db        *postgres.Connection
	otel      otel.Otel
	teams     nhlteamRepo.NHLTeam
	standings nhlstandingRepo.NHLStanding
}

// NewNHLStandingsService constructs an NHLStandingsService.
func NewNHLStandingsService(
	client nhl.NHL,
	db *postgres.Connection,
	otl otel.Otel,
	teams nhlteamRepo.NHLTeam,
	standings nhlstandingRepo.NHLStanding,
) *NHLStandingsService {
	return &NHLStandingsService{nhl: client, db: db, otel: otl, teams: teams, standings: standings}
}

// SyncStandings fetches the current standings and upserts one row per team for
// the standings date. Rows whose team is not in the DB are skipped with a warn.
func (s *NHLStandingsService) SyncStandings(ctx context.Context) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.NHLStandings")
	defer scope.End()

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		resp, err := s.nhl.GetStandings(ctx)
		if err != nil {
			return fmt.Errorf("fetch nhl standings: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode nhl standings: %w", err)
		}

		rows := mslice(root, "standings")
		if len(rows) == 0 {
			log.Warn().Msg("no_nhl_standings_found")

			return nil
		}

		date := parseStandingsDate(root)

		for _, raw := range rows {
			row := asMap(raw)
			abbrev := nhlTeamAbbrev(row)

			teamID, err := s.teams.IDByAbbrevTx(ctx, tx, abbrev)
			if err != nil {
				return err
			}

			if teamID == nil {
				log.Warn().Str("abbreviation", abbrev).Msg("nhl_standings_team_not_found_run_sync_teams_first")

				result.Errors++

				continue
			}

			created, err := s.standings.UpsertTx(ctx, tx, parseNHLStanding(row, *teamID, date))
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

		return IngestionResult{}, fmt.Errorf("failed to ingest nhl standings: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}
