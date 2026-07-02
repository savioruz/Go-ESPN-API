// Package dto contains request/response models for the NHL game domain.
package dto

import (
	"time"

	teamdto "go-espn-api/internal/domains/nhlteam/model/dto"
	"go-espn-api/shared/drf"
)

// GameColumns lists the nhl_games columns selected for the DRF GameSerializer.
const GameColumns = "id, game_id, season, game_type, date, home_team_id, away_team_id, home_score, away_score, status"

// GameRow holds the nhl_games columns for the DRF GameSerializer.
type GameRow struct {
	ID         int64     `db:"id"`
	GameID     string    `db:"game_id"`
	Season     string    `db:"season"`
	GameType   int       `db:"game_type"`
	Date       time.Time `db:"date"`
	HomeTeamID int64     `db:"home_team_id"`
	AwayTeamID int64     `db:"away_team_id"`
	HomeScore  *int      `db:"home_score"`
	AwayScore  *int      `db:"away_score"`
	Status     string    `db:"status"`
}

// GameResponse matches the DRF NHL GameSerializer.
type GameResponse struct {
	ID        int64                `json:"id"`
	GameID    string               `json:"game_id"`
	Season    string               `json:"season"`
	GameType  int                  `json:"game_type"`
	Date      drf.DateTime         `json:"date"`
	HomeTeam  teamdto.TeamResponse `json:"home_team"`
	AwayTeam  teamdto.TeamResponse `json:"away_team"`
	HomeScore *int                 `json:"home_score"`
	AwayScore *int                 `json:"away_score"`
	Status    string               `json:"status"`
}

// NewGameResponse builds a GameResponse, resolving the home/away teams from the
// pre-fetched team map (batch hydration, no per-row query).
func NewGameResponse(r GameRow, teams map[int64]teamdto.TeamResponse) GameResponse {
	return GameResponse{
		ID:        r.ID,
		GameID:    r.GameID,
		Season:    r.Season,
		GameType:  r.GameType,
		Date:      drf.NewDateTimeValue(r.Date),
		HomeTeam:  teams[r.HomeTeamID],
		AwayTeam:  teams[r.AwayTeamID],
		HomeScore: r.HomeScore,
		AwayScore: r.AwayScore,
		Status:    r.Status,
	}
}

// TeamIDsOf collects the distinct home/away team ids referenced by rows.
func TeamIDsOf(rows []GameRow) []int64 {
	seen := map[int64]struct{}{}
	ids := []int64{}

	add := func(id int64) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	for _, r := range rows {
		add(r.HomeTeamID)
		add(r.AwayTeamID)
	}

	return ids
}
