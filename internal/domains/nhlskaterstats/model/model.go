package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "nhl_skater_season_stats"
	EntityName = "nhl_skater_season_stats"

	FieldID          = "id"
	FieldPlayerID    = "player_id"
	FieldSeason      = "season"
	FieldGamesPlayed = "games_played"
	FieldGoals       = "goals"
	FieldAssists     = "assists"
	FieldPoints      = "points"
	FieldPlusMinus   = "plus_minus"
	FieldRawData     = "raw_data"
)

type NHLSkaterSeasonStats struct {
	ID          int64           `db:"id"`
	PlayerID    int64           `db:"player_id"`
	Season      string          `db:"season"`
	GamesPlayed int             `db:"games_played"`
	Goals       int             `db:"goals"`
	Assists     int             `db:"assists"`
	Points      int             `db:"points"`
	PlusMinus   int             `db:"plus_minus"`
	RawData     json.RawMessage `db:"raw_data"`
	model.Timestamp
}
