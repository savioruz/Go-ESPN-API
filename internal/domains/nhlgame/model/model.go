package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "nhl_games"
	EntityName = "nhl_game"

	FieldID         = "id"
	FieldGameID     = "game_id"
	FieldSeason     = "season"
	FieldGameType   = "game_type"
	FieldDate       = "date"
	FieldHomeTeamID = "home_team_id"
	FieldAwayTeamID = "away_team_id"
	FieldHomeScore  = "home_score"
	FieldAwayScore  = "away_score"
	FieldStatus     = "status"
	FieldRawData    = "raw_data"
)

type NHLGame struct {
	ID         int64           `db:"id"`
	GameID     string          `db:"game_id"`
	Season     string          `db:"season"`
	GameType   int             `db:"game_type"`
	Date       time.Time       `db:"date"`
	HomeTeamID int64           `db:"home_team_id"`
	AwayTeamID int64           `db:"away_team_id"`
	HomeScore  *int            `db:"home_score"`
	AwayScore  *int            `db:"away_score"`
	Status     string          `db:"status"`
	RawData    json.RawMessage `db:"raw_data"`
	model.Timestamp
}
