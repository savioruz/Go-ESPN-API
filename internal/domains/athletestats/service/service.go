// Package service contains the athlete-stats domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/athletestats/model/dto"
	"go-espn-api/internal/domains/athletestats/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// AthleteStats defines the athlete-stats service operations.
type AthleteStats interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.AthleteStatsResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.AthleteStatsResponse, error)
}

type serviceImpl struct {
	repo repository.AthleteStats
	otel otel.Otel
}

// New creates a new athlete-stats service.
func New(repo repository.AthleteStats, otl otel.Otel) AthleteStats {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.AthleteStatsResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".athlete_stats.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count athlete stats: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list athlete stats: %v", err))
	}

	res = make([]dto.AthleteStatsResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewAthleteStatsResponse(row)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.AthleteStatsResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".athlete_stats.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get athlete stats: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewAthleteStatsResponse(*row)

	return &out, nil
}
