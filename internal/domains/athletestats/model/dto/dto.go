// Package dto contains request/response models for the athlete-stats domain.
package dto

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/drf"
)

// AthleteStatsRow is an athlete season stats row joined with league and sport.
type AthleteStatsRow struct {
	ID            int64           `db:"id"`
	AthleteESPNID string          `db:"athlete_espn_id"`
	AthleteName   string          `db:"athlete_name"`
	SeasonYear    int             `db:"season_year"`
	SeasonType    int             `db:"season_type"`
	Stats         json.RawMessage `db:"stats"`
	LeagueSlug    string          `db:"league_slug"`
	SportSlug     string          `db:"sport_slug"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

// AthleteStatsResponse matches DRF AthleteSeasonStatsSerializer.
type AthleteStatsResponse struct {
	ID            int64           `json:"id"`
	AthleteESPNID string          `json:"athlete_espn_id"`
	AthleteName   string          `json:"athlete_name"`
	SeasonYear    int             `json:"season_year"`
	SeasonType    int             `json:"season_type"`
	Stats         json.RawMessage `json:"stats"`
	LeagueSlug    string          `json:"league_slug"`
	SportSlug     string          `json:"sport_slug"`
	CreatedAt     drf.DateTime    `json:"created_at"`
	UpdatedAt     drf.DateTime    `json:"updated_at"`
}

// NewAthleteStatsResponse builds the serializer output from a joined row.
func NewAthleteStatsResponse(r AthleteStatsRow) AthleteStatsResponse {
	return AthleteStatsResponse{
		ID:            r.ID,
		AthleteESPNID: r.AthleteESPNID,
		AthleteName:   r.AthleteName,
		SeasonYear:    r.SeasonYear,
		SeasonType:    r.SeasonType,
		Stats:         drf.JSONBObject(r.Stats),
		LeagueSlug:    r.LeagueSlug,
		SportSlug:     r.SportSlug,
		CreatedAt:     drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt:     drf.NewDateTimeValue(r.UpdatedAt),
	}
}
