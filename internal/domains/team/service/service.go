// Package service contains the team domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/team/model/dto"
	"go-espn-api/internal/domains/team/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// Team defines the team service operations.
type Team interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.TeamListResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.TeamDetailResponse, error)
	GetByESPNID(ctx context.Context, espnID string) (*dto.TeamDetailResponse, error)
}

type serviceImpl struct {
	repo repository.Team
	otel otel.Otel
}

// New creates a new team service.
func New(repo repository.Team, otl otel.Otel) Team {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.TeamListResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".team.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count teams: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list teams: %v", err))
	}

	res = make([]dto.TeamListResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewTeamListResponse(row)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.TeamDetailResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".team.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get team: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewTeamDetailResponse(*row)

	return &out, nil
}

func (s *serviceImpl) GetByESPNID(ctx context.Context, espnID string) (res *dto.TeamDetailResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".team.GetByESPNID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByESPNID(ctx, espnID)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get team: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewTeamDetailResponse(*row)

	return &out, nil
}
