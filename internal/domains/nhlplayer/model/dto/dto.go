// Package dto contains request/response models for the NHL player domain.
package dto

import (
	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
)

// PlayerColumns lists the nhl_players columns selected for the DRF PlayerSerializer.
// Reused by the season-stats domains that nest a player.
const PlayerColumns = "id, player_id, first_name, last_name, full_name, sweater_number, position, current_team_id, is_active, headshot_url"

// PlayerRow holds the nhl_players columns for the DRF PlayerSerializer.
type PlayerRow struct {
	ID            int64  `db:"id"`
	PlayerID      string `db:"player_id"`
	FirstName     string `db:"first_name"`
	LastName      string `db:"last_name"`
	FullName      string `db:"full_name"`
	SweaterNumber string `db:"sweater_number"`
	Position      string `db:"position"`
	CurrentTeamID *int64 `db:"current_team_id"`
	IsActive      bool   `db:"is_active"`
	HeadshotURL   string `db:"headshot_url"`
}

// PlayerResponse matches the DRF NHL PlayerSerializer. current_team is null when
// the FK is null (SET_NULL) or the team is unavailable.
type PlayerResponse struct {
	ID            int64                 `json:"id"`
	PlayerID      string                `json:"player_id"`
	FirstName     string                `json:"first_name"`
	LastName      string                `json:"last_name"`
	FullName      string                `json:"full_name"`
	SweaterNumber string                `json:"sweater_number"`
	Position      string                `json:"position"`
	CurrentTeam   *teamdto.TeamResponse `json:"current_team"`
	IsActive      bool                  `json:"is_active"`
	HeadshotURL   string                `json:"headshot_url"`
}

// NewPlayerResponse builds a PlayerResponse, resolving current_team from the
// pre-fetched team map (batch hydration, no per-row query).
func NewPlayerResponse(r PlayerRow, teams map[int64]teamdto.TeamResponse) PlayerResponse {
	var team *teamdto.TeamResponse

	if r.CurrentTeamID != nil {
		if t, ok := teams[*r.CurrentTeamID]; ok {
			team = &t
		}
	}

	return PlayerResponse{
		ID:            r.ID,
		PlayerID:      r.PlayerID,
		FirstName:     r.FirstName,
		LastName:      r.LastName,
		FullName:      r.FullName,
		SweaterNumber: r.SweaterNumber,
		Position:      r.Position,
		CurrentTeam:   team,
		IsActive:      r.IsActive,
		HeadshotURL:   r.HeadshotURL,
	}
}

// PlayerMap indexes hydrated player responses by their primary key.
func PlayerMap(rows []PlayerRow, teams map[int64]teamdto.TeamResponse) map[int64]PlayerResponse {
	m := make(map[int64]PlayerResponse, len(rows))
	for _, r := range rows {
		m[r.ID] = NewPlayerResponse(r, teams)
	}

	return m
}

// TeamIDsOf collects the distinct non-null current_team_id values from rows.
func TeamIDsOf(rows []PlayerRow) []int64 {
	seen := map[int64]struct{}{}
	ids := []int64{}

	for _, r := range rows {
		if r.CurrentTeamID == nil {
			continue
		}

		if _, ok := seen[*r.CurrentTeamID]; !ok {
			seen[*r.CurrentTeamID] = struct{}{}
			ids = append(ids, *r.CurrentTeamID)
		}
	}

	return ids
}
