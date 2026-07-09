// Package repository provides data access for the team domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/team/model"
	"go-espn-api/internal/domains/team/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
)

// ListFilter holds the team list query parameters.
type ListFilter struct {
	Sport        string
	League       string
	IsActive     *bool
	Abbreviation string
	Search       string
	Ordering     string
}

// MinimalTeam holds the minimal fields used to create a placeholder team from a
// scoreboard competitor when the team is not already known.
type MinimalTeam struct {
	LeagueID         int64
	ESPNID           string
	Abbreviation     string
	DisplayName      string
	ShortDisplayName string
	Name             string
	Location         string
	Logos            []byte
}

// Team defines read and ingest-write access for teams.
type Team interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.TeamListRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.TeamDetailRow, error)
	GetByESPNID(ctx context.Context, espnID string) (*dto.TeamDetailRow, error)

	// Upsert update-or-creates a full team by (league_id, espn_id); reports created.
	Upsert(ctx context.Context, m model.Team) (bool, error)
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.Team) (bool, error)
	// GetOrCreateMinimal gets a team id by (league_id, espn_id), creating a
	// minimal placeholder only when missing (never overwriting an existing team).
	GetOrCreateMinimal(ctx context.Context, m MinimalTeam) (int64, error)
	GetOrCreateMinimalTx(ctx context.Context, tx *sqlx.Tx, m MinimalTeam) (int64, error)
	// IDByESPNInLeagueTx resolves a team id within a league by ESPN id, returning
	// nil when the ESPN id is blank or unknown.
	IDByESPNInLeagueTx(ctx context.Context, tx *sqlx.Tx, leagueID int64, espnID string) (*int64, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new team repository.
func New(db *postgres.Connection, otl otel.Otel) Team {
	return &repositoryImpl{db: db, otel: otl}
}

const teamListSelect = `
	SELECT t.id, t.espn_id, t.abbreviation, t.display_name, t.short_display_name,
	       t.location, t.color, t.logos, t.is_active,
	       l.slug AS league_slug, s.slug AS sport_slug
	FROM teams t
	JOIN leagues l ON l.id = t.league_id
	JOIN sports s ON s.id = l.sport_id`

const teamDetailSelect = `
	SELECT t.id, t.espn_id, t.uid, t.slug, t.abbreviation, t.display_name,
	       t.short_display_name, t.name, t.nickname, t.location, t.color,
	       t.alternate_color, t.is_active, t.is_all_star, t.logos,
	       t.created_at, t.updated_at,
	       l.id AS league_id, l.slug AS league_slug, l.name AS league_name,
	       l.abbreviation AS league_abbreviation, s.slug AS sport_slug
	FROM teams t
	JOIN leagues l ON l.id = t.league_id
	JOIN sports s ON s.id = l.sport_id`

var teamOrdering = map[string]string{
	"display_name": "t.display_name",
	"abbreviation": "t.abbreviation",
	"created_at":   "t.created_at",
}

// teamConditions builds the WHERE clause. The queryset always filters
// is_active=TRUE (matching TeamViewSet.get_queryset); an explicit ?is_active
// filter narrows further.
func teamConditions(f ListFilter, args map[string]any) string {
	conds := []string{"t.is_active = TRUE"}

	if f.Sport != "" {
		args["sport"] = f.Sport

		conds = append(conds, "LOWER(s.slug) = LOWER(:sport)")
	}

	if f.League != "" {
		args["league"] = f.League

		conds = append(conds, "LOWER(l.slug) = LOWER(:league)")
	}

	if f.IsActive != nil {
		args["is_active"] = *f.IsActive

		conds = append(conds, "t.is_active = :is_active")
	}

	if f.Abbreviation != "" {
		args["abbreviation"] = f.Abbreviation

		conds = append(conds, "LOWER(t.abbreviation) = LOWER(:abbreviation)")
	}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(t.display_name ILIKE :search OR t.abbreviation ILIKE :search OR t.location ILIKE :search OR t.name ILIKE :search)")
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.TeamListRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, teamOrdering, "t.display_name ASC")
	query := teamListSelect + teamConditions(f, args) + " ORDER BY " + order + ", t.id LIMIT :limit OFFSET :offset"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []dto.TeamListRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.Count")
	defer scope.End()

	args := map[string]any{}
	query := `SELECT COUNT(t.id) FROM teams t
		JOIN leagues l ON l.id = t.league_id
		JOIN sports s ON s.id = l.sport_id` + teamConditions(f, args)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) getDetail(ctx context.Context, clause string, args map[string]any) (*dto.TeamDetailRow, error) {
	var row dto.TeamDetailRow

	err := dbx.NamedGet(ctx, repo.db, teamDetailSelect+clause, args, &row)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.TeamDetailRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.GetByID")
	defer scope.End()

	const clause = " WHERE t.is_active = TRUE AND t.id = :id"
	scope.SetAttribute(constant.OtelQueryAttributeKey, teamDetailSelect+clause)

	row, err := repo.getDetail(ctx, clause, map[string]any{"id": id})
	if err != nil {
		scope.TraceError(err)
	}

	return row, err
}

func (repo *repositoryImpl) GetByESPNID(ctx context.Context, espnID string) (*dto.TeamDetailRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.GetByESPNID")
	defer scope.End()

	const clause = " WHERE t.is_active = TRUE AND t.espn_id = :espn_id ORDER BY t.display_name LIMIT 1"
	scope.SetAttribute(constant.OtelQueryAttributeKey, teamDetailSelect+clause)

	row, err := repo.getDetail(ctx, clause, map[string]any{"espn_id": espnID})
	if err != nil {
		scope.TraceError(err)
	}

	return row, err
}

