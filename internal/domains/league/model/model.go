package model

import "go-espn-api/shared/model"

const (
	TableName  = "leagues"
	EntityName = "league"

	FieldID           = "id"
	FieldSportID      = "sport_id"
	FieldSlug         = "slug"
	FieldName         = "name"
	FieldAbbreviation = "abbreviation"
)

type League struct {
	ID           int64  `db:"id"`
	SportID      int64  `db:"sport_id"`
	Slug         string `db:"slug"`
	Name         string `db:"name"`
	Abbreviation string `db:"abbreviation"`
	model.Timestamp
}
