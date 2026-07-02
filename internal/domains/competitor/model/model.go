package model

import (
	"encoding/json"

	"go-espn-api/shared/model"
)

const (
	TableName  = "competitors"
	EntityName = "competitor"

	FieldID         = "id"
	FieldEventID    = "event_id"
	FieldTeamID     = "team_id"
	FieldHomeAway   = "home_away"
	FieldScore      = "score"
	FieldWinner     = "winner"
	FieldLineScores = "line_scores"
	FieldRecords    = "records"
	FieldStatistics = "statistics"
	FieldLeaders    = "leaders"
	FieldOrder      = "order"
	FieldRawData    = "raw_data"
)

type Competitor struct {
	ID         int64           `db:"id"`
	EventID    int64           `db:"event_id"`
	TeamID     int64           `db:"team_id"`
	HomeAway   string          `db:"home_away"`
	Score      string          `db:"score"`
	Winner     *bool           `db:"winner"`
	LineScores json.RawMessage `db:"line_scores"`
	Records    json.RawMessage `db:"records"`
	Statistics json.RawMessage `db:"statistics"`
	Leaders    json.RawMessage `db:"leaders"`
	Order      int             `db:"order"`
	RawData    json.RawMessage `db:"raw_data"`
	model.Timestamp
}
