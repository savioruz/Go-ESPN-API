// Package repository provides data access for the transaction domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/transaction/model"
	"go-espn-api/internal/domains/transaction/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
)

// ListFilter holds the transaction list query parameters.
type ListFilter struct {
	Sport    string
	League   string
	DateFrom string
	Search   string
	Ordering string
}

// Transaction defines read and ingest-write access for transactions.
type Transaction interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.TransactionRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.TransactionRow, error)

	// UpsertByESPNTx update-or-creates by (league_id, espn_id); reports created.
	// The transactions table has no unique constraint on espn_id, so this does a
	// manual get-then-update/insert (matching Django update_or_create).
	UpsertByESPNTx(ctx context.Context, tx *sqlx.Tx, m model.Transaction) (bool, error)
	// InsertTx creates a transaction row (used when espn_id is absent).
	InsertTx(ctx context.Context, tx *sqlx.Tx, m model.Transaction) error
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new transaction repository.
func New(db *postgres.Connection, otl otel.Otel) Transaction {
	return &repositoryImpl{db: db, otel: otl}
}

const transactionFrom = `
	FROM transactions tr
	JOIN leagues l ON l.id = tr.league_id
	JOIN sports s ON s.id = l.sport_id
	LEFT JOIN teams t ON t.id = tr.team_id`

const transactionSelect = `
	SELECT tr.id, tr.espn_id, tr.date, tr.description, tr.type, tr.athlete_name, tr.athlete_espn_id,
	       l.slug AS league_slug, s.slug AS sport_slug, t.abbreviation AS team_abbreviation,
	       tr.created_at, tr.updated_at` + transactionFrom

var transactionOrdering = map[string]string{
	"date":       "tr.date",
	"created_at": "tr.created_at",
}

func transactionConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Sport != "" {
		args["sport"] = f.Sport

		conds = append(conds, "LOWER(s.slug) = LOWER(:sport)")
	}

	if f.League != "" {
		args["league"] = f.League

		conds = append(conds, "LOWER(l.slug) = LOWER(:league)")
	}

	if f.DateFrom != "" {
		args["date_from"] = f.DateFrom

		conds = append(conds, "tr.date >= :date_from")
	}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(tr.description ILIKE :search OR tr.athlete_name ILIKE :search OR tr.type ILIKE :search)")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.TransactionRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".transaction.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, transactionOrdering, "tr.date DESC")
	query := transactionSelect + transactionConditions(f, args) + " ORDER BY " + order + " LIMIT :limit OFFSET :offset"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []dto.TransactionRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".transaction.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(tr.id)" + transactionFrom + transactionConditions(f, args)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.TransactionRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".transaction.GetByID")
	defer scope.End()

	query := transactionSelect + " WHERE tr.id = :id"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var row dto.TransactionRow

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

const transactionInsertSQL = `
	INSERT INTO transactions (league_id, team_id, espn_id, date, description, type,
		athlete_name, athlete_espn_id, raw_data)
	VALUES (:league_id, :team_id, :espn_id, :date, :description, :type,
		:athlete_name, :athlete_espn_id, :raw_data)`

const transactionFindSQL = `SELECT id FROM transactions WHERE league_id = :league_id AND espn_id = :espn_id LIMIT 1`

const transactionUpdateSQL = `
	UPDATE transactions SET
		team_id = :team_id,
		date = :date,
		description = :description,
		type = :type,
		athlete_name = :athlete_name,
		athlete_espn_id = :athlete_espn_id,
		raw_data = :raw_data,
		updated_at = NOW()
	WHERE id = :id`

func transactionArgs(m model.Transaction) map[string]any {
	return map[string]any{
		"league_id":       m.LeagueID,
		"team_id":         m.TeamID,
		"espn_id":         m.ESPNID,
		"date":            m.Date,
		"description":     m.Description,
		"type":            m.Type,
		"athlete_name":    m.AthleteName,
		"athlete_espn_id": m.AthleteESPNID,
		"raw_data":        dbx.JSONOrDefault(m.RawData, "{}"),
	}
}

// UpsertByESPNTx update-or-creates a transaction keyed by (league_id, espn_id).
func (repo *repositoryImpl) UpsertByESPNTx(ctx context.Context, tx *sqlx.Tx, m model.Transaction) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".transaction.UpsertByESPN")
	defer scope.End()

	scope.SetAttribute(constant.OtelQueryAttributeKey, transactionFindSQL)

	var id int64

	err := dbx.NamedGetP(ctx, tx, transactionFindSQL,
		map[string]any{"league_id": m.LeagueID, "espn_id": m.ESPNID}, &id)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		scope.SetAttribute(constant.OtelQueryAttributeKey, transactionInsertSQL)

		if ierr := dbx.NamedExecP(ctx, tx, transactionInsertSQL, transactionArgs(m)); ierr != nil {
			scope.TraceError(ierr)

			return false, ierr
		}

		return true, nil
	case err != nil:
		scope.TraceError(err)

		return false, err
	default:
		args := transactionArgs(m)

		args["id"] = id

		scope.SetAttribute(constant.OtelQueryAttributeKey, transactionUpdateSQL)

		if uerr := dbx.NamedExecP(ctx, tx, transactionUpdateSQL, args); uerr != nil {
			scope.TraceError(uerr)

			return false, uerr
		}

		return false, nil
	}
}

// InsertTx creates a transaction row (used when espn_id is absent).
func (repo *repositoryImpl) InsertTx(ctx context.Context, tx *sqlx.Tx, m model.Transaction) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".transaction.Insert")
	defer scope.End()

	scope.SetAttribute(constant.OtelQueryAttributeKey, transactionInsertSQL)

	if err := dbx.NamedExecP(ctx, tx, transactionInsertSQL, transactionArgs(m)); err != nil {
		scope.TraceError(err)

		return err
	}

	return nil
}
