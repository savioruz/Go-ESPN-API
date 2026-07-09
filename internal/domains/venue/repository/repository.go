// Package repository provides ingest-write access for the venue domain.
package repository

import (
	"context"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/venue/model"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"

	"github.com/jmoiron/sqlx"
)

// Venue defines ingest-write access for venues.
type Venue interface {
	Upsert(ctx context.Context, m model.Venue) (int64, error)
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.Venue) (int64, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new venue repository.
func New(db *postgres.Connection, otl otel.Otel) Venue {
	return &repositoryImpl{db: db, otel: otl}
}

// venueUpsertSQL updates-or-creates a venue keyed by espn_id (full field update).
const venueUpsertSQL = `
	INSERT INTO venues (espn_id, name, city, state, country, is_indoor, capacity, raw_data)
	VALUES (:espn_id, :name, :city, :state, :country, :is_indoor, :capacity, :raw_data)
	ON CONFLICT (espn_id) DO UPDATE SET
		name = EXCLUDED.name,
		city = EXCLUDED.city,
		state = EXCLUDED.state,
		country = EXCLUDED.country,
		is_indoor = EXCLUDED.is_indoor,
		capacity = EXCLUDED.capacity,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING id`

// Upsert update-or-creates a venue by espn_id and returns its id.
func (repo *repositoryImpl) Upsert(ctx context.Context, m model.Venue) (int64, error) {
	return repo.upsert(ctx, repo.db.Write, m)
}

// UpsertTx is the transactional variant of Upsert.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.Venue) (int64, error) {
	return repo.upsert(ctx, tx, m)
}

func (repo *repositoryImpl) upsert(ctx context.Context, p dbx.NamedPreparer, m model.Venue) (int64, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".venue.Upsert")
	defer scope.End()

	args := map[string]any{
		"espn_id":   m.ESPNID,
		"name":      m.Name,
		"city":      m.City,
		"state":     m.State,
		"country":   m.Country,
		"is_indoor": m.IsIndoor,
		"capacity":  m.Capacity,
		"raw_data":  dbx.JSONOrDefault(m.RawData, "{}"),
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, venueUpsertSQL)

	var id int64
	if err := dbx.NamedGetP(ctx, p, venueUpsertSQL, args, &id); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return id, nil
}
