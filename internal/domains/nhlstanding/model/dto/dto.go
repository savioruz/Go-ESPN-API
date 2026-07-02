// Package dto contains request/response models for the NHL standing domain.
package dto

import (
	"time"

	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/drf"
)

// StandingColumns lists the nhl_standings columns for the DRF StandingSerializer.
const StandingColumns = "id, team_id, date, games_played, wins, losses, ot_losses, points, point_pct, division_name, conference_name"

// StandingRow holds the nhl_standings columns for the DRF StandingSerializer.
type StandingRow struct {
	ID             int64     `db:"id"`
	TeamID         int64     `db:"team_id"`
	Date           time.Time `db:"date"`
	GamesPlayed    int       `db:"games_played"`
	Wins           int       `db:"wins"`
	Losses         int       `db:"losses"`
	OTLosses       int       `db:"ot_losses"`
	Points         int       `db:"points"`
	PointPct       *float64  `db:"point_pct"`
	DivisionName   string    `db:"division_name"`
	ConferenceName string    `db:"conference_name"`
}

// StandingResponse matches the DRF NHL StandingSerializer.
type StandingResponse struct {
	ID             int64                `json:"id"`
	Team           teamdto.TeamResponse `json:"team"`
	Date           drf.Date             `json:"date"`
	GamesPlayed    int                  `json:"games_played"`
	Wins           int                  `json:"wins"`
	Losses         int                  `json:"losses"`
	OTLosses       int                  `json:"ot_losses"`
	Points         int                  `json:"points"`
	PointPct       *float64             `json:"point_pct"`
	DivisionName   string               `json:"division_name"`
	ConferenceName string               `json:"conference_name"`
}

// NewStandingResponse builds a StandingResponse, resolving team from the
// pre-fetched team map (batch hydration, no per-row query).
func NewStandingResponse(r StandingRow, teams map[int64]teamdto.TeamResponse) StandingResponse {
	d := r.Date

	return StandingResponse{
		ID:             r.ID,
		Team:           teams[r.TeamID],
		Date:           drf.NewDate(&d),
		GamesPlayed:    r.GamesPlayed,
		Wins:           r.Wins,
		Losses:         r.Losses,
		OTLosses:       r.OTLosses,
		Points:         r.Points,
		PointPct:       r.PointPct,
		DivisionName:   r.DivisionName,
		ConferenceName: r.ConferenceName,
	}
}

// TeamIDsOf collects the distinct team ids referenced by rows.
func TeamIDsOf(rows []StandingRow) []int64 {
	seen := map[int64]struct{}{}
	ids := []int64{}

	for _, r := range rows {
		if _, ok := seen[r.TeamID]; !ok {
			seen[r.TeamID] = struct{}{}
			ids = append(ids, r.TeamID)
		}
	}

	return ids
}
