// Package dto contains request/response models for the league domain.
package dto

import (
	"time"

	"go-espn-api/shared/drf"
)

// LeagueRow is a league joined with its parent sport.
type LeagueRow struct {
	ID             int64     `db:"id"`
	Slug           string    `db:"slug"`
	Name           string    `db:"name"`
	Abbreviation   string    `db:"abbreviation"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	SportID        int64     `db:"sport_id"`
	SportSlug      string    `db:"sport_slug"`
	SportName      string    `db:"sport_name"`
	SportCreatedAt time.Time `db:"sport_created_at"`
	SportUpdatedAt time.Time `db:"sport_updated_at"`
}

type sportNested struct {
	ID        int64        `json:"id"`
	Slug      string       `json:"slug"`
	Name      string       `json:"name"`
	CreatedAt drf.DateTime `json:"created_at"`
	UpdatedAt drf.DateTime `json:"updated_at"`
}

// LeagueResponse matches DRF LeagueSerializer (nested full SportSerializer).
type LeagueResponse struct {
	ID           int64        `json:"id"`
	Slug         string       `json:"slug"`
	Name         string       `json:"name"`
	Abbreviation string       `json:"abbreviation"`
	Sport        sportNested  `json:"sport"`
	CreatedAt    drf.DateTime `json:"created_at"`
	UpdatedAt    drf.DateTime `json:"updated_at"`
}

// NewLeagueResponse builds a LeagueResponse from a joined row.
func NewLeagueResponse(r LeagueRow) LeagueResponse {
	return LeagueResponse{
		ID:           r.ID,
		Slug:         r.Slug,
		Name:         r.Name,
		Abbreviation: r.Abbreviation,
		Sport: sportNested{
			ID:        r.SportID,
			Slug:      r.SportSlug,
			Name:      r.SportName,
			CreatedAt: drf.NewDateTimeValue(r.SportCreatedAt),
			UpdatedAt: drf.NewDateTimeValue(r.SportUpdatedAt),
		},
		CreatedAt: drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt: drf.NewDateTimeValue(r.UpdatedAt),
	}
}
