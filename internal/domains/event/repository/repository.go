// Package repository provides data access for the event domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/event/model"
	"go-espn-api/internal/domains/event/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ListFilter holds the event list query parameters.
type ListFilter struct {
	Sport      string
	League     string
	Date       string
	DateFrom   string
	DateTo     string
	Status     string
	SeasonYear *int
	SeasonType *int
	Team       string
	Search     string
	Ordering   string
}

// UpsertResult reports the id and whether the row was created (vs updated).
type UpsertResult struct {
	ID       int64 `db:"id"`
	Inserted bool  `db:"inserted"`
}

// StuckEventRef identifies an event past kickoff that is still marked
// scheduled/in_progress, together with the sport/league slugs needed to
// re-ingest its scoreboard.
type StuckEventRef struct {
	SportSlug  string    `db:"sport_slug"`
	LeagueSlug string    `db:"league_slug"`
	Date       time.Time `db:"date"`
}

// Event defines read and ingest-write access for events.
type Event interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.EventListRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.EventDetailRow, error)
	GetByESPNID(ctx context.Context, espnID string) (*dto.EventDetailRow, error)
	CompetitorsByEventIDs(ctx context.Context, ids []int64) ([]dto.CompetitorRow, error)

	// StuckEvents returns events past kickoff still marked scheduled/in_progress
	// within the lookback window, ordered by date, capped at limit.
	StuckEvents(ctx context.Context, lookbackDays, limit int) ([]StuckEventRef, error)

	// Upsert update-or-creates an event by (league_id, espn_id).
	Upsert(ctx context.Context, m model.Event) (UpsertResult, error)
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.Event) (UpsertResult, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new event repository.
func New(db *postgres.Connection, otl otel.Otel) Event {
	return &repositoryImpl{db: db, otel: otl}
}

const eventListSelect = `
	SELECT e.id, e.espn_id, e.date, e.name, e.short_name, e.status, e.status_detail,
	       l.slug AS league_slug, s.slug AS sport_slug, v.name AS venue_name
	FROM events e
	JOIN leagues l ON l.id = e.league_id
	JOIN sports s ON s.id = l.sport_id
	LEFT JOIN venues v ON v.id = e.venue_id`

const eventDetailSelect = `
	SELECT e.id, e.espn_id, e.uid, e.date, e.name, e.short_name,
	       e.season_year, e.season_type, e.season_slug, e.week,
	       e.status, e.status_detail, e.clock, e.period, e.attendance,
	       e.broadcasts, e.links, e.created_at, e.updated_at,
	       l.id AS league_id, l.slug AS league_slug, l.name AS league_name,
	       l.abbreviation AS league_abbreviation, s.slug AS sport_slug,
	       v.id AS venue_id, v.espn_id AS venue_espn_id, v.name AS venue_name,
	       v.city AS venue_city, v.state AS venue_state, v.country AS venue_country,
	       v.is_indoor AS venue_is_indoor, v.capacity AS venue_capacity,
	       v.created_at AS venue_created_at, v.updated_at AS venue_updated_at
	FROM events e
	JOIN leagues l ON l.id = e.league_id
	JOIN sports s ON s.id = l.sport_id
	LEFT JOIN venues v ON v.id = e.venue_id`

const competitorsSelect = `
	SELECT c.event_id, c.id, c.home_away, c.score, c.winner,
	       c.line_scores, c.records, c.statistics, c.leaders, c."order",
	       t.id AS team_id, t.espn_id AS team_espn_id, t.abbreviation AS team_abbreviation,
	       t.display_name AS team_display_name, t.short_display_name AS team_short_display_name,
	       t.location AS team_location, t.color AS team_color, t.logos AS team_logos
	FROM competitors c
	JOIN teams t ON t.id = c.team_id
	WHERE c.event_id = ANY(:ids)
	ORDER BY c.event_id, c."order"`

var eventOrdering = map[string]string{
	"date":       "e.date",
	"created_at": "e.created_at",
}

func eventConditions(f ListFilter, args map[string]any) string {
	conds := []string{}

	if f.Sport != "" {
		args["sport"] = f.Sport

		conds = append(conds, "LOWER(s.slug) = LOWER(:sport)")
	}

	if f.League != "" {
		args["league"] = f.League

		conds = append(conds, "LOWER(l.slug) = LOWER(:league)")
	}

	// Note: use CAST(... AS date) rather than the "::date" cast operator — the
	// double-colon confuses sqlx's named-parameter scanner and breaks Prepare.
	if f.Date != "" {
		args["date_eq"] = f.Date

		conds = append(conds, "CAST(e.date AS date) = :date_eq")
	}

	if f.DateFrom != "" {
		args["date_from"] = f.DateFrom

		conds = append(conds, "CAST(e.date AS date) >= :date_from")
	}

	if f.DateTo != "" {
		args["date_to"] = f.DateTo

		conds = append(conds, "CAST(e.date AS date) <= :date_to")
	}

	if f.Status != "" {
		args["status"] = f.Status

		conds = append(conds, "e.status = :status")
	}

	if f.SeasonYear != nil {
		args["season_year"] = *f.SeasonYear

		conds = append(conds, "e.season_year = :season_year")
	}

	if f.SeasonType != nil {
		args["season_type"] = *f.SeasonType

		conds = append(conds, "e.season_type = :season_type")
	}

	if f.Team != "" {
		args["team"] = f.Team

		conds = append(conds, `EXISTS (
			SELECT 1 FROM competitors tc
			JOIN teams tt ON tt.id = tc.team_id
			WHERE tc.event_id = e.id
			  AND (tt.espn_id = :team OR LOWER(tt.abbreviation) = LOWER(:team)))`)
	}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(e.name ILIKE :search OR e.short_name ILIKE :search)")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.EventListRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, eventOrdering, "e.date DESC")
	query := eventListSelect + eventConditions(f, args) + " ORDER BY " + order + ", e.id LIMIT :limit OFFSET :offset"

	items := []dto.EventListRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.Count")
	defer scope.End()

	args := map[string]any{}
	query := `SELECT COUNT(e.id) FROM events e
		JOIN leagues l ON l.id = e.league_id
		JOIN sports s ON s.id = l.sport_id
		LEFT JOIN venues v ON v.id = e.venue_id` + eventConditions(f, args)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) getDetail(ctx context.Context, clause string, args map[string]any) (*dto.EventDetailRow, error) {
	var row dto.EventDetailRow

	err := dbx.NamedGet(ctx, repo.db, eventDetailSelect+clause, args, &row)
	if errors.Is(err, sql.ErrNoRows) {
		//nolint:nilnil // (nil, nil) signals not-found; callers check for a nil result
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.EventDetailRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.GetByID")
	defer scope.End()

	row, err := repo.getDetail(ctx, " WHERE e.id = :id", map[string]any{"id": id})
	if err != nil {
		scope.TraceError(err)
	}

	return row, err
}

func (repo *repositoryImpl) GetByESPNID(ctx context.Context, espnID string) (*dto.EventDetailRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.GetByESPNID")
	defer scope.End()

	row, err := repo.getDetail(ctx, " WHERE e.espn_id = :espn_id ORDER BY e.date DESC LIMIT 1", map[string]any{"espn_id": espnID})
	if err != nil {
		scope.TraceError(err)
	}

	return row, err
}

// CompetitorsByEventIDs fetches, in a single query, every competitor for the
// given event ids joined to its minimal team. Rows are returned ordered by
// (event_id, order) so callers can group them without re-sorting.
func (repo *repositoryImpl) CompetitorsByEventIDs(ctx context.Context, ids []int64) ([]dto.CompetitorRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.CompetitorsByEventIDs")
	defer scope.End()

	rows := []dto.CompetitorRow{}
	if len(ids) == 0 {
		return rows, nil
	}

	args := map[string]any{"ids": pq.Array(ids)}
	if err := dbx.NamedSelect(ctx, repo.db, competitorsSelect, args, &rows); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return rows, nil
}

// stuckEventsSQL selects events past kickoff still stuck scheduled/in_progress
// inside the lookback window. make_interval(days => :lookback_days) is used
// instead of an `interval` string cast because the "::" cast operator confuses
// sqlx's named-parameter scanner.
const stuckEventsSQL = `
	SELECT s.slug AS sport_slug, l.slug AS league_slug, e.date AS date
	FROM events e
	JOIN leagues l ON l.id = e.league_id
	JOIN sports s ON s.id = l.sport_id
	WHERE e.status IN ('scheduled', 'in_progress')
	  AND e.date < NOW()
	  AND e.date >= NOW() - make_interval(days => :lookback_days)
	ORDER BY e.date
	LIMIT :limit`

// StuckEvents returns events past kickoff still marked scheduled/in_progress.
func (repo *repositoryImpl) StuckEvents(ctx context.Context, lookbackDays, limit int) ([]StuckEventRef, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.StuckEvents")
	defer scope.End()

	args := map[string]any{"lookback_days": lookbackDays, "limit": limit}

	rows := []StuckEventRef{}
	if err := dbx.NamedSelect(ctx, repo.db, stuckEventsSQL, args, &rows); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return rows, nil
}

const eventUpsertSQL = `
	INSERT INTO events (league_id, venue_id, espn_id, uid, date, name, short_name,
		season_year, season_type, season_slug, week, status, status_detail, clock,
		period, attendance, broadcasts, links, raw_data)
	VALUES (:league_id, :venue_id, :espn_id, :uid, :date, :name, :short_name,
		:season_year, :season_type, :season_slug, :week, :status, :status_detail, :clock,
		:period, :attendance, :broadcasts, :links, :raw_data)
	ON CONFLICT (league_id, espn_id) DO UPDATE SET
		venue_id = EXCLUDED.venue_id,
		uid = EXCLUDED.uid,
		date = EXCLUDED.date,
		name = EXCLUDED.name,
		short_name = EXCLUDED.short_name,
		season_year = EXCLUDED.season_year,
		season_type = EXCLUDED.season_type,
		season_slug = EXCLUDED.season_slug,
		week = EXCLUDED.week,
		status = EXCLUDED.status,
		status_detail = EXCLUDED.status_detail,
		clock = EXCLUDED.clock,
		period = EXCLUDED.period,
		attendance = EXCLUDED.attendance,
		broadcasts = EXCLUDED.broadcasts,
		links = EXCLUDED.links,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING id, (xmax = 0) AS inserted`

// Upsert update-or-creates an event by (league_id, espn_id).
func (repo *repositoryImpl) Upsert(ctx context.Context, m model.Event) (UpsertResult, error) {
	return repo.upsert(ctx, repo.db.Write, m)
}

// UpsertTx is the transactional variant of Upsert.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.Event) (UpsertResult, error) {
	return repo.upsert(ctx, tx, m)
}

func (repo *repositoryImpl) upsert(ctx context.Context, p dbx.NamedPreparer, m model.Event) (UpsertResult, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".event.Upsert")
	defer scope.End()

	args := map[string]any{
		"league_id":     m.LeagueID,
		"venue_id":      m.VenueID,
		"espn_id":       m.ESPNID,
		"uid":           m.UID,
		"date":          m.Date,
		"name":          m.Name,
		"short_name":    m.ShortName,
		"season_year":   m.SeasonYear,
		"season_type":   m.SeasonType,
		"season_slug":   m.SeasonSlug,
		"week":          m.Week,
		"status":        m.Status,
		"status_detail": m.StatusDetail,
		"clock":         m.Clock,
		"period":        m.Period,
		"attendance":    m.Attendance,
		"broadcasts":    dbx.JSONOrDefault(m.Broadcasts, "[]"),
		"links":         dbx.JSONOrDefault(m.Links, "[]"),
		"raw_data":      dbx.JSONOrDefault(m.RawData, "{}"),
	}

	var res UpsertResult
	if err := dbx.NamedGetP(ctx, p, eventUpsertSQL, args, &res); err != nil {
		scope.TraceError(err)

		return UpsertResult{}, err
	}

	return res, nil
}
