package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"go-espn-api/infras/espn"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	leagueRepo "go-espn-api/internal/domains/league/repository"
	newsModel "go-espn-api/internal/domains/news/model"
	newsRepo "go-espn-api/internal/domains/news/repository"
	sportRepo "go-espn-api/internal/domains/sport/repository"
	"go-espn-api/shared/constant"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// maxHeadlineLen caps a persisted news headline length.
const maxHeadlineLen = 500

// NewsService ingests news articles from ESPN.
type NewsService struct {
	espn    espn.ESPN
	db      *postgres.Connection
	otel    otel.Otel
	sports  sportRepo.Sport
	leagues leagueRepo.League
	news    newsRepo.News
}

// NewNewsService constructs a NewsService.
func NewNewsService(
	espnClient espn.ESPN,
	db *postgres.Connection,
	otl otel.Otel,
	sports sportRepo.Sport,
	leagues leagueRepo.League,
	news newsRepo.News,
) *NewsService {
	return &NewsService{espn: espnClient, db: db, otel: otl, sports: sports, leagues: leagues, news: news}
}

// IngestNews fetches and upserts news articles for a sport and league.
func (s *NewsService) IngestNews(ctx context.Context, sport, league string, limit int) (IngestionResult, error) {
	ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".ingest.News")
	defer scope.End()

	scope.SetAttributes(map[string]any{"sport": sport, "league": league, "limit": limit})

	result := newResult()

	err := withTx(s.db, func(tx *sqlx.Tx) error {
		leagueID, err := resolveSportLeague(ctx, tx, s.sports, s.leagues, sport, league)
		if err != nil {
			return err
		}

		resp, err := s.espn.GetNews(ctx, sport, league, limit)
		if err != nil {
			return fmt.Errorf("fetch news: %w", err)
		}

		var root map[string]any
		if err := json.Unmarshal(resp.Data, &root); err != nil {
			return fmt.Errorf("decode news: %w", err)
		}

		articles := mslice(root, "articles")
		if len(articles) == 0 {
			log.Info().Str("sport", sport).Str("league", league).Msg("no_news_found")

			return nil
		}

		for _, raw := range articles {
			m, ok := parseArticle(leagueID, asMap(raw))
			if !ok {
				result.Errors++

				continue
			}

			created, err := s.news.UpsertTx(ctx, tx, m)
			if err != nil {
				return err
			}

			if created {
				result.Created++
			} else {
				result.Updated++
			}
		}

		return nil
	})
	if err != nil {
		scope.TraceError(err)

		return IngestionResult{}, fmt.Errorf("failed to ingest news: %w", err)
	}

	scope.SetAttributes(map[string]any{"created": result.Created, "updated": result.Updated, "errors": result.Errors})

	return result, nil
}

// parseArticle mirrors _parse_article. It returns ok=false when the article has
// no espn id or headline (Python returns None → counted as an error).
func parseArticle(leagueID int64, item map[string]any) (newsModel.NewsArticle, bool) {
	espnID := mid(item, "dataSourceIdentifier")
	if espnID == "" {
		espnID = mid(item, "id")
	}

	headline := mstrOr(item, "headline", "title")

	if espnID == "" || headline == "" {
		return newsModel.NewsArticle{}, false
	}

	lid := leagueID

	return newsModel.NewsArticle{
		ESPNID:       espnID,
		Headline:     truncate(headline, maxHeadlineLen),
		Description:  mstrOr(item, "description", "abstract"),
		Story:        mstr(item, "story"),
		Published:    parseDTPtr(mstr(item, "published")),
		LastModified: parseDTPtr(mstr(item, "lastModified")),
		Type:         mstr(item, "type"),
		LeagueID:     &lid,
		Categories:   rawObj(item["categories"], "[]"),
		Images:       rawObj(item["images"], "[]"),
		Links:        rawObj(item["links"], "{}"),
		RawData:      rawObj(item, "{}"),
	}, true
}
