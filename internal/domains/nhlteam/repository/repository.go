// Package repository provides data access for the NHL team domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/nhlteam/model"
	"go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
)

// ListFilter holds the NHL team list query parameters.
type ListFilter struct {
	Search   string
	Ordering string
}

// NHLTeam defines read and ingest-write access for NHL teams.
type NHLTeam interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.TeamRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.TeamRow, error)

	// ListActive returns all active NHL teams (id + abbreviation are the fields
	// the roster ingest needs).
	ListActive(ctx context.Context) ([]model.NHLTeam, error)
	// DeactivateAllTx marks every team inactive; the ingest re-activates the
	// teams present in the feed within the same transaction.
	DeactivateAllTx(ctx context.Context, tx *sqlx.Tx) error
	// UpsertTx update-or-creates a team by team_id; reports created.
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NHLTeam) (bool, error)
	// IDByAbbrevTx resolves a team id by abbreviation, returning nil when the
	// abbreviation is blank or unknown.
	IDByAbbrevTx(ctx context.Context, tx *sqlx.Tx, abbrev string) (*int64, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new NHL team repository.
func New(db *postgres.Connection, otl otel.Otel) NHLTeam {
	return &repositoryImpl{db: db, otel: otl}
}

const teamSelect = "SELECT " + dto.TeamColumns + " FROM nhl_teams"

var teamOrdering = map[string]string{
	"name":         "name",
	"abbreviation": "abbreviation",
}

// teamConditions always filters is_active=TRUE (matching TeamViewSet.get_queryset);
// search icontains-matches name, full_name, abbreviation.
func teamConditions(f ListFilter, args map[string]any) string {
	conds := []string{"is_active = TRUE"}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(name ILIKE :search OR full_name ILIKE :search OR abbreviation ILIKE :search)")
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.TeamRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, teamOrdering, "name ASC")
	query := teamSelect + teamConditions(f, args) + " ORDER BY " + order + ", id LIMIT :limit OFFSET :offset"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []dto.TeamRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(id) FROM nhl_teams" + teamConditions(f, args)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.TeamRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.GetByID")
	defer scope.End()

	query := teamSelect + " WHERE is_active = TRUE AND id = :id"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var row dto.TeamRow

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

const teamListActiveSQL = `
	SELECT id, team_id, abbreviation, name, full_name, franchise_id, is_active,
	       raw_data, created_at, updated_at
	FROM nhl_teams
	WHERE is_active = TRUE
	ORDER BY id`

// ListActive returns all active NHL teams.
func (repo *repositoryImpl) ListActive(ctx context.Context) ([]model.NHLTeam, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.ListActive")
	defer scope.End()

	scope.SetAttribute(constant.OtelQueryAttributeKey, teamListActiveSQL)

	items := []model.NHLTeam{}
	if err := dbx.NamedSelect(ctx, repo.db, teamListActiveSQL, map[string]any{}, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

// DeactivateAllTx marks every NHL team inactive within the given transaction.
func (repo *repositoryImpl) DeactivateAllTx(ctx context.Context, tx *sqlx.Tx) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.DeactivateAll")
	defer scope.End()

	const query = "UPDATE nhl_teams SET is_active = FALSE, updated_at = NOW()"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	if err := dbx.NamedExecP(ctx, tx, query, map[string]any{}); err != nil {
		scope.TraceError(err)

		return err
	}

	return nil
}

const teamUpsertSQL = `
	INSERT INTO nhl_teams (team_id, abbreviation, name, full_name, franchise_id,
		is_active, raw_data)
	VALUES (:team_id, :abbreviation, :name, :full_name, :franchise_id,
		:is_active, :raw_data)
	ON CONFLICT (team_id) DO UPDATE SET
		abbreviation = EXCLUDED.abbreviation,
		name = EXCLUDED.name,
		full_name = EXCLUDED.full_name,
		franchise_id = EXCLUDED.franchise_id,
		is_active = EXCLUDED.is_active,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING (xmax = 0) AS inserted`

// UpsertTx update-or-creates a team by team_id, reporting whether it was created.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NHLTeam) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.Upsert")
	defer scope.End()

	args := map[string]any{
		"team_id":      m.TeamID,
		"abbreviation": m.Abbreviation,
		"name":         m.Name,
		"full_name":    m.FullName,
		"franchise_id": m.FranchiseID,
		"is_active":    m.IsActive,
		"raw_data":     dbx.JSONOrDefault(m.RawData, "{}"),
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, teamUpsertSQL)

	var inserted bool
	if err := dbx.NamedGetP(ctx, tx, teamUpsertSQL, args, &inserted); err != nil {
		scope.TraceError(err)

		return false, err
	}

	return inserted, nil
}

const teamIDByAbbrevSQL = `SELECT id FROM nhl_teams WHERE abbreviation = :abbreviation LIMIT 1`

// IDByAbbrevTx resolves a team id by abbreviation (nullable when blank/unknown).
func (repo *repositoryImpl) IDByAbbrevTx(ctx context.Context, tx *sqlx.Tx, abbrev string) (*int64, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_team.IDByAbbrev")
	defer scope.End()

	if abbrev == "" {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, teamIDByAbbrevSQL)

	var id int64

	err := dbx.NamedGetP(ctx, tx, teamIDByAbbrevSQL, map[string]any{"abbreviation": abbrev}, &id)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return &id, nil
}
