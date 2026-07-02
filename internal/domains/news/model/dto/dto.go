// Package dto contains request/response models for the news domain.
package dto

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/drf"
)

// NewsRow is a news article joined with its (nullable) league and sport slugs.
type NewsRow struct {
	ID           int64           `db:"id"`
	ESPNID       string          `db:"espn_id"`
	Headline     string          `db:"headline"`
	Description  string          `db:"description"`
	Published    *time.Time      `db:"published"`
	LastModified *time.Time      `db:"last_modified"`
	Type         string          `db:"type"`
	Categories   json.RawMessage `db:"categories"`
	Images       json.RawMessage `db:"images"`
	Links        json.RawMessage `db:"links"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
	LeagueSlug   *string         `db:"league_slug"`
	SportSlug    *string         `db:"sport_slug"`
}

// computeThumbnail returns the first images[].url or images[].href, else nil.
func computeThumbnail(images json.RawMessage) *string {
	if len(images) == 0 {
		return nil
	}

	var arr []map[string]any
	if err := json.Unmarshal(images, &arr); err != nil || len(arr) == 0 {
		return nil
	}

	first := arr[0]
	for _, key := range []string{"url", "href"} {
		if v, ok := first[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				out := s

				return &out
			}
		}
	}

	return nil
}

// NewsListResponse matches DRF NewsArticleListSerializer.
type NewsListResponse struct {
	ID          int64        `json:"id"`
	ESPNID      string       `json:"espn_id"`
	Headline    string       `json:"headline"`
	Description string       `json:"description"`
	Published   drf.DateTime `json:"published"`
	Type        string       `json:"type"`
	Thumbnail   *string      `json:"thumbnail"`
	LeagueSlug  *string      `json:"league_slug"`
	SportSlug   *string      `json:"sport_slug"`
}

// NewNewsListResponse builds the list serializer output.
func NewNewsListResponse(r NewsRow) NewsListResponse {
	return NewsListResponse{
		ID:          r.ID,
		ESPNID:      r.ESPNID,
		Headline:    r.Headline,
		Description: r.Description,
		Published:   drf.NewDateTime(r.Published),
		Type:        r.Type,
		Thumbnail:   computeThumbnail(r.Images),
		LeagueSlug:  r.LeagueSlug,
		SportSlug:   r.SportSlug,
	}
}

// NewsDetailResponse matches DRF NewsArticleSerializer.
type NewsDetailResponse struct {
	ID           int64           `json:"id"`
	ESPNID       string          `json:"espn_id"`
	Headline     string          `json:"headline"`
	Description  string          `json:"description"`
	Published    drf.DateTime    `json:"published"`
	LastModified drf.DateTime    `json:"last_modified"`
	Type         string          `json:"type"`
	Categories   json.RawMessage `json:"categories"`
	Images       json.RawMessage `json:"images"`
	Links        json.RawMessage `json:"links"`
	Thumbnail    *string         `json:"thumbnail"`
	LeagueSlug   *string         `json:"league_slug"`
	SportSlug    *string         `json:"sport_slug"`
	CreatedAt    drf.DateTime    `json:"created_at"`
	UpdatedAt    drf.DateTime    `json:"updated_at"`
}

// NewNewsDetailResponse builds the detail serializer output.
func NewNewsDetailResponse(r NewsRow) NewsDetailResponse {
	return NewsDetailResponse{
		ID:           r.ID,
		ESPNID:       r.ESPNID,
		Headline:     r.Headline,
		Description:  r.Description,
		Published:    drf.NewDateTime(r.Published),
		LastModified: drf.NewDateTime(r.LastModified),
		Type:         r.Type,
		Categories:   drf.JSONBArray(r.Categories),
		Images:       drf.JSONBArray(r.Images),
		Links:        drf.JSONBObject(r.Links),
		Thumbnail:    computeThumbnail(r.Images),
		LeagueSlug:   r.LeagueSlug,
		SportSlug:    r.SportSlug,
		CreatedAt:    drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt:    drf.NewDateTimeValue(r.UpdatedAt),
	}
}
