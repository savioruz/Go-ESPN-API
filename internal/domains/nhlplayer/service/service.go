// Package service contains the NHL player domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlplayer/model/dto"
	"go-espn-api/internal/domains/nhlplayer/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// NHLPlayer defines the NHL player service operations.
type NHLPlayer interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.PlayerResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.PlayerResponse, error)
}

type serviceImpl struct {
	repo repository.NHLPlayer
	otel otel.Otel
}

// New creates a new NHL player service.
func New(repo repository.NHLPlayer, otl otel.Otel) NHLPlayer {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.PlayerResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_player.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count nhl players: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list nhl players: %v", err))
	}

	teams, err := s.repo.TeamsByIDs(ctx, dto.TeamIDsOf(rows))
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl teams: %v", err))
	}

	res = make([]dto.PlayerResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewPlayerResponse(row, teams)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.PlayerResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_player.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get nhl player: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	teams, err := s.repo.TeamsByIDs(ctx, dto.TeamIDsOf([]dto.PlayerRow{*row}))
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl teams: %v", err))
	}

	out := dto.NewPlayerResponse(*row, teams)

	return &out, nil
}
