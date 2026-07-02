// Package service contains the league domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/league/model/dto"
	"go-espn-api/internal/domains/league/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// League defines the league service operations.
type League interface {
	List(ctx context.Context, sport string, page, pageSize int) ([]dto.LeagueResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.LeagueResponse, error)
}

type serviceImpl struct {
	repo repository.League
	otel otel.Otel
}

// New creates a new league service.
func New(repo repository.League, otl otel.Otel) League {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, sport string, page, pageSize int) (res []dto.LeagueResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".league.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, sport)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count leagues: %v", err))
	}

	rows, err := s.repo.List(ctx, sport, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list leagues: %v", err))
	}

	res = make([]dto.LeagueResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewLeagueResponse(row)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.LeagueResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".league.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get league: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewLeagueResponse(*row)

	return &out, nil
}
