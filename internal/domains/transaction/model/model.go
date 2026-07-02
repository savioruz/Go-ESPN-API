package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "transactions"
	EntityName = "transaction"

	FieldID            = "id"
	FieldLeagueID      = "league_id"
	FieldTeamID        = "team_id"
	FieldESPNID        = "espn_id"
	FieldDate          = "date"
	FieldDescription   = "description"
	FieldType          = "type"
	FieldAthleteName   = "athlete_name"
	FieldAthleteESPNID = "athlete_espn_id"
	FieldRawData       = "raw_data"
)

type Transaction struct {
	ID            int64           `db:"id"`
	LeagueID      int64           `db:"league_id"`
	TeamID        *int64          `db:"team_id"`
	ESPNID        string          `db:"espn_id"`
	Date          *time.Time      `db:"date"`
	Description   string          `db:"description"`
	Type          string          `db:"type"`
	AthleteName   string          `db:"athlete_name"`
	AthleteESPNID string          `db:"athlete_espn_id"`
	RawData       json.RawMessage `db:"raw_data"`
	model.Timestamp
}
