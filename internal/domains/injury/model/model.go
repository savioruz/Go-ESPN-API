package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "injuries"
	EntityName = "injury"

	FieldID            = "id"
	FieldLeagueID      = "league_id"
	FieldTeamID        = "team_id"
	FieldESPNID        = "espn_id"
	FieldAthleteESPNID = "athlete_espn_id"
	FieldAthleteName   = "athlete_name"
	FieldPosition      = "position"
	FieldStatus        = "status"
	FieldStatusDisplay = "status_display"
	FieldDescription   = "description"
	FieldInjuryType    = "injury_type"
	FieldInjuryDate    = "injury_date"
	FieldReturnDate    = "return_date"
	FieldRawData       = "raw_data"
)

type Injury struct {
	ID            int64           `db:"id"`
	LeagueID      int64           `db:"league_id"`
	TeamID        *int64          `db:"team_id"`
	ESPNID        string          `db:"espn_id"`
	AthleteESPNID string          `db:"athlete_espn_id"`
	AthleteName   string          `db:"athlete_name"`
	Position      string          `db:"position"`
	Status        string          `db:"status"`
	StatusDisplay string          `db:"status_display"`
	Description   string          `db:"description"`
	InjuryType    string          `db:"injury_type"`
	InjuryDate    *time.Time      `db:"injury_date"`
	ReturnDate    *time.Time      `db:"return_date"`
	RawData       json.RawMessage `db:"raw_data"`
	model.Timestamp
}
