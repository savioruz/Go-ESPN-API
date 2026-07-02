package model

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/model"
)

const (
	TableName  = "events"
	EntityName = "event"

	FieldID           = "id"
	FieldLeagueID     = "league_id"
	FieldVenueID      = "venue_id"
	FieldESPNID       = "espn_id"
	FieldUID          = "uid"
	FieldDate         = "date"
	FieldName         = "name"
	FieldShortName    = "short_name"
	FieldSeasonYear   = "season_year"
	FieldSeasonType   = "season_type"
	FieldSeasonSlug   = "season_slug"
	FieldWeek         = "week"
	FieldStatus       = "status"
	FieldStatusDetail = "status_detail"
	FieldClock        = "clock"
	FieldPeriod       = "period"
	FieldAttendance   = "attendance"
	FieldBroadcasts   = "broadcasts"
	FieldLinks        = "links"
	FieldRawData      = "raw_data"
)

type Event struct {
	ID           int64           `db:"id"`
	LeagueID     int64           `db:"league_id"`
	VenueID      *int64          `db:"venue_id"`
	ESPNID       string          `db:"espn_id"`
	UID          string          `db:"uid"`
	Date         time.Time       `db:"date"`
	Name         string          `db:"name"`
	ShortName    string          `db:"short_name"`
	SeasonYear   int             `db:"season_year"`
	SeasonType   int             `db:"season_type"`
	SeasonSlug   string          `db:"season_slug"`
	Week         *int            `db:"week"`
	Status       string          `db:"status"`
	StatusDetail string          `db:"status_detail"`
	Clock        string          `db:"clock"`
	Period       *int            `db:"period"`
	Attendance   *int            `db:"attendance"`
	Broadcasts   json.RawMessage `db:"broadcasts"`
	Links        json.RawMessage `db:"links"`
	RawData      json.RawMessage `db:"raw_data"`
	model.Timestamp
}
