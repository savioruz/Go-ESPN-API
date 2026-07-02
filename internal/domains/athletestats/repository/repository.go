// Package repository provides data access for the athlete-stats domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/athletestats/model"
	"go-espn-api/internal/domains/athletestats/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
)

// ListFilter holds the athlete-stats list query parameters.
type ListFilter struct {
	Sport         string
	League        string
	Season        string
	AthleteESPNID string
	Search        string
	Ordering      string
}

// AthleteStats defines read and ingest-write access for athlete season stats.
type AthleteStats interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.AthleteStatsRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.AthleteStatsRow, error)

	// Upsert update-or-creates by (league_id, athlete_espn_id, season_year, season_type).
	Upsert(ctx context.Context, m model.AthleteSeasonStats) (bool, error)
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.AthleteSeasonStats) (bool, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new athlete-stats repository.
func New(db *postgres.Connection, otl otel.Otel) AthleteStats {
	return &repositoryImpl{db: db, otel: otl}
}

const athleteStatsFrom = `
	FROM athlete_season_stats a
	JOIN leagues l ON l.id = a.league_id
	JOIN sports s ON s.id = l.sport_id`

const athleteStatsSelect = `
	SELECT a.id, a.athlete_espn_id, a.athlete_name, a.season_year, a.season_type, a.stats,
	       l.slug AS league_slug, s.slug AS sport_slug, a.created_at, a.updated_at` + athleteStatsFrom

var athleteStatsOrdering = map[string]string{
	"season_year":  "a.season_year",
	"athlete_name": "a.athlete_name",
}

func athleteStatsConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Sport != "" {
		args["sport"] = f.Sport

		conds = append(conds, "LOWER(s.slug) = LOWER(:sport)")
	}

	if f.League != "" {
		args["league"] = f.League

		conds = append(conds, "LOWER(l.slug) = LOWER(:league)")
	}

	if f.Season != "" && isDigits(f.Season) {
		args["season"] = f.Season

		conds = append(conds, "a.season_year = :season")
	}

	if f.AthleteESPNID != "" {
		args["athlete_espn_id"] = f.AthleteESPNID

		conds = append(conds, "a.athlete_espn_id = :athlete_espn_id")
	}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "a.athlete_name ILIKE :search")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.AthleteStatsRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".athlete_stats.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, athleteStatsOrdering, "a.season_year DESC")
	query := athleteStatsSelect + athleteStatsConditions(f, args) + " ORDER BY " + order + " LIMIT :limit OFFSET :offset"

	items := []dto.AthleteStatsRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".athlete_stats.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(a.id)" + athleteStatsFrom + athleteStatsConditions(f, args)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.AthleteStatsRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".athlete_stats.GetByID")
	defer scope.End()

	query := athleteStatsSelect + " WHERE a.id = :id"

	var row dto.AthleteStatsRow

	err := dbx.NamedGet(ctx, repo.db, query, map[string]any{"id": id}, &row)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return &row, nil
}

const athleteStatsUpsertSQL = `
	INSERT INTO athlete_season_stats (athlete_id, league_id, athlete_espn_id,
		athlete_name, season_year, season_type, stats, raw_data)
	VALUES (:athlete_id, :league_id, :athlete_espn_id,
		:athlete_name, :season_year, :season_type, :stats, :raw_data)
	ON CONFLICT (league_id, athlete_espn_id, season_year, season_type) DO UPDATE SET
		athlete_id = EXCLUDED.athlete_id,
		athlete_name = EXCLUDED.athlete_name,
		stats = EXCLUDED.stats,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING (xmax = 0) AS inserted`

// Upsert update-or-creates athlete season stats.
func (repo *repositoryImpl) Upsert(ctx context.Context, m model.AthleteSeasonStats) (bool, error) {
	return repo.upsert(ctx, repo.db.Write, m)
}

// UpsertTx is the transactional variant of Upsert.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.AthleteSeasonStats) (bool, error) {
	return repo.upsert(ctx, tx, m)
}

func (repo *repositoryImpl) upsert(ctx context.Context, p dbx.NamedPreparer, m model.AthleteSeasonStats) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".athletestats.Upsert")
	defer scope.End()

	args := map[string]any{
		"athlete_id":      m.AthleteID,
		"league_id":       m.LeagueID,
		"athlete_espn_id": m.AthleteESPNID,
		"athlete_name":    m.AthleteName,
		"season_year":     m.SeasonYear,
		"season_type":     m.SeasonType,
		"stats":           dbx.JSONOrDefault(m.Stats, "{}"),
		"raw_data":        dbx.JSONOrDefault(m.RawData, "{}"),
	}

	var inserted bool
	if err := dbx.NamedGetP(ctx, p, athleteStatsUpsertSQL, args, &inserted); err != nil {
		scope.TraceError(err)

		return false, err
	}

	return inserted, nil
}
