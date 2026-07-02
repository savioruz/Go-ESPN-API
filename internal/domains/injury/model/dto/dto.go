// Package dto contains request/response models for the injury domain.
package dto

import (
	"time"

	"go-espn-api/shared/drf"
)

// InjuryRow is an injury joined with league, sport and (nullable) team.
type InjuryRow struct {
	ID               int64      `db:"id"`
	AthleteESPNID    string     `db:"athlete_espn_id"`
	AthleteName      string     `db:"athlete_name"`
	Position         string     `db:"position"`
	Status           string     `db:"status"`
	StatusDisplay    string     `db:"status_display"`
	Description      string     `db:"description"`
	InjuryType       string     `db:"injury_type"`
	InjuryDate       *time.Time `db:"injury_date"`
	ReturnDate       *time.Time `db:"return_date"`
	LeagueSlug       string     `db:"league_slug"`
	SportSlug        string     `db:"sport_slug"`
	TeamAbbreviation *string    `db:"team_abbreviation"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// InjuryResponse matches DRF InjurySerializer.
type InjuryResponse struct {
	ID               int64        `json:"id"`
	AthleteESPNID    string       `json:"athlete_espn_id"`
	AthleteName      string       `json:"athlete_name"`
	Position         string       `json:"position"`
	Status           string       `json:"status"`
	StatusDisplay    string       `json:"status_display"`
	Description      string       `json:"description"`
	InjuryType       string       `json:"injury_type"`
	InjuryDate       drf.Date     `json:"injury_date"`
	ReturnDate       drf.Date     `json:"return_date"`
	LeagueSlug       string       `json:"league_slug"`
	SportSlug        string       `json:"sport_slug"`
	TeamAbbreviation *string      `json:"team_abbreviation"`
	CreatedAt        drf.DateTime `json:"created_at"`
	UpdatedAt        drf.DateTime `json:"updated_at"`
}

// NewInjuryResponse builds the serializer output from a joined row.
func NewInjuryResponse(r InjuryRow) InjuryResponse {
	return InjuryResponse{
		ID:               r.ID,
		AthleteESPNID:    r.AthleteESPNID,
		AthleteName:      r.AthleteName,
		Position:         r.Position,
		Status:           r.Status,
		StatusDisplay:    r.StatusDisplay,
		Description:      r.Description,
		InjuryType:       r.InjuryType,
		InjuryDate:       drf.NewDate(r.InjuryDate),
		ReturnDate:       drf.NewDate(r.ReturnDate),
		LeagueSlug:       r.LeagueSlug,
		SportSlug:        r.SportSlug,
		TeamAbbreviation: r.TeamAbbreviation,
		CreatedAt:        drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt:        drf.NewDateTimeValue(r.UpdatedAt),
	}
}
