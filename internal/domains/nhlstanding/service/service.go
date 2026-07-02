// Package service contains the NHL standing domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlstanding/model/dto"
	"go-espn-api/internal/domains/nhlstanding/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// NHLStanding defines the NHL standing service operations.
type NHLStanding interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.StandingResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.StandingResponse, error)
}

type serviceImpl struct {
	repo repository.NHLStanding
	otel otel.Otel
}

// New creates a new NHL standing service.
func New(repo repository.NHLStanding, otl otel.Otel) NHLStanding {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.StandingResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_standing.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count nhl standings: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list nhl standings: %v", err))
	}

	teams, err := s.repo.TeamsByIDs(ctx, dto.TeamIDsOf(rows))
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl teams: %v", err))
	}

	res = make([]dto.StandingResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewStandingResponse(row, teams)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.StandingResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_standing.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get nhl standing: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	teams, err := s.repo.TeamsByIDs(ctx, dto.TeamIDsOf([]dto.StandingRow{*row}))
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl teams: %v", err))
	}

	out := dto.NewStandingResponse(*row, teams)

	return &out, nil
}
