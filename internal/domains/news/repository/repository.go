// Package repository provides data access for the news domain.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/internal/domains/news/model"
	"go-espn-api/internal/domains/news/model/dto"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dbx"
	"go-espn-api/shared/drf"

	"github.com/jmoiron/sqlx"
)

// ListFilter holds the news list query parameters.
type ListFilter struct {
	Sport    string
	League   string
	DateFrom string
	Search   string
	Ordering string
}

// News defines read and ingest-write access for news articles.
type News interface {
	List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.NewsRow, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	GetByID(ctx context.Context, id int64) (*dto.NewsRow, error)

	// Upsert update-or-creates an article by espn_id; reports created.
	Upsert(ctx context.Context, m model.NewsArticle) (bool, error)
	UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NewsArticle) (bool, error)
}

type repositoryImpl struct {
	db   *postgres.Connection
	otel otel.Otel
}

// New creates a new news repository.
func New(db *postgres.Connection, otl otel.Otel) News {
	return &repositoryImpl{db: db, otel: otl}
}

const newsSelect = `
	SELECT n.id, n.espn_id, n.headline, n.description, n.published, n.last_modified,
	       n.type, n.categories, n.images, n.links, n.created_at, n.updated_at,
	       l.slug AS league_slug, s.slug AS sport_slug
	FROM news_articles n
	LEFT JOIN leagues l ON l.id = n.league_id
	LEFT JOIN sports s ON s.id = l.sport_id`

var newsOrdering = map[string]string{
	"published":  "n.published",
	"created_at": "n.created_at",
}

func newsConditions(f ListFilter, args map[string]any) string {
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

		conds = append(conds, "n.published::date >= :date_from")
	}

	if f.Search != "" {
		args["search"] = "%" + f.Search + "%"

		conds = append(conds, "(n.headline ILIKE :search OR n.description ILIKE :search)")
	}

	if len(conds) == 0 {
		return ""
	}

	return " WHERE " + strings.Join(conds, " AND ")
}

func (repo *repositoryImpl) List(ctx context.Context, f ListFilter, page, pageSize int) ([]dto.NewsRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".news.List")
	defer scope.End()

	args := map[string]any{"limit": pageSize, "offset": (page - 1) * pageSize}
	order := drf.ResolveOrdering(f.Ordering, newsOrdering, "n.published DESC")
	query := newsSelect + newsConditions(f, args) + " ORDER BY " + order + " LIMIT :limit OFFSET :offset"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	items := []dto.NewsRow{}
	if err := dbx.NamedSelect(ctx, repo.db, query, args, &items); err != nil {
		scope.TraceError(err)

		return nil, err
	}

	return items, nil
}

func (repo *repositoryImpl) Count(ctx context.Context, f ListFilter) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".news.Count")
	defer scope.End()

	args := map[string]any{}
	query := `SELECT COUNT(n.id) FROM news_articles n
		LEFT JOIN leagues l ON l.id = n.league_id
		LEFT JOIN sports s ON s.id = l.sport_id` + newsConditions(f, args)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int
	if err := dbx.NamedGet(ctx, repo.db, query, args, &count); err != nil {
		scope.TraceError(err)

		return 0, err
	}

	return count, nil
}

func (repo *repositoryImpl) GetByID(ctx context.Context, id int64) (*dto.NewsRow, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".news.GetByID")
	defer scope.End()

	query := newsSelect + " WHERE n.id = :id"
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var row dto.NewsRow

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

const newsUpsertSQL = `
	INSERT INTO news_articles (espn_id, headline, description, story, published,
		last_modified, type, league_id, categories, images, links, raw_data)
	VALUES (:espn_id, :headline, :description, :story, :published,
		:last_modified, :type, :league_id, :categories, :images, :links, :raw_data)
	ON CONFLICT (espn_id) DO UPDATE SET
		headline = EXCLUDED.headline,
		description = EXCLUDED.description,
		story = EXCLUDED.story,
		published = EXCLUDED.published,
		last_modified = EXCLUDED.last_modified,
		type = EXCLUDED.type,
		league_id = EXCLUDED.league_id,
		categories = EXCLUDED.categories,
		images = EXCLUDED.images,
		links = EXCLUDED.links,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW()
	RETURNING (xmax = 0) AS inserted`

// Upsert update-or-creates a news article by espn_id.
func (repo *repositoryImpl) Upsert(ctx context.Context, m model.NewsArticle) (bool, error) {
	return repo.upsert(ctx, repo.db.Write, m)
}

// UpsertTx is the transactional variant of Upsert.
func (repo *repositoryImpl) UpsertTx(ctx context.Context, tx *sqlx.Tx, m model.NewsArticle) (bool, error) {
	return repo.upsert(ctx, tx, m)
}

func (repo *repositoryImpl) upsert(ctx context.Context, p dbx.NamedPreparer, m model.NewsArticle) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".news.Upsert")
	defer scope.End()

	args := map[string]any{
		"espn_id":       m.ESPNID,
		"headline":      m.Headline,
		"description":   m.Description,
		"story":         m.Story,
		"published":     m.Published,
		"last_modified": m.LastModified,
		"type":          m.Type,
		"league_id":     m.LeagueID,
		"categories":    dbx.JSONOrDefault(m.Categories, "[]"),
		"images":        dbx.JSONOrDefault(m.Images, "[]"),
		"links":         dbx.JSONOrDefault(m.Links, "{}"),
		"raw_data":      dbx.JSONOrDefault(m.RawData, "{}"),
	}

	scope.SetAttribute(constant.OtelQueryAttributeKey, newsUpsertSQL)

	var inserted bool
	if err := dbx.NamedGetP(ctx, p, newsUpsertSQL, args, &inserted); err != nil {
		scope.TraceError(err)

		return false, err
	}

	return inserted, nil
}
