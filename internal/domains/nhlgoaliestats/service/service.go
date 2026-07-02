// Package service contains the NHL goalie-stats domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlgoaliestats/model/dto"
	"go-espn-api/internal/domains/nhlgoaliestats/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// NHLGoalieStats defines the NHL goalie-stats service operations.
type NHLGoalieStats interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.GoalieStatsResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.GoalieStatsResponse, error)
}

type serviceImpl struct {
	repo repository.NHLGoalieStats
	otel otel.Otel
}

// New creates a new NHL goalie-stats service.
func New(repo repository.NHLGoalieStats, otl otel.Otel) NHLGoalieStats {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.GoalieStatsResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_goalie_stats.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count nhl goalie stats: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list nhl goalie stats: %v", err))
	}

	players, err := s.repo.PlayersByIDs(ctx, dto.PlayerIDsOf(rows))
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl players: %v", err))
	}

	res = make([]dto.GoalieStatsResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewGoalieStatsResponse(row, players)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.GoalieStatsResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_goalie_stats.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get nhl goalie stats: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	players, err := s.repo.PlayersByIDs(ctx, dto.PlayerIDsOf([]dto.GoalieStatsRow{*row}))
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl players: %v", err))
	}

	out := dto.NewGoalieStatsResponse(*row, players)

	return &out, nil
}
