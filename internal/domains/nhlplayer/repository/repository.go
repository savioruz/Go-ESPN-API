// Package repository provides data access for the NHL player domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/nhlplayer/model"
	"go-espn-api/internal/domains/nhlplayer/model/dto"
	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ListFilter holds the NHL player list query parameters.
type ListFilter struct {
	Search   string
	Ordering string
}

// NHLPlayer defines read and ingest-write access for NHL players.
type NHLPlayer interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.PlayerRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.PlayerRow, error)
	TeamsByIDs(ctx context.Context, ids []int64) (map[int64]teamdto.TeamResponse, error)

	// UpsertTx update-or-creates a player by player_id; reports created.
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NHLPlayer) (bool, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new NHL player repository.
func New(db *postgres.Connection, otl otel.Otel) NHLPlayer {
	return &repositoryImpl{db: db, otel: otl}
}

const playerSelect = "SELECT " + dto.PlayerColumns + " FROM nhl_players"

var playerOrdering = map[string]string{
	"last_name":      "last_name",
	"first_name":     "first_name",
	"sweater_number": "sweater_number",
}

func playerConditions(f ListFilter, args map[string]any) string {
	conds := []string{"is_active = TRUE"}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(first_name ILIKE :search OR last_name ILIKE :search OR full_name ILIKE :search)")
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.PlayerRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_player.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, playerOrdering, "last_name ASC, first_name ASC")
	query := playerSelect + playerConditions(f, args) + " ORDER BY " + order + ", id LIMIT :limit OFFSET :offset"

	items := []dto.PlayerRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_player.Count")
	defer scope.End()

	args := map[string]any{}
	query := "SELECT COUNT(id) FROM nhl_players" + playerConditions(f, args)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.PlayerRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_player.GetByID")
	defer scope.End()

	var row dto.PlayerRow

	err := dbx.NamedGet(ctx, repo.db, playerSelect+" WHERE is_active = TRUE AND id = :id", map[string]any{"id": id}, &row)
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

// TeamsByIDs batch-loads teams for the given ids, returning a lookup keyed by id.
func (repo *repositoryImpl) TeamsByIDs(ctx context.Context, ids []int64) (map[int64]teamdto.TeamResponse, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_player.TeamsByIDs")
	defer scope.End()

	if len(ids) == 0 {
		return map[int64]teamdto.TeamResponse{}, nil
	}

	rows := []teamdto.TeamRow{}

	query := "SELECT " + teamdto.TeamColumns + " FROM nhl_teams WHERE id = ANY(:ids)"
	if err := dbx.NamedSelect(ctx, repo.db, query, map[string]any{"ids": pq.Int64Array(ids)}, &rows); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return teamdto.TeamMap(rows), nil
}

const playerUpsertSQL = `
	INSERT INTO nhl_players (player_id, first_name, last_name, full_name,
		sweater_number, position, current_team_id, is_active, headshot_url, raw_data)
	VALUES (:player_id, :first_name, :last_name, :full_name,
		:sweater_number, :position, :current_team_id, :is_active, :headshot_url, :raw_data)
	ON CONFLICT (player_id) DO UPDATE SET
		first_name = EXCLUDED.first_name,
		last_name = EXCLUDED.last_name,
		full_name = EXCLUDED.full_name,
		sweater_number = EXCLUDED.sweater_number,
		position = EXCLUDED.position,
		current_team_id = EXCLUDED.current_team_id,
		is_active = EXCLUDED.is_active,
		headshot_url = EXCLUDED.headshot_url,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING (xmax = 0) AS inserted`

// UpsertTx update-or-creates a player by player_id, reporting whether created.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NHLPlayer) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".nhl_player.Upsert")
	defer scope.End()

	args := map[string]any{
		"player_id":       m.PlayerID,
		"first_name":      m.FirstName,
		"last_name":       m.LastName,
		"full_name":       m.FullName,
		"sweater_number":  m.SweaterNumber,
		"position":        m.Position,
		"current_team_id": m.CurrentTeamID,
		"is_active":       m.IsActive,
		"headshot_url":    m.HeadshotURL,
		"raw_data":        dbx.JSONOrDefault(m.RawData, "{}"),
	}

	var inserted bool
	if err := dbx.NamedGetP(ctx, tx, playerUpsertSQL, args, &inserted); err != nil {
		scope.TraceError(err)

		return false, err
	}

	return inserted, nil
}
