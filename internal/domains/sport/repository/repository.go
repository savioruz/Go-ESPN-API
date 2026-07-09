// Package repository provides data access for the sport domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/sport/model"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"

	"github.com/jmoiron/sqlx"
)

// Sport defines read and ingest-write access for sports.
type Sport interface {
	List(ctx context.Context, page, pageSize int) ([]model.Sport, error)
	Count(ctx context.Context) (int, error)
	GetBySlug(ctx context.Context, slug string) (*model.Sport, error)
	GetOrCreate(ctx context.Context, slug, name string) (int64, error)
	GetOrCreateTx(ctx context.Context, tx *sqlx.Tx, slug, name string) (int64, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new sport repository.
func New(db *postgres.Connection, otl otel.Otel) Sport {
	return &repositoryImpl{db: db, otel: otl}
}

const sportColumns = `id, slug, name, created_at, updated_at`

func (repo *repositoryImpl) List(ctx context.Context, page, pageSize int) ([]model.Sport, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".sport.List")
	defer scope.End()

	query := fmt.Sprintf(
		"SELECT %s FROM sports ORDER BY name ASC LIMIT :limit OFFSET :offset",
		sportColumns,
	)
	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}

	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []model.Sport{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".sport.Count")
	defer scope.End()

	const query = "SELECT COUNT(id) FROM sports"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, map[string]any{}, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

// getOrCreateSQL upserts a sport by slug. The no-op DO UPDATE keeps the existing
// name (Django get_or_create does not clobber it) while still RETURNING the id.
const getOrCreateSQL = `
	INSERT INTO sports (slug, name) VALUES (:slug, :name)
	ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
	RETURNING id`

// GetOrCreate gets or creates a sport by slug and returns its id.
func (repo *repositoryImpl) GetOrCreate(ctx context.Context, slug, name string) (int64, error) {
	return repo.getOrCreate(ctx, repo.db.Write, slug, name)
}

// GetOrCreateTx is the transactional variant of GetOrCreate.
func (repo *repositoryImpl) GetOrCreateTx(ctx context.Context, tx *sqlx.Tx, slug, name string) (int64, error) {
	return repo.getOrCreate(ctx, tx, slug, name)
}

func (repo *repositoryImpl) getOrCreate(ctx context.Context, p dbx.NamedPreparer, slug, name string) (int64, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".sport.GetOrCreate")
	defer scope.End()

	scope.SetAttribute(constant.OtelQueryAttributeKey, getOrCreateSQL)

	var id int64
	if err := dbx.NamedGetP(ctx, p, getOrCreateSQL, map[string]any{"slug": slug, "name": name}, &id); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return id, nil
}

func (repo *repositoryImpl) GetBySlug(ctx context.Context, slug string) (*model.Sport, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".sport.GetBySlug")
	defer scope.End()

	query := fmt.Sprintf("SELECT %s FROM sports WHERE slug = :slug", sportColumns)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var m model.Sport

	err := dbx.NamedGet(ctx, repo.db, query, map[string]any{"slug": slug}, &m)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return &m, nil
}
