package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "news_articles"
	EntityName = "news_article"

	FieldID           = "id"
	FieldESPNID       = "espn_id"
	FieldHeadline     = "headline"
	FieldDescription  = "description"
	FieldStory        = "story"
	FieldPublished    = "published"
	FieldLastModified = "last_modified"
	FieldType         = "type"
	FieldLeagueID     = "league_id"
	FieldCategories   = "categories"
	FieldImages       = "images"
	FieldLinks        = "links"
	FieldRawData      = "raw_data"
)

type NewsArticle struct {
	ID           int64           `db:"id"`
	ESPNID       string          `db:"espn_id"`
	Headline     string          `db:"headline"`
	Description  string          `db:"description"`
	Story        string          `db:"story"`
	Published    *time.Time      `db:"published"`
	LastModified *time.Time      `db:"last_modified"`
	Type         string          `db:"type"`
	LeagueID     *int64          `db:"league_id"`
	Categories   json.RawMessage `db:"categories"`
	Images       json.RawMessage `db:"images"`
	Links        json.RawMessage `db:"links"`
	RawData      json.RawMessage `db:"raw_data"`
	model.Timestamp
}
