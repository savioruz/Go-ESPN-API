// Package service contains the event domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/event/model/dto"
	"go-espn-api/internal/domains/event/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// Event defines the event service operations.
type Event interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.EventListResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.EventDetailResponse, error)
	GetByESPNID(ctx context.Context, espnID string) (*dto.EventDetailResponse, error)
}

type serviceImpl struct {
	repo repository.Event
	otel otel.Otel
}

// New creates a new event service.
func New(repo repository.Event, otl otel.Otel) Event {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.EventListResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".event.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count events: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list events: %v", err))
	}

	// N+1 avoidance: fetch all competitors for the page in one query, then
	// group them in Go by event id.
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}

	compRows, err := s.repo.CompetitorsByEventIDs(ctx, ids)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list competitors: %v", err))
	}

	grouped := dto.GroupCompetitors(compRows)

	res = make([]dto.EventListResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewEventListResponse(row, grouped[row.ID])
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.EventDetailResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".event.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get event: %v", err))
	}

	return s.buildDetail(ctx, row)
}

func (s *serviceImpl) GetByESPNID(ctx context.Context, espnID string) (res *dto.EventDetailResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".event.GetByESPNID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByESPNID(ctx, espnID)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get event: %v", err))
	}

	return s.buildDetail(ctx, row)
}

func (s *serviceImpl) buildDetail(ctx context.Context, row *dto.EventDetailRow) (*dto.EventDetailResponse, error) {
	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	compRows, err := s.repo.CompetitorsByEventIDs(ctx, []int64{row.ID})
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list competitors: %v", err))
	}

	out := dto.NewEventDetailResponse(*row, dto.GroupCompetitors(compRows)[row.ID])

	return &out, nil
}
