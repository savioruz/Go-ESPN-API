// Package repository provides data access for the NHL standing domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/nhlstanding/model"
	"go-espn-api/internal/domains/nhlstanding/model/dto"
	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ListFilter holds the NHL standing list query parameters.
type ListFilter struct {
	Date     string
	Ordering string
}

// NHLStanding defines read and ingest-write access for NHL standings.
type NHLStanding interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.StandingRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.StandingRow, error)
	TeamsByIDs(ctx context.Context, ids []int64) (map[int64]teamdto.TeamResponse, error)

	// UpsertTx update-or-creates a standing by (team_id, date); reports created.
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NHLStanding) (bool, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new NHL standing repository.
func New(db *postgres.Connection, otl otel.Otel) NHLStanding {
	return &repositoryImpl{db: db, otel: otl}
}

const standingSelect = "SELECT " + dto.StandingColumns + " FROM nhl_standings"

var standingOrdering = map[string]string{
	"date":      "date",
	"points":    "points",
	"point_pct": "point_pct",
}

func standingConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Date != "" {
		args["date"] = f.Date

		conds = append(conds, "date = :date")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.StandingRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_standing.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, standingOrdering, "date DESC, points DESC")
	query := standingSelect + standingConditions(f, args) + " ORDER BY " + order + ", id LIMIT :limit OFFSET :offset"

	items := []dto.StandingRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_standing.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(id) FROM nhl_standings" + standingConditions(f, args)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.StandingRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_standing.GetByID")
	defer scope.End()

	var row dto.StandingRow

	err := dbx.NamedGet(ctx, repo.db, standingSelect+" WHERE id = :id", map[string]any{"id": id}, &row)
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

// TeamsByIDs batch-loads teams for the given ids, returning a lookup keyed by id.
func (repo *repositoryImpl) TeamsByIDs(ctx context.Context, ids []int64) (map[int64]teamdto.TeamResponse, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_standing.TeamsByIDs")
	defer scope.End()

	if len(ids) == 0 {
		return map[int64]teamdto.TeamResponse{}, nil
	}

	rows := []teamdto.TeamRow{}

	query := "SELECT " + teamdto.TeamColumns + " FROM nhl_teams WHERE id = ANY(:ids)"
	if err := dbx.NamedSelect(ctx, repo.db, query, map[string]any{"ids": pq.Int64Array(ids)}, &rows); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return teamdto.TeamMap(rows), nil
}

const standingUpsertSQL = `
	INSERT INTO nhl_standings (team_id, date, games_played, wins, losses,
		ot_losses, points, point_pct, division_name, conference_name, raw_data)
	VALUES (:team_id, :date, :games_played, :wins, :losses,
		:ot_losses, :points, :point_pct, :division_name, :conference_name, :raw_data)
	ON CONFLICT (team_id, date) DO UPDATE SET
		games_played = EXCLUDED.games_played,
		wins = EXCLUDED.wins,
		losses = EXCLUDED.losses,
		ot_losses = EXCLUDED.ot_losses,
		points = EXCLUDED.points,
		point_pct = EXCLUDED.point_pct,
		division_name = EXCLUDED.division_name,
		conference_name = EXCLUDED.conference_name,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING (xmax = 0) AS inserted`

// UpsertTx update-or-creates a standing by (team_id, date), reporting created.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NHLStanding) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_standing.Upsert")
	defer scope.End()

	args := map[string]any{
		"team_id":         m.TeamID,
		"date":            m.Date,
		"games_played":    m.GamesPlayed,
		"wins":            m.Wins,
		"losses":          m.Losses,
		"ot_losses":       m.OTLosses,
		"points":          m.Points,
		"point_pct":       m.PointPct,
		"division_name":   m.DivisionName,
		"conference_name": m.ConferenceName,
		"raw_data":        dbx.JSONOrDefault(m.RawData, "{}"),
	}

	var inserted bool
	if err := dbx.NamedGetP(ctx, tx, standingUpsertSQL, args, &inserted); err != nil {
		scope.TraceError(err)

		return false, err
	}

	return inserted, nil
}
