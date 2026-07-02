// Package service contains the injury domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/injury/model/dto"
	"go-espn-api/internal/domains/injury/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// Injury defines the injury service operations.
type Injury interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.InjuryResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.InjuryResponse, error)
}

type serviceImpl struct {
	repo repository.Injury
	otel otel.Otel
}

// New creates a new injury service.
func New(repo repository.Injury, otl otel.Otel) Injury {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.InjuryResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".injury.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count injuries: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list injuries: %v", err))
	}

	res = make([]dto.InjuryResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewInjuryResponse(row)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.InjuryResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".injury.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get injury: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewInjuryResponse(*row)

	return &out, nil
}
