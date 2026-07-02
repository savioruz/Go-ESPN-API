// Package repository provides data access for the injury domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/injury/model"
	"go-espn-api/internal/domains/injury/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
)

// ListFilter holds the injury list query parameters.
type ListFilter struct {
	Sport    string
	League   string
	Status   string
	Team     string
	Search   string
	Ordering string
}

// Injury defines read and ingest-write access for injuries.
type Injury interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.InjuryRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.InjuryRow, error)

	// DeleteByLeague clears the injury snapshot for a league.
	DeleteByLeague(ctx context.Context, leagueID int64) error
	DeleteByLeagueTx(ctx context.Context, tx *sqlx.Tx, leagueID int64) error
	// Insert appends one injury row.
	Insert(ctx context.Context, m model.Injury) error
	InsertTx(ctx context.Context, tx *sqlx.Tx, m model.Injury) error
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new injury repository.
func New(db *postgres.Connection, otl otel.Otel) Injury {
	return &repositoryImpl{db: db, otel: otl}
}

const injuryFrom = `
	FROM injuries i
	JOIN leagues l ON l.id = i.league_id
	JOIN sports s ON s.id = l.sport_id
	LEFT JOIN teams t ON t.id = i.team_id`

const injurySelect = `
	SELECT i.id, i.athlete_espn_id, i.athlete_name, i.position, i.status,
	       i.status_display, i.description, i.injury_type, i.injury_date, i.return_date,
	       l.slug AS league_slug, s.slug AS sport_slug, t.abbreviation AS team_abbreviation,
	       i.created_at, i.updated_at` + injuryFrom

var injuryOrdering = map[string]string{
	"updated_at":   "i.updated_at",
	"athlete_name": "i.athlete_name",
}

func injuryConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Sport != "" {
		args["sport"] = f.Sport

		conds = append(conds, "LOWER(s.slug) = LOWER(:sport)")
	}

	if f.League != "" {
		args["league"] = f.League

		conds = append(conds, "LOWER(l.slug) = LOWER(:league)")
	}

	if f.Status != "" {
		args["status"] = f.Status

		conds = append(conds, "LOWER(i.status) = LOWER(:status)")
	}

	if f.Team != "" {
		args["team"] = f.Team

		conds = append(conds, "LOWER(t.abbreviation) = LOWER(:team)")
	}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(i.athlete_name ILIKE :search OR i.injury_type ILIKE :search OR i.description ILIKE :search)")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.InjuryRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".injury.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, injuryOrdering, "i.updated_at DESC")
	query := injurySelect + injuryConditions(f, args) + " ORDER BY " + order + " LIMIT :limit OFFSET :offset"

	items := []dto.InjuryRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".injury.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(i.id)" + injuryFrom + injuryConditions(f, args)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.InjuryRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".injury.GetByID")
	defer scope.End()

	query := injurySelect + " WHERE i.id = :id"

	var row dto.InjuryRow

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

const injuryDeleteSQL = `DELETE FROM injuries WHERE league_id = :league_id`

const injuryInsertSQL = `
	INSERT INTO injuries (league_id, team_id, espn_id, athlete_espn_id, athlete_name,
		position, status, status_display, description, injury_type, raw_data)
	VALUES (:league_id, :team_id, :espn_id, :athlete_espn_id, :athlete_name,
		:position, :status, :status_display, :description, :injury_type, :raw_data)`

// DeleteByLeague clears the injury snapshot for a league.
func (repo *repositoryImpl) DeleteByLeague(ctx context.Context, leagueID int64) error {
	return repo.deleteByLeague(ctx, repo.db.Write, leagueID)
}

// DeleteByLeagueTx is the transactional variant of DeleteByLeague.
func (repo *repositoryImpl) DeleteByLeagueTx(ctx context.Context, tx *sqlx.Tx, leagueID int64) error {
	return repo.deleteByLeague(ctx, tx, leagueID)
}

func (repo *repositoryImpl) deleteByLeague(ctx context.Context, p dbx.NamedPreparer, leagueID int64) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".injury.DeleteByLeague")
	defer scope.End()

	if err := dbx.NamedExecP(ctx, p, injuryDeleteSQL, map[string]any{"league_id": leagueID}); err != nil {
		scope.TraceError(err)

		return err
	}

	return nil
}

// Insert appends one injury row.
func (repo *repositoryImpl) Insert(ctx context.Context, m model.Injury) error {
	return repo.insert(ctx, repo.db.Write, m)
}

// InsertTx is the transactional variant of Insert.
func (repo *repositoryImpl) InsertTx(ctx context.Context, tx *sqlx.Tx, m model.Injury) error {
	return repo.insert(ctx, tx, m)
}

func (repo *repositoryImpl) insert(ctx context.Context, p dbx.NamedPreparer, m model.Injury) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".injury.Insert")
	defer scope.End()

	args := map[string]any{
		"league_id":       m.LeagueID,
		"team_id":         m.TeamID,
		"espn_id":         m.ESPNID,
		"athlete_espn_id": m.AthleteESPNID,
		"athlete_name":    m.AthleteName,
		"position":        m.Position,
		"status":          m.Status,
		"status_display":  m.StatusDisplay,
		"description":     m.Description,
		"injury_type":     m.InjuryType,
		"raw_data":        dbx.JSONOrDefault(m.RawData, "{}"),
	}

	if err := dbx.NamedExecP(ctx, p, injuryInsertSQL, args); err != nil {
		scope.TraceError(err)

		return err
	}

	return nil
}
