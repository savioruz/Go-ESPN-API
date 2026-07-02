// Package service contains the sport domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/sport/model/dto"
	"go-espn-api/internal/domains/sport/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// Sport defines the sport service operations.
type Sport interface {
	List(ctx context.Context, page, pageSize int) ([]dto.SportResponse, int, error)
	GetBySlug(ctx context.Context, slug string) (*dto.SportResponse, error)
}

type serviceImpl struct {
	repo repository.Sport
	otel otel.Otel
}

// New creates a new sport service.
func New(repo repository.Sport, otl otel.Otel) Sport {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, page, pageSize int) (res []dto.SportResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".sport.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count sports: %v", err))
	}

	items, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list sports: %v", err))
	}

	res = make([]dto.SportResponse, len(items))
	for i, m := range items {
		res[i] = dto.NewSportResponse(m)
	}

	return res, total, nil
}

func (s *serviceImpl) GetBySlug(ctx context.Context, slug string) (res *dto.SportResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".sport.GetBySlug")
	defer scope.End()
	defer scope.TraceIfError(err)

	m, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get sport: %v", err))
	}

	if m == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewSportResponse(*m)

	return &out, nil
}
