package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "athletes"
	EntityName = "athlete"

	FieldID                   = "id"
	FieldESPNID               = "espn_id"
	FieldUID                  = "uid"
	FieldFirstName            = "first_name"
	FieldLastName             = "last_name"
	FieldFullName             = "full_name"
	FieldDisplayName          = "display_name"
	FieldShortName            = "short_name"
	FieldTeamID               = "team_id"
	FieldPosition             = "position"
	FieldPositionAbbreviation = "position_abbreviation"
	FieldJersey               = "jersey"
	FieldIsActive             = "is_active"
	FieldHeight               = "height"
	FieldWeight               = "weight"
	FieldAge                  = "age"
	FieldBirthDate            = "birth_date"
	FieldBirthPlace           = "birth_place"
	FieldHeadshot             = "headshot"
	FieldLinks                = "links"
	FieldRawData              = "raw_data"
)

type Athlete struct {
	ID                   int64           `db:"id"`
	ESPNID               string          `db:"espn_id"`
	UID                  string          `db:"uid"`
	FirstName            string          `db:"first_name"`
	LastName             string          `db:"last_name"`
	FullName             string          `db:"full_name"`
	DisplayName          string          `db:"display_name"`
	ShortName            string          `db:"short_name"`
	TeamID               *int64          `db:"team_id"`
	Position             string          `db:"position"`
	PositionAbbreviation string          `db:"position_abbreviation"`
	Jersey               string          `db:"jersey"`
	IsActive             bool            `db:"is_active"`
	Height               string          `db:"height"`
	Weight               *int            `db:"weight"`
	Age                  *int            `db:"age"`
	BirthDate            *time.Time      `db:"birth_date"`
	BirthPlace           string          `db:"birth_place"`
	Headshot             string          `db:"headshot"`
	Links                json.RawMessage `db:"links"`
	RawData              json.RawMessage `db:"raw_data"`
	model.Timestamp
}
