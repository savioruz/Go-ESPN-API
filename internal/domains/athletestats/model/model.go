package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "athlete_season_stats"
	EntityName = "athlete_season_stats"

	FieldID            = "id"
	FieldAthleteID     = "athlete_id"
	FieldLeagueID      = "league_id"
	FieldAthleteESPNID = "athlete_espn_id"
	FieldAthleteName   = "athlete_name"
	FieldSeasonYear    = "season_year"
	FieldSeasonType    = "season_type"
	FieldStats         = "stats"
	FieldRawData       = "raw_data"
)

type AthleteSeasonStats struct {
	ID            int64           `db:"id"`
	AthleteID     *int64          `db:"athlete_id"`
	LeagueID      int64           `db:"league_id"`
	AthleteESPNID string          `db:"athlete_espn_id"`
	AthleteName   string          `db:"athlete_name"`
	SeasonYear    int             `db:"season_year"`
	SeasonType    int             `db:"season_type"`
	Stats         json.RawMessage `db:"stats"`
	RawData       json.RawMessage `db:"raw_data"`
	model.Timestamp
}
