// Package repository provides lookup access for the athlete domain used by the
// athlete-stats ingest service.
package repository

import (
	"context"
	"database/sql"
	"errors"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"

	"github.com/jmoiron/sqlx"
)

// Ref is the minimal athlete identity needed to link season stats.
type Ref struct {
	ID          int64  `db:"id"`
	DisplayName string `db:"display_name"`
}

// Athlete defines lookup access for athletes.
type Athlete interface {
	GetRefByESPNID(ctx context.Context, espnID string) (*Ref, error)
	GetRefByESPNIDTx(ctx context.Context, tx *sqlx.Tx, espnID string) (*Ref, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new athlete repository.
func New(db *postgres.Connection, otl otel.Otel) Athlete {
	return &repositoryImpl{db: db, otel: otl}
}

const athleteRefSQL = `SELECT id, display_name FROM athletes WHERE espn_id = :espn_id LIMIT 1`

// GetRefByESPNID returns the athlete ref for an ESPN id, or nil when not found.
func (repo *repositoryImpl) GetRefByESPNID(ctx context.Context, espnID string) (*Ref, error) {
	return repo.getRef(ctx, repo.db.Read, espnID)
}

// GetRefByESPNIDTx is the transactional variant of GetRefByESPNID.
func (repo *repositoryImpl) GetRefByESPNIDTx(ctx context.Context, tx *sqlx.Tx, espnID string) (*Ref, error) {
	return repo.getRef(ctx, tx, espnID)
}

func (repo *repositoryImpl) getRef(ctx context.Context, p dbx.NamedPreparer, espnID string) (*Ref, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".athlete.GetRefByESPNID")
	defer scope.End()

	scope.SetAttribute(constant.OtelQueryAttributeKey, athleteRefSQL)

	var ref Ref

	err := dbx.NamedGetP(ctx, p, athleteRefSQL, map[string]any{"espn_id": espnID}, &ref)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return &ref, nil
}