const teamUpsertSQL = `
	INSERT INTO teams (league_id, espn_id, uid, slug, abbreviation, display_name,
		short_display_name, name, nickname, location, color, alternate_color,
		is_active, is_all_star, logos, links, raw_data)
	VALUES (:league_id, :espn_id, :uid, :slug, :abbreviation, :display_name,
		:short_display_name, :name, :nickname, :location, :color, :alternate_color,
		:is_active, :is_all_star, :logos, :links, :raw_data)
	ON CONFLICT (league_id, espn_id) DO UPDATE SET
		uid = EXCLUDED.uid,
		slug = EXCLUDED.slug,
		abbreviation = EXCLUDED.abbreviation,
		display_name = EXCLUDED.display_name,
		short_display_name = EXCLUDED.short_display_name,
		name = EXCLUDED.name,
		nickname = EXCLUDED.nickname,
		location = EXCLUDED.location,
		color = EXCLUDED.color,
		alternate_color = EXCLUDED.alternate_color,
		is_active = EXCLUDED.is_active,
		is_all_star = EXCLUDED.is_all_star,
		logos = EXCLUDED.logos,
		links = EXCLUDED.links,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING (xmax = 0) AS inserted`

// Upsert update-or-creates a full team by (league_id, espn_id).
func (repo *repositoryImpl) Upsert(ctx context.Context, m model.Team) (bool, error) {
	return repo.upsert(ctx, repo.db.Write, m)
}

// UpsertTx is the transactional variant of Upsert.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.Team) (bool, error) {
	return repo.upsert(ctx, tx, m)
}

func (repo *repositoryImpl) upsert(ctx context.Context, p dbx.NamedPreparer, m model.Team) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.Upsert")
	defer scope.End()

	args := map[string]any{
		"league_id":          m.LeagueID,
		"espn_id":            m.ESPNID,
		"uid":                m.UID,
		"slug":               m.Slug,
		"abbreviation":       m.Abbreviation,
		"display_name":       m.DisplayName,
		"short_display_name": m.ShortDisplayName,
		"name":               m.Name,
		"nickname":           m.Nickname,
		"location":           m.Location,
		"color":              m.Color,
		"alternate_color":    m.AlternateColor,
		"is_active":          m.IsActive,
		"is_all_star":        m.IsAllStar,
		"logos":              dbx.JSONOrDefault(m.Logos, "[]"),
		"links":              dbx.JSONOrDefault(m.Links, "[]"),
		"raw_data":           dbx.JSONOrDefault(m.RawData, "{}"),
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, teamUpsertSQL)

	var inserted bool
	if err := dbx.NamedGetP(ctx, p, teamUpsertSQL, args, &inserted); err != nil {
		scope.TraceError(err)

		return false, err
	}

	return inserted, nil
}

// teamMinimalSQL creates a placeholder team only when missing. The no-op DO
// UPDATE preserves an existing (possibly fully-populated) team while still
// RETURNING its id.
const teamMinimalSQL = `
	INSERT INTO teams (league_id, espn_id, abbreviation, display_name,
		short_display_name, name, location, logos)
	VALUES (:league_id, :espn_id, :abbreviation, :display_name,
		:short_display_name, :name, :location, :logos)
	ON CONFLICT (league_id, espn_id) DO UPDATE SET league_id = EXCLUDED.league_id
	RETURNING id`

// GetOrCreateMinimal gets a team id, creating a minimal placeholder if missing.
func (repo *repositoryImpl) GetOrCreateMinimal(ctx context.Context, m MinimalTeam) (int64, error) {
	return repo.getOrCreateMinimal(ctx, repo.db.Write, m)
}

// GetOrCreateMinimalTx is the transactional variant of GetOrCreateMinimal.
func (repo *repositoryImpl) GetOrCreateMinimalTx(ctx context.Context, tx *sqlx.Tx, m MinimalTeam) (int64, error) {
	return repo.getOrCreateMinimal(ctx, tx, m)
}

func (repo *repositoryImpl) getOrCreateMinimal(ctx context.Context, p dbx.NamedPreparer, m MinimalTeam) (int64, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.GetOrCreateMinimal")
	defer scope.End()

	args := map[string]any{
		"league_id":          m.LeagueID,
		"espn_id":            m.ESPNID,
		"abbreviation":       m.Abbreviation,
		"display_name":       m.DisplayName,
		"short_display_name": m.ShortDisplayName,
		"name":               m.Name,
		"location":           m.Location,
		"logos":              dbx.JSONOrDefault(m.Logos, "[]"),
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, teamMinimalSQL)

	var id int64
	if err := dbx.NamedGetP(ctx, p, teamMinimalSQL, args, &id); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return id, nil
}

const teamIDByESPNSQL = `SELECT id FROM teams WHERE league_id = :league_id AND espn_id = :espn_id LIMIT 1`

// IDByESPNInLeagueTx resolves a team id within a league by ESPN id (nullable).
func (repo *repositoryImpl) IDByESPNInLeagueTx(ctx context.Context, tx *sqlx.Tx, leagueID int64, espnID string) (*int64, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".team.IDByESPNInLeague")
	defer scope.End()

	if espnID == "" {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, teamIDByESPNSQL)

	var id int64

	err := dbx.NamedGetP(ctx, tx, teamIDByESPNSQL, map[string]any{"league_id": leagueID, "espn_id": espnID}, &id)
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
