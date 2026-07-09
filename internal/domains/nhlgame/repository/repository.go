// Package repository provides data access for the NHL game domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/nhlgame/model/dto"
	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/lib/pq"
)

// ListFilter holds the NHL game list query parameters (all exact-match).
type ListFilter struct {
	Season   string
	GameType *int
	Status   string
	Ordering string
}

// NHLGame defines read access for NHL games.
type NHLGame interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.GameRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.GameRow, error)
	TeamsByIDs(ctx context.Context, ids []int64) (map[int64]teamdto.TeamResponse, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new NHL game repository.
func New(db *postgres.Connection, otl otel.Otel) NHLGame {
	return &repositoryImpl{db: db, otel: otl}
}

const gameSelect = "SELECT " + dto.GameColumns + " FROM nhl_games"

var gameOrdering = map[string]string{
	"date": "date",
}

func gameConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Season != "" {
		args["season"] = f.Season

		conds = append(conds, "season = :season")
	}

	if f.GameType != nil {
		args["game_type"] = *f.GameType

		conds = append(conds, "game_type = :game_type")
	}

	if f.Status != "" {
		args["status"] = f.Status

		conds = append(conds, "status = :status")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.GameRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_game.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, gameOrdering, "date DESC")
	query := gameSelect + gameConditions(f, args) + " ORDER BY " + order + ", id LIMIT :limit OFFSET :offset"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []dto.GameRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_game.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(id) FROM nhl_games" + gameConditions(f, args)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.GameRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_game.GetByID")
	defer scope.End()

	query := gameSelect + " WHERE id = :id"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var row dto.GameRow

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

// TeamsByIDs batch-loads teams for the given ids, returning a lookup keyed by id.
func (repo *repositoryImpl) TeamsByIDs(ctx context.Context, ids []int64) (map[int64]teamdto.TeamResponse, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_game.TeamsByIDs")
	defer scope.End()

	if len(ids) == 0 {
		return map[int64]teamdto.TeamResponse{}, nil
	}

	rows := []teamdto.TeamRow{}

	query := "SELECT " + teamdto.TeamColumns + " FROM nhl_teams WHERE id = ANY(:ids)"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	if err := dbx.NamedSelect(ctx, repo.db, query, map[string]any{"ids": pq.Int64Array(ids)}, &rows); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return teamdto.TeamMap(rows), nil
}
