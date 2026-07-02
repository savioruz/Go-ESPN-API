package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "nhl_standings"
	EntityName = "nhl_standing"

	FieldID             = "id"
	FieldTeamID         = "team_id"
	FieldDate           = "date"
	FieldGamesPlayed    = "games_played"
	FieldWins           = "wins"
	FieldLosses         = "losses"
	FieldOTLosses       = "ot_losses"
	FieldPoints         = "points"
	FieldPointPct       = "point_pct"
	FieldDivisionName   = "division_name"
	FieldConferenceName = "conference_name"
	FieldRawData        = "raw_data"
)

type NHLStanding struct {
	ID             int64           `db:"id"`
	TeamID         int64           `db:"team_id"`
	Date           time.Time       `db:"date"`
	GamesPlayed    int             `db:"games_played"`
	Wins           int             `db:"wins"`
	Losses         int             `db:"losses"`
	OTLosses       int             `db:"ot_losses"`
	Points         int             `db:"points"`
	PointPct       *float64        `db:"point_pct"`
	DivisionName   string          `db:"division_name"`
	ConferenceName string          `db:"conference_name"`
	RawData        json.RawMessage `db:"raw_data"`
	model.Timestamp
}
