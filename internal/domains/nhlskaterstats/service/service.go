// Package service contains the NHL skater-stats domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlskaterstats/model/dto"
	"go-espn-api/internal/domains/nhlskaterstats/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// NHLSkaterStats defines the NHL skater-stats service operations.
type NHLSkaterStats interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.SkaterStatsResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.SkaterStatsResponse, error)
}

type serviceImpl struct {
	repo repository.NHLSkaterStats
	otel otel.Otel
}

// New creates a new NHL skater-stats service.
func New(repo repository.NHLSkaterStats, otl otel.Otel) NHLSkaterStats {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.SkaterStatsResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_skater_stats.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count nhl skater stats: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list nhl skater stats: %v", err))
	}

	players, err := s.repo.PlayersByIDs(ctx, dto.PlayerIDsOf(rows))
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl players: %v", err))
	}

	res = make([]dto.SkaterStatsResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewSkaterStatsResponse(row, players)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.SkaterStatsResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_skater_stats.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get nhl skater stats: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	players, err := s.repo.PlayersByIDs(ctx, dto.PlayerIDsOf([]dto.SkaterStatsRow{*row}))
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl players: %v", err))
	}

	out := dto.NewSkaterStatsResponse(*row, players)

	return &out, nil
}
