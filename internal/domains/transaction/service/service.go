// Package service contains the transaction domain business logic.
package service

import (
	"context"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/transaction/model/dto"
	"go-espn-api/internal/domains/transaction/repository"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
)

// Transaction defines the transaction service operations.
type Transaction interface {
	List(ctx context.Context, f repository.ListFilter, page, pageSize int) ([]dto.TransactionResponse, int, error)
	GetByID(ctx context.Context, id int64) (*dto.TransactionResponse, error)
}

type serviceImpl struct {
	repo repository.Transaction
	otel otel.Otel
}

// New creates a new transaction service.
func New(repo repository.Transaction, otl otel.Otel) Transaction {
	return &serviceImpl{repo: repo, otel: otl}
}

func (s *serviceImpl) List(ctx context.Context, f repository.ListFilter, page, pageSize int) (res []dto.TransactionResponse, total int, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".transaction.List")
	defer scope.End()
	defer scope.TraceIfError(err)

	total, err = s.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to count transactions: %v", err))
	}

	rows, err := s.repo.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to list transactions: %v", err))
	}

	res = make([]dto.TransactionResponse, len(rows))
	for i, row := range rows {
		res[i] = dto.NewTransactionResponse(row)
	}

	return res, total, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id int64) (res *dto.TransactionResponse, err error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".transaction.GetByID")
	defer scope.End()
	defer scope.TraceIfError(err)

	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, fmt.Sprintf("failed to get transaction: %v", err))
	}

	if row == nil {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	out := dto.NewTransactionResponse(*row)

	return &out, nil
}
