// Package repository provides ingest-write access for the competitor domain.
package repository

import (
	"context"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/competitor/model"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"

	"github.com/jmoiron/sqlx"
)

// Competitor defines ingest-write access for competitors.
type Competitor interface {
	DeleteByEvent(ctx context.Context, eventID int64) error
	DeleteByEventTx(ctx context.Context, tx *sqlx.Tx, eventID int64) error
	Insert(ctx context.Context, m model.Competitor) error
	InsertTx(ctx context.Context, tx *sqlx.Tx, m model.Competitor) error
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new competitor repository.
func New(db *postgres.Connection, otl otel.Otel) Competitor {
	return &repositoryImpl{db: db, otel: otl}
}

const competitorDeleteSQL = `DELETE FROM competitors WHERE event_id = :event_id`

const competitorInsertSQL = `
	INSERT INTO competitors (event_id, team_id, home_away, score, winner,
		line_scores, records, statistics, leaders, "order", raw_data)
	VALUES (:event_id, :team_id, :home_away, :score, :winner,
		:line_scores, :records, :statistics, :leaders, :order, :raw_data)`

// DeleteByEvent removes all competitors attached to an event.
func (repo *repositoryImpl) DeleteByEvent(ctx context.Context, eventID int64) error {
	return repo.deleteByEvent(ctx, repo.db.Write, eventID)
}

// DeleteByEventTx is the transactional variant of DeleteByEvent.
func (repo *repositoryImpl) DeleteByEventTx(ctx context.Context, tx *sqlx.Tx, eventID int64) error {
	return repo.deleteByEvent(ctx, tx, eventID)
}

func (repo *repositoryImpl) deleteByEvent(ctx context.Context, p dbx.NamedPreparer, eventID int64) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".competitor.DeleteByEvent")
	defer scope.End()

	if err := dbx.NamedExecP(ctx, p, competitorDeleteSQL, map[string]any{"event_id": eventID}); err != nil {
		scope.TraceError(err)

		return err
	}

	return nil
}

// Insert creates a single competitor row.
func (repo *repositoryImpl) Insert(ctx context.Context, m model.Competitor) error {
	return repo.insert(ctx, repo.db.Write, m)
}

// InsertTx is the transactional variant of Insert.
func (repo *repositoryImpl) InsertTx(ctx context.Context, tx *sqlx.Tx, m model.Competitor) error {
	return repo.insert(ctx, tx, m)
}

func (repo *repositoryImpl) insert(ctx context.Context, p dbx.NamedPreparer, m model.Competitor) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".competitor.Insert")
	defer scope.End()

	args := map[string]any{
		"event_id":    m.EventID,
		"team_id":     m.TeamID,
		"home_away":   m.HomeAway,
		"score":       m.Score,
		"winner":      m.Winner,
		"line_scores": dbx.JSONOrDefault(m.LineScores, "[]"),
		"records":     dbx.JSONOrDefault(m.Records, "[]"),
		"statistics":  dbx.JSONOrDefault(m.Statistics, "[]"),
		"leaders":     dbx.JSONOrDefault(m.Leaders, "[]"),
		"order":       m.Order,
		"raw_data":    dbx.JSONOrDefault(m.RawData, "{}"),
	}

	if err := dbx.NamedExecP(ctx, p, competitorInsertSQL, args); err != nil {
		scope.TraceError(err)

		return err
	}

	return nil
}
