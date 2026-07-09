// Package repository provides data access for the NHL goalie-stats domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/nhlgoaliestats/model/dto"
	playerdto "go-espn-api/internal/domains/nhlplayer/model/dto"
	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/lib/pq"
)

// ListFilter holds the NHL goalie-stats list query parameters.
type ListFilter struct {
	Season   string
	Ordering string
}

// NHLGoalieStats defines read access for NHL goalie season stats.
type NHLGoalieStats interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.GoalieStatsRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.GoalieStatsRow, error)
	PlayersByIDs(ctx context.Context, ids []int64) (map[int64]playerdto.PlayerResponse, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new NHL goalie-stats repository.
func New(db *postgres.Connection, otl otel.Otel) NHLGoalieStats {
	return &repositoryImpl{db: db, otel: otl}
}

const goalieSelect = "SELECT " + dto.GoalieStatsColumns + " FROM nhl_goalie_season_stats"

var goalieOrdering = map[string]string{
	"wins":     "wins",
	"save_pct": "save_pct",
	"gaa":      "gaa",
	"shutouts": "shutouts",
}

func goalieConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Season != "" {
		args["season"] = f.Season

		conds = append(conds, "season = :season")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.GoalieStatsRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_goalie_stats.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, goalieOrdering, "season DESC, wins DESC")
	query := goalieSelect + goalieConditions(f, args) + " ORDER BY " + order + ", id LIMIT :limit OFFSET :offset"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []dto.GoalieStatsRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_goalie_stats.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(id) FROM nhl_goalie_season_stats" + goalieConditions(f, args)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.GoalieStatsRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_goalie_stats.GetByID")
	defer scope.End()

	query := goalieSelect + " WHERE id = :id"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var row dto.GoalieStatsRow

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

// PlayersByIDs batch-loads players (and their current teams) for the given ids,
// returning fully-hydrated player responses keyed by id (3-level nesting).
func (repo *repositoryImpl) PlayersByIDs(ctx context.Context, ids []int64) (map[int64]playerdto.PlayerResponse, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_goalie_stats.PlayersByIDs")
	defer scope.End()

	if len(ids) == 0 {
		return map[int64]playerdto.PlayerResponse{}, nil
	}

	players := []playerdto.PlayerRow{}

	pQuery := "SELECT " + playerdto.PlayerColumns + " FROM nhl_players WHERE id = ANY(:ids)"
	scope.SetAttribute(constant.OtelQueryAttributeKey, pQuery)

	if err := dbx.NamedSelect(ctx, repo.db, pQuery, map[string]any{"ids": pq.Int64Array(ids)}, &players); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	teamIDs := playerdto.TeamIDsOf(players)

	teamMap := map[int64]teamdto.TeamResponse{}

	if len(teamIDs) > 0 {
		teams := []teamdto.TeamRow{}

		tQuery := "SELECT " + teamdto.TeamColumns + " FROM nhl_teams WHERE id = ANY(:ids)"
		if err := dbx.NamedSelect(ctx, repo.db, tQuery, map[string]any{"ids": pq.Int64Array(teamIDs)}, &teams); err != nil {
			scope.TraceError(err)

			return nil, err
		}

		teamMap = teamdto.TeamMap(teams)
	}

	return playerdto.PlayerMap(players, teamMap), nil
}
