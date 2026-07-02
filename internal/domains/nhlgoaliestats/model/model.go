package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "nhl_goalie_season_stats"
	EntityName = "nhl_goalie_season_stats"

	FieldID          = "id"
	FieldPlayerID    = "player_id"
	FieldSeason      = "season"
	FieldGamesPlayed = "games_played"
	FieldWins        = "wins"
	FieldLosses      = "losses"
	FieldSavePct     = "save_pct"
	FieldGAA         = "gaa"
	FieldShutouts    = "shutouts"
	FieldRawData     = "raw_data"
)

type NHLGoalieSeasonStats struct {
	ID          int64           `db:"id"`
	PlayerID    int64           `db:"player_id"`
	Season      string          `db:"season"`
	GamesPlayed int             `db:"games_played"`
	Wins        int             `db:"wins"`
	Losses      int             `db:"losses"`
	SavePct     *float64        `db:"save_pct"`
	GAA         *float64        `db:"gaa"`
	Shutouts    int             `db:"shutouts"`
	RawData     json.RawMessage `db:"raw_data"`
	model.Timestamp
}
