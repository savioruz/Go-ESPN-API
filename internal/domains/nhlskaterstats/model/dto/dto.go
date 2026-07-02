// Package dto contains request/response models for the NHL skater-stats domain.
package dto

import (
	"encoding/json"

	playerdto "go-espn-api/internal/domains/nhlplayer/model/dto"
	"go-espn-api/shared/drf"
)

// SkaterStatsColumns lists the columns for the DRF SkaterSeasonStatsSerializer.
const SkaterStatsColumns = "id, player_id, season, games_played, goals, assists, points, plus_minus, raw_data"

// SkaterStatsRow holds the nhl_skater_season_stats columns.
type SkaterStatsRow struct {
	ID          int64           `db:"id"`
	PlayerID    int64           `db:"player_id"`
	Season      string          `db:"season"`
	GamesPlayed int             `db:"games_played"`
	Goals       int             `db:"goals"`
	Assists     int             `db:"assists"`
	Points      int             `db:"points"`
	PlusMinus   int             `db:"plus_minus"`
	RawData     json.RawMessage `db:"raw_data"`
}

// SkaterStatsResponse matches the DRF NHL SkaterSeasonStatsSerializer. player
// (FK NOT NULL) nests the full PlayerSerializer, which itself nests current_team.
type SkaterStatsResponse struct {
	ID          int64                    `json:"id"`
	Player      playerdto.PlayerResponse `json:"player"`
	Season      string                   `json:"season"`
	GamesPlayed int                      `json:"games_played"`
	Goals       int                      `json:"goals"`
	Assists     int                      `json:"assists"`
	Points      int                      `json:"points"`
	PlusMinus   int                      `json:"plus_minus"`
	RawData     json.RawMessage          `json:"raw_data"`
}

// NewSkaterStatsResponse builds a response, resolving player from the pre-fetched
// player map (batch hydration, no per-row query).
func NewSkaterStatsResponse(r SkaterStatsRow, players map[int64]playerdto.PlayerResponse) SkaterStatsResponse {
	return SkaterStatsResponse{
		ID:          r.ID,
		Player:      players[r.PlayerID],
		Season:      r.Season,
		GamesPlayed: r.GamesPlayed,
		Goals:       r.Goals,
		Assists:     r.Assists,
		Points:      r.Points,
		PlusMinus:   r.PlusMinus,
		RawData:     drf.JSONBObject(r.RawData),
	}
}

// PlayerIDsOf collects the distinct player ids referenced by rows.
func PlayerIDsOf(rows []SkaterStatsRow) []int64 {
	seen := map[int64]struct{}{}
	ids := []int64{}

	for _, r := range rows {
		if _, ok := seen[r.PlayerID]; !ok {
			seen[r.PlayerID] = struct{}{}
			ids = append(ids, r.PlayerID)
		}
	}

	return ids
}
