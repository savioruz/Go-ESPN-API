package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/espn"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	athleteRepo "go-espn-api/internal/domains/athlete/repository"
	athleteStatsModel "go-espn-api/internal/domains/athletestats/model"
	athleteStatsRepo "go-espn-api/internal/domains/athletestats/repository"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
)

// AthleteStatsService ingests athlete season stats from the common/v3 endpoint.
// It has no HTTP route; it is provided for scheduler use.
type AthleteStatsService struct {
	espn     espn.ESPN
	db       *postgres.Connection
	otel     otel.Otel
	sports   sportRepo.Sport
	leagues  leagueRepo.League
	athletes athleteRepo.Athlete
	stats    athleteStatsRepo.AthleteStats
}

// NewAthleteStatsService constructs an AthleteStatsService.
func NewAthleteStatsService(
	espnClient espn.ESPN,
	db *postgres.Connection,
	otl otel.Otel,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	athletes athleteRepo.Athlete,
	stats athleteStatsRepo.AthleteStats,
) *AthleteStatsService {
	return &AthleteStatsService{espn: espnClient, db: db, otel: otl, sports: sports, leagues: leagues, athletes: athletes, stats: stats}
}

// IngestAthleteStats fetches and upserts season stats for a single athlete.
func (s *AthleteStatsService) IngestAthleteStats(ctx context.Context, sport, league, athleteESPNID string, season, seasonType int) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.AthleteStats")
	defer scope.End()

	scope.SetAttributes(map[string]any{"sport": sport, "league": league, "athlete_espn_id": athleteESPNID})

	if seasonType <= 0 {
		seasonType = 2
	}

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		leagueID, err := resolveSportLeague(ctx, tx, s.sports, s.leagues, sport, league)
		if err != nil {
			return err
		}

		resp, err := s.espn.GetAthleteStats(ctx, sport, league, athleteESPNID, season, seasonType)
		if err != nil {
			return fmt.Errorf("fetch athlete stats: %w", err)
		}

		var data map[string]any
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return fmt.Errorf("decode athlete stats: %w", err)
		}

		ref, err := s.athletes.GetRefByESPNIDTx(ctx, tx, athleteESPNID)
		if err != nil {
			return err
		}

		var athleteID *int64

		athleteName := athleteESPNID

		if ref != nil {
			athleteID = &ref.ID
			athleteName = ref.DisplayName
		}

		seasonVal := season
		if seasonVal == 0 {
			seasonVal = mint(mmap(data, "season"), "year", 0)
		}

		stats := data["stats"]
		if stats == nil {
			stats = data["splits"]
		}

		m := athleteStatsModel.AthleteSeasonStats{
			AthleteID:     athleteID,
			LeagueID:      leagueID,
			AthleteESPNID: athleteESPNID,
			AthleteName:   athleteName,
			SeasonYear:    seasonVal,
			SeasonType:    seasonType,
			Stats:         rawObj(stats, "{}"),
			RawData:       rawObj(data, "{}"),
		}

		created, err := s.stats.UpsertTx(ctx, tx, m)
		if err != nil {
			return err
		}

		if created {
			result.Created++
		} else {
			result.Updated++
		}

		return nil
	})
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("failed to ingest athlete stats: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated})

	return result, nil
}
