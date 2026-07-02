// Package service contains the NHL game domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlgame/model/dto"
	"go-espn-api/internal/domains/nhlgame/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// NHLGame defines the NHL game service operations.
type NHLGame interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.GameResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.GameResponse, error)
}

type serviceImpl struct {
	repo repository.NHLGame
	otel otel.Otel
}

// New creates a new NHL game service.
func New(repo repository.NHLGame, otl otel.Otel) NHLGame {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.GameResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_game.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count nhl games: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list nhl games: %v", err))
	}

	teams, err := s.repo.TeamsByIDs(ctx, dto.TeamIDsOf(rows))
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl teams: %v", err))
	}

	res = make([]dto.GameResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewGameResponse(row, teams)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.GameResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".nhl_game.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get nhl game: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	teams, err := s.repo.TeamsByIDs(ctx, dto.TeamIDsOf([]dto.GameRow{*row}))
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to hydrate nhl teams: %v", err))
	}

	out := dto.NewGameResponse(*row, teams)

	return &out, nil
}
