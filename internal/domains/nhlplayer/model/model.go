package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "nhl_players"
	EntityName = "nhl_player"

	FieldID            = "id"
	FieldPlayerID      = "player_id"
	FieldFirstName     = "first_name"
	FieldLastName      = "last_name"
	FieldFullName      = "full_name"
	FieldSweaterNumber = "sweater_number"
	FieldPosition      = "position"
	FieldCurrentTeamID = "current_team_id"
	FieldIsActive      = "is_active"
	FieldHeadshotURL   = "headshot_url"
	FieldRawData       = "raw_data"
)

type NHLPlayer struct {
	ID            int64           `db:"id"`
	PlayerID      string          `db:"player_id"`
	FirstName     string          `db:"first_name"`
	LastName      string          `db:"last_name"`
	FullName      string          `db:"full_name"`
	SweaterNumber string          `db:"sweater_number"`
	Position      string          `db:"position"`
	CurrentTeamID *int64          `db:"current_team_id"`
	IsActive      bool            `db:"is_active"`
	HeadshotURL   string          `db:"headshot_url"`
	RawData       json.RawMessage `db:"raw_data"`
	model.Timestamp
}
