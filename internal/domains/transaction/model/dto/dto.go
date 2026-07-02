// Package dto contains request/response models for the transaction domain.
package dto

import (
	"time"

	"go-espn-api/shared/drf"
)

// TransactionRow is a transaction joined with league, sport and (nullable) team.
type TransactionRow struct {
	ID               int64      `db:"id"`
	ESPNID           string     `db:"espn_id"`
	Date             *time.Time `db:"date"`
	Description      string     `db:"description"`
	Type             string     `db:"type"`
	AthleteName      string     `db:"athlete_name"`
	AthleteESPNID    string     `db:"athlete_espn_id"`
	LeagueSlug       string     `db:"league_slug"`
	SportSlug        string     `db:"sport_slug"`
	TeamAbbreviation *string    `db:"team_abbreviation"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// TransactionResponse matches DRF TransactionSerializer.
type TransactionResponse struct {
	ID               int64        `json:"id"`
	ESPNID           string       `json:"espn_id"`
	Date             drf.Date     `json:"date"`
	Description      string       `json:"description"`
	Type             string       `json:"type"`
	AthleteName      string       `json:"athlete_name"`
	AthleteESPNID    string       `json:"athlete_espn_id"`
	LeagueSlug       string       `json:"league_slug"`
	SportSlug        string       `json:"sport_slug"`
	TeamAbbreviation *string      `json:"team_abbreviation"`
	CreatedAt        drf.DateTime `json:"created_at"`
	UpdatedAt        drf.DateTime `json:"updated_at"`
}

// NewTransactionResponse builds the serializer output from a joined row.
func NewTransactionResponse(r TransactionRow) TransactionResponse {
	return TransactionResponse{
		ID:               r.ID,
		ESPNID:           r.ESPNID,
		Date:             drf.NewDate(r.Date),
		Description:      r.Description,
		Type:             r.Type,
		AthleteName:      r.AthleteName,
		AthleteESPNID:    r.AthleteESPNID,
		LeagueSlug:       r.LeagueSlug,
		SportSlug:        r.SportSlug,
		TeamAbbreviation: r.TeamAbbreviation,
		CreatedAt:        drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt:        drf.NewDateTimeValue(r.UpdatedAt),
	}
}
