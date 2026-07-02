package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "teams"
	EntityName = "team"

	FieldID               = "id"
	FieldLeagueID         = "league_id"
	FieldESPNID           = "espn_id"
	FieldUID              = "uid"
	FieldSlug             = "slug"
	FieldAbbreviation     = "abbreviation"
	FieldDisplayName      = "display_name"
	FieldShortDisplayName = "short_display_name"
	FieldName             = "name"
	FieldNickname         = "nickname"
	FieldLocation         = "location"
	FieldColor            = "color"
	FieldAlternateColor   = "alternate_color"
	FieldIsActive         = "is_active"
	FieldIsAllStar        = "is_all_star"
	FieldLogos            = "logos"
	FieldLinks            = "links"
	FieldRawData          = "raw_data"
)

type Team struct {
	ID               int64           `db:"id"`
	LeagueID         int64           `db:"league_id"`
	ESPNID           string          `db:"espn_id"`
	UID              string          `db:"uid"`
	Slug             string          `db:"slug"`
	Abbreviation     string          `db:"abbreviation"`
	DisplayName      string          `db:"display_name"`
	ShortDisplayName string          `db:"short_display_name"`
	Name             string          `db:"name"`
	Nickname         string          `db:"nickname"`
	Location         string          `db:"location"`
	Color            string          `db:"color"`
	AlternateColor   string          `db:"alternate_color"`
	IsActive         bool            `db:"is_active"`
	IsAllStar        bool            `db:"is_all_star"`
	Logos            json.RawMessage `db:"logos"`
	Links            json.RawMessage `db:"links"`
	RawData          json.RawMessage `db:"raw_data"`
	model.Timestamp
}
