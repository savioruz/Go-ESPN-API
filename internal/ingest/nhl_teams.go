package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/nhl"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	nhlteamRepo "go-espn-api/internal/domains/nhlteam/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// NHLTeamsService ingests NHL team data from the Stats REST API. It ports
// sync_teams: all teams are deactivated then re-activated as they appear in the
// feed, within a single transaction.
type NHLTeamsService struct {
	nhl   nhl.NHL
	db    *postgres.Connection
	otel  otel.Otel
	teams nhlteamRepo.NHLTeam
}

// NewNHLTeamsService constructs an NHLTeamsService.
func NewNHLTeamsService(
	client nhl.NHL,
	db *postgres.Connection,
	otl otel.Otel,
	teams nhlteamRepo.NHLTeam,
) *NHLTeamsService {
	return &NHLTeamsService{nhl: client, db: db, otel: otl, teams: teams}
}

// SyncTeams fetches all NHL teams and upserts them by team_id. All teams are
// deactivated first so any not present in the feed are left inactive.
func (s *NHLTeamsService) SyncTeams(ctx context.Context) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.NHLTeams")
	defer scope.End()

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		resp, err := s.nhl.GetTeams(ctx)
		if err != nil {
			return fmt.Errorf("fetch nhl teams: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode nhl teams: %w", err)
		}

		teams := mslice(root, "data")
		if len(teams) == 0 {
			log.Warn().Msg("no_nhl_teams_found")

			return nil
		}

		if err := s.teams.DeactivateAllTx(ctx, tx); err != nil {
			return err
		}

		for _, raw := range teams {
			m, teamID := parseNHLTeam(asMap(raw))
			if teamID == "" {
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

		return IngestionResult{}, fmt.Errorf("failed to ingest nhl teams: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}
