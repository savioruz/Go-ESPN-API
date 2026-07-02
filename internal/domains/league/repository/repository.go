// Package repository provides data access for the league domain.
package repository

import (
	"context"
	"database/sql"
	"errors"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/league/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"

	"github.com/jmoiron/sqlx"
)

// League defines read and ingest-write access for leagues.
type League interface {
	List(ctx context.Context, sport string, page, pageSize int) ([]dto.LeagueRow, error)
	Count(ctx context.Context, sport string) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.LeagueRow, error)
	GetOrCreate(ctx context.Context, sportID int64, slug, name, abbreviation string) (int64, error)
	GetOrCreateTx(ctx context.Context, tx *sqlx.Tx, sportID int64, slug, name, abbreviation string) (int64, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new league repository.
func New(db *postgres.Connection, otl otel.Otel) League {
	return &repositoryImpl{db: db, otel: otl}
}

const leagueSelect = `
	SELECT l.id, l.slug, l.name, l.abbreviation, l.created_at, l.updated_at,
	       s.id AS sport_id, s.slug AS sport_slug, s.name AS sport_name,
	       s.created_at AS sport_created_at, s.updated_at AS sport_updated_at
	FROM leagues l
	JOIN sports s ON s.id = l.sport_id`

func leagueWhere(sport string, args map[string]any) string {
	if sport == "" {
		return ""
	}

	args["sport"] = sport

	return " WHERE LOWER(s.slug) = LOWER(:sport)"
}

func (repo *repositoryImpl) List(ctx context.Context, sport string, page, pageSize int) ([]dto.LeagueRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".league.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	query := leagueSelect + leagueWhere(sport, args) +
		" ORDER BY s.name ASC, l.name ASC LIMIT :limit OFFSET :offset"

	items := []dto.LeagueRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, sport string) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".league.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(l.id) FROM leagues l JOIN sports s ON s.id = l.sport_id" + leagueWhere(sport, args)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

// leagueGetOrCreateSQL upserts a league by (sport_id, slug). The no-op DO UPDATE
// keeps the existing name/abbreviation while still RETURNING the id.
const leagueGetOrCreateSQL = `
	INSERT INTO leagues (sport_id, slug, name, abbreviation)
	VALUES (:sport_id, :slug, :name, :abbreviation)
	ON CONFLICT (sport_id, slug) DO UPDATE SET sport_id = EXCLUDED.sport_id
	RETURNING id`

// GetOrCreate gets or creates a league by (sport_id, slug) and returns its id.
func (repo *repositoryImpl) GetOrCreate(ctx context.Context, sportID int64, slug, name, abbreviation string) (int64, error) {
	return repo.getOrCreate(ctx, repo.db.Write, sportID, slug, name, abbreviation)
}

// GetOrCreateTx is the transactional variant of GetOrCreate.
func (repo *repositoryImpl) GetOrCreateTx(ctx context.Context, tx *sqlx.Tx, sportID int64, slug, name, abbreviation string) (int64, error) {
	return repo.getOrCreate(ctx, tx, sportID, slug, name, abbreviation)
}

func (repo *repositoryImpl) getOrCreate(ctx context.Context, p dbx.NamedPreparer, sportID int64, slug, name, abbreviation string) (int64, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".league.GetOrCreate")
	defer scope.End()

	args := map[string]any{"sport_id": sportID, "slug": slug, "name": name, "abbreviation": abbreviation}

	var id int64
	if err := dbx.NamedGetP(ctx, p, leagueGetOrCreateSQL, args, &id); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return id, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.LeagueRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".league.GetByID")
	defer scope.End()

	query := leagueSelect + " WHERE l.id = :id"

	var row dto.LeagueRow

	err := dbx.NamedGet(ctx, repo.db, query, map[string]any{"id": id}, &row)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return &row, nil
}
