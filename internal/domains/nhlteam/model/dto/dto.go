// Package dto contains request/response models for the NHL team domain.
package dto

import (
	"encoding/json"

	"go-espn-api/shared/drf"
)

// TeamColumns lists the nhl_teams columns selected for the DRF TeamSerializer.
// It is reused by every domain that nests a team (players, games, standings).
const TeamColumns = "id, team_id, abbreviation, name, full_name, franchise_id, is_active, raw_data"

// TeamRow holds the nhl_teams columns for the DRF TeamSerializer.
type TeamRow struct {
	ID           int64           `db:"id"`
	TeamID       string          `db:"team_id"`
	Abbreviation string          `db:"abbreviation"`
	Name         string          `db:"name"`
	FullName     string          `db:"full_name"`
	FranchiseID  string          `db:"franchise_id"`
	IsActive     bool            `db:"is_active"`
	RawData      json.RawMessage `db:"raw_data"`
}

// TeamResponse matches the DRF NHL TeamSerializer.
type TeamResponse struct {
	ID           int64           `json:"id"`
	TeamID       string          `json:"team_id"`
	Abbreviation string          `json:"abbreviation"`
	Name         string          `json:"name"`
	FullName     string          `json:"full_name"`
	FranchiseID  string          `json:"franchise_id"`
	IsActive     bool            `json:"is_active"`
	RawData      json.RawMessage `json:"raw_data"`
}

// NewTeamResponse builds a TeamResponse, guarding raw_data to {} when empty.
func NewTeamResponse(r TeamRow) TeamResponse {
	return TeamResponse{
		ID:           r.ID,
		TeamID:       r.TeamID,
		Abbreviation: r.Abbreviation,
		Name:         r.Name,
		FullName:     r.FullName,
		FranchiseID:  r.FranchiseID,
		IsActive:     r.IsActive,
		RawData:      drf.JSONBObject(r.RawData),
	}
}

// TeamMap indexes team responses by their primary key for batch hydration.
func TeamMap(rows []TeamRow) map[int64]TeamResponse {
	m := make(map[int64]TeamResponse, len(rows))
	for _, r := range rows {
		m[r.ID] = NewTeamResponse(r)
	}

	return m
}
