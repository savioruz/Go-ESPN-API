// Package service contains the news domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/news/model/dto"
	"go-espn-api/internal/domains/news/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// News defines the news service operations.
type News interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.NewsListResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.NewsDetailResponse, error)
}

type serviceImpl struct {
	repo repository.News
	otel otel.Otel
}

// New creates a new news service.
func New(repo repository.News, otl otel.Otel) News {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.NewsListResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".news.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count news: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list news: %v", err))
	}

	res = make([]dto.NewsListResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewNewsListResponse(row)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.NewsDetailResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".news.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get news: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewNewsDetailResponse(*row)

	return &out, nil
}
