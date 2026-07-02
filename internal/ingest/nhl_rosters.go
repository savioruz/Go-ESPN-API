package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"go-espn-api/config"
	"go-espn-api/infras/nhl"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	nhlplayerRepo "go-espn-api/internal/domains/nhlplayer/repository"
	nhlteamModel "go-espn-api/internal/domains/nhlteam/model"
	nhlteamRepo "go-espn-api/internal/domains/nhlteam/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// NHLRostersService ingests current rosters for all active NHL teams. It ports
// sync_team_rosters: each active team's roster (forwards + defensemen + goalies)
// is upserted inside its own transaction; a team whose fetch or write fails is
// logged and skipped without aborting the others. The per-team loop is bounded
// by SERVICES_NHL_INGEST_CONCURRENCY to keep concurrent DB writes within the
// Postgres pool.
type NHLRostersService struct {
	nhl         nhl.NHL
	db          *postgres.Connection
	otel        otel.Otel
	teams       nhlteamRepo.NHLTeam
	players     nhlplayerRepo.NHLPlayer
	concurrency int
}

// NewNHLRostersService constructs an NHLRostersService, clamping the ingest
// concurrency to a sane minimum.
func NewNHLRostersService(
	client nhl.NHL,
	db *postgres.Connection,
	otl otel.Otel,
	cfg *config.Config,
	teams nhlteamRepo.NHLTeam,
	players nhlplayerRepo.NHLPlayer,
) *NHLRostersService {
	concurrency := cfg.Services.NHL.IngestConcurrency
	if concurrency < 1 {
		concurrency = 1
	}

	return &NHLRostersService{nhl: client, db: db, otel: otl, teams: teams, players: players, concurrency: concurrency}
}

// SyncRosters fetches and upserts current players for every active team, fanning
// out over teams with a bounded worker pool. A failing team is counted and
// skipped, never aborting the run.
func (s *NHLRostersService) SyncRosters(ctx context.Context) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.NHLRosters")
	defer scope.End()

	teams, err := s.teams.ListActive(ctx)
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("list active nhl teams: %w", err)
	}

	var created, updated, errCount int64

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for _, team := range teams {
		team := team

		g.Go(func() error {
			// Panic isolation: a panic on one team must not abort the run.
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&errCount, 1)
					log.Error().Str("team", team.Abbreviation).Interface("panic", r).Msg("nhl roster ingest panicked")
				}
			}()

			if gctx.Err() != nil {
				return nil
			}

			res, terr := s.syncTeamRoster(gctx, team)
			if terr != nil {
				atomic.AddInt64(&errCount, 1)
				log.Error().Err(terr).Str("team", team.Abbreviation).Msg("failed to sync nhl roster")

				// Errors are counted, not propagated: one failing team never
				// cancels the group (Django logs+skips per team).
				return nil
			}

			atomic.AddInt64(&created, int64(res.Created))
			atomic.AddInt64(&updated, int64(res.Updated))
			atomic.AddInt64(&errCount, int64(res.Errors))

			return nil
		})
	}

	_ = g.Wait()

	result := newResult()
	result.Created = int(created)
	result.Updated = int(updated)
	result.Errors = int(errCount)

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}

// syncTeamRoster fetches one team's roster and upserts its players inside a
// single transaction. On any error the transaction is rolled back and the whole
// team is reported as failed (empty result + error).
func (s *NHLRostersService) syncTeamRoster(ctx context.Context, team nhlteamModel.NHLTeam) (IngestionResult, error) {
	resp, err := s.nhl.GetRoster(ctx, team.Abbreviation)
	if err != nil {
		return IngestionResult{}, fmt.Errorf("fetch roster %s: %w", team.Abbreviation, err)
	}

	var root map[string]any
	if err := json.Unmarshal(resp.Data, &root); err != nil {
		return IngestionResult{}, fmt.Errorf("decode roster %s: %w", team.Abbreviation, err)
	}

	forwards := mslice(root, "forwards")
	defensemen := mslice(root, "defensemen")
	goalies := mslice(root, "goalies")

	players := make([]any, 0, len(forwards)+len(defensemen)+len(goalies))
	players = append(players, forwards...)
	players = append(players, defensemen...)
	players = append(players, goalies...)

	result := newResult()

	err = withTx(s.db, func(tx *sqlx.Tx) error {
		for _, raw := range players {
			m, playerID := parseNHLPlayer(asMap(raw), team.ID)
			if playerID == "" {
				result.Errors++

				continue
			}

			created, uerr := s.players.UpsertTx(ctx, tx, m)
			if uerr != nil {
				return uerr
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
		return IngestionResult{}, err
	}

	return result, nil
}
