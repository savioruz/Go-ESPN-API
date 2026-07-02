// Package dto contains request/response models for the NHL goalie-stats domain.
package dto

import (
	"encoding/json"

	playerdto "go-espn-api/internal/domains/nhlplayer/model/dto"
	"go-espn-api/shared/drf"
)

// GoalieStatsColumns lists the columns for the DRF GoalieSeasonStatsSerializer.
const GoalieStatsColumns = "id, player_id, season, games_played, wins, losses, save_pct, gaa, shutouts, raw_data"

// GoalieStatsRow holds the nhl_goalie_season_stats columns.
type GoalieStatsRow struct {
	ID          int64           `db:"id"`
	PlayerID    int64           `db:"player_id"`
	Season      string          `db:"season"`
	GamesPlayed int             `db:"games_played"`
	Wins        int             `db:"wins"`
	Losses      int             `db:"losses"`
	SavePct     *float64        `db:"save_pct"`
	GAA         *float64        `db:"gaa"`
	Shutouts    int             `db:"shutouts"`
	RawData     json.RawMessage `db:"raw_data"`
}

// GoalieStatsResponse matches the DRF NHL GoalieSeasonStatsSerializer. player
// (FK NOT NULL) nests the full PlayerSerializer, which itself nests current_team.
type GoalieStatsResponse struct {
	ID          int64                    `json:"id"`
	Player      playerdto.PlayerResponse `json:"player"`
	Season      string                   `json:"season"`
	GamesPlayed int                      `json:"games_played"`
	Wins        int                      `json:"wins"`
	Losses      int                      `json:"losses"`
	SavePct     *float64                 `json:"save_pct"`
	GAA         *float64                 `json:"gaa"`
	Shutouts    int                      `json:"shutouts"`
	RawData     json.RawMessage          `json:"raw_data"`
}

// NewGoalieStatsResponse builds a response, resolving player from the pre-fetched
// player map (batch hydration, no per-row query).
func NewGoalieStatsResponse(r GoalieStatsRow, players map[int64]playerdto.PlayerResponse) GoalieStatsResponse {
	return GoalieStatsResponse{
		ID:          r.ID,
		Player:      players[r.PlayerID],
		Season:      r.Season,
		GamesPlayed: r.GamesPlayed,
		Wins:        r.Wins,
		Losses:      r.Losses,
		SavePct:     r.SavePct,
		GAA:         r.GAA,
		Shutouts:    r.Shutouts,
		RawData:     drf.JSONBObject(r.RawData),
	}
}

// PlayerIDsOf collects the distinct player ids referenced by rows.
func PlayerIDsOf(rows []GoalieStatsRow) []int64 {
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
