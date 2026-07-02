package model

import "go-espn-api/shared/model"

const (
	TableName  = "sports"
	EntityName = "sport"

	FieldID   = "id"
	FieldSlug = "slug"
	FieldName = "name"
)

type Sport struct {
	ID   int64  `db:"id"`
	Slug string `db:"slug"`
	Name string `db:"name"`
	model.Timestamp
}
