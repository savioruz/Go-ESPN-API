// Package dto contains request/response models for the event domain.
package dto

import (
	"encoding/json"
	"strconv"
	"time"

	teamdto "go-espn-api/internal/domains/team/model/dto"
	"go-espn-api/shared/drf"
)

// ComputeScoreInt ports Competitor.score_int: int(score) or nil when empty /
// non-numeric.
func ComputeScoreInt(score string) *int {
	if score == "" {
		return nil
	}

	n, err := strconv.Atoi(score)
	if err != nil {
		return nil
	}

	return &n
}

// CompetitorRow is a competitor joined with its minimal team, tagged with the
// owning event id for in-Go grouping.
type CompetitorRow struct {
	EventID              int64           `db:"event_id"`
	ID                   int64           `db:"id"`
	HomeAway             string          `db:"home_away"`
	Score                string          `db:"score"`
	Winner               *bool           `db:"winner"`
	LineScores           json.RawMessage `db:"line_scores"`
	Records              json.RawMessage `db:"records"`
	Statistics           json.RawMessage `db:"statistics"`
	Leaders              json.RawMessage `db:"leaders"`
	Order                int             `db:"order"`
	TeamID               int64           `db:"team_id"`
	TeamESPNID           string          `db:"team_espn_id"`
	TeamAbbreviation     string          `db:"team_abbreviation"`
	TeamDisplayName      string          `db:"team_display_name"`
	TeamShortDisplayName string          `db:"team_short_display_name"`
	TeamLocation         string          `db:"team_location"`
	TeamColor            string          `db:"team_color"`
	TeamLogos            json.RawMessage `db:"team_logos"`
}

// CompetitorResponse matches DRF CompetitorSerializer.
type CompetitorResponse struct {
	ID         int64                       `json:"id"`
	Team       teamdto.TeamMinimalResponse `json:"team"`
	HomeAway   string                      `json:"home_away"`
	Score      string                      `json:"score"`
	ScoreInt   *int                        `json:"score_int"`
	Winner     *bool                       `json:"winner"`
	LineScores json.RawMessage             `json:"line_scores"`
	Records    json.RawMessage             `json:"records"`
	Statistics json.RawMessage             `json:"statistics"`
	Leaders    json.RawMessage             `json:"leaders"`
	Order      int                         `json:"order"`
}

// NewCompetitorResponse builds a CompetitorResponse from a joined row.
func NewCompetitorResponse(r CompetitorRow) CompetitorResponse {
	return CompetitorResponse{
		ID: r.ID,
		Team: teamdto.NewTeamMinimalResponse(
			r.TeamID, r.TeamESPNID, r.TeamAbbreviation, r.TeamDisplayName,
			r.TeamShortDisplayName, r.TeamLocation, r.TeamColor, r.TeamLogos,
		),
		HomeAway:   r.HomeAway,
		Score:      r.Score,
		ScoreInt:   ComputeScoreInt(r.Score),
		Winner:     r.Winner,
		LineScores: drf.JSONBArray(r.LineScores),
		Records:    drf.JSONBArray(r.Records),
		Statistics: drf.JSONBArray(r.Statistics),
		Leaders:    drf.JSONBArray(r.Leaders),
		Order:      r.Order,
	}
}

// GroupCompetitors groups competitor rows by event id, preserving the query
// order (event_id, "order" ASC). Each event id maps to a non-nil slice so that
// events with competitors always serialise as a JSON array.
func GroupCompetitors(rows []CompetitorRow) map[int64][]CompetitorResponse {
	out := make(map[int64][]CompetitorResponse)
	for _, r := range rows {
		out[r.EventID] = append(out[r.EventID], NewCompetitorResponse(r))
	}

	return out
}

// EventListRow is an event joined with league/sport slugs and venue name.
type EventListRow struct {
	ID           int64     `db:"id"`
	ESPNID       string    `db:"espn_id"`
	Date         time.Time `db:"date"`
	Name         string    `db:"name"`
	ShortName    string    `db:"short_name"`
	Status       string    `db:"status"`
	StatusDetail string    `db:"status_detail"`
	LeagueSlug   string    `db:"league_slug"`
	SportSlug    string    `db:"sport_slug"`
	VenueName    *string   `db:"venue_name"`
}

// EventListResponse matches DRF EventListSerializer.
type EventListResponse struct {
	ID           int64                `json:"id"`
	ESPNID       string               `json:"espn_id"`
	Date         drf.DateTime         `json:"date"`
	Name         string               `json:"name"`
	ShortName    string               `json:"short_name"`
	Status       string               `json:"status"`
	StatusDetail string               `json:"status_detail"`
	LeagueSlug   string               `json:"league_slug"`
	SportSlug    string               `json:"sport_slug"`
	VenueName    *string              `json:"venue_name"`
	Competitors  []CompetitorResponse `json:"competitors"`
}

// NewEventListResponse builds the list serializer output with its competitors.
func NewEventListResponse(r EventListRow, competitors []CompetitorResponse) EventListResponse {
	if competitors == nil {
		competitors = []CompetitorResponse{}
	}

	return EventListResponse{
		ID:           r.ID,
		ESPNID:       r.ESPNID,
		Date:         drf.NewDateTimeValue(r.Date),
		Name:         r.Name,
		ShortName:    r.ShortName,
		Status:       r.Status,
		StatusDetail: r.StatusDetail,
		LeagueSlug:   r.LeagueSlug,
		SportSlug:    r.SportSlug,
		VenueName:    r.VenueName,
		Competitors:  competitors,
	}
}

// VenueResponse matches DRF VenueSerializer.
type VenueResponse struct {
	ID        int64        `json:"id"`
	ESPNID    string       `json:"espn_id"`
	Name      string       `json:"name"`
	City      string       `json:"city"`
	State     string       `json:"state"`
	Country   string       `json:"country"`
	IsIndoor  bool         `json:"is_indoor"`
	Capacity  *int         `json:"capacity"`
	CreatedAt drf.DateTime `json:"created_at"`
	UpdatedAt drf.DateTime `json:"updated_at"`
}

// EventDetailRow is an event joined with its minimal league and nullable venue.
type EventDetailRow struct {
	ID           int64           `db:"id"`
	ESPNID       string          `db:"espn_id"`
	UID          string          `db:"uid"`
	Date         time.Time       `db:"date"`
	Name         string          `db:"name"`
	ShortName    string          `db:"short_name"`
	SeasonYear   int             `db:"season_year"`
	SeasonType   int             `db:"season_type"`
	SeasonSlug   string          `db:"season_slug"`
	Week         *int            `db:"week"`
	Status       string          `db:"status"`
	StatusDetail string          `db:"status_detail"`
	Clock        string          `db:"clock"`
	Period       *int            `db:"period"`
	Attendance   *int            `db:"attendance"`
	Broadcasts   json.RawMessage `db:"broadcasts"`
	Links        json.RawMessage `db:"links"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`

	teamdto.LeagueMinimalRow

	VenueID        *int64     `db:"venue_id"`
	VenueESPNID    *string    `db:"venue_espn_id"`
	VenueName      *string    `db:"venue_name"`
	VenueCity      *string    `db:"venue_city"`
	VenueState     *string    `db:"venue_state"`
	VenueCountry   *string    `db:"venue_country"`
	VenueIsIndoor  *bool      `db:"venue_is_indoor"`
	VenueCapacity  *int       `db:"venue_capacity"`
	VenueCreatedAt *time.Time `db:"venue_created_at"`
	VenueUpdatedAt *time.Time `db:"venue_updated_at"`
}

// EventDetailResponse matches DRF EventSerializer.
type EventDetailResponse struct {
	ID           int64                         `json:"id"`
	ESPNID       string                        `json:"espn_id"`
	UID          string                        `json:"uid"`
	Date         drf.DateTime                  `json:"date"`
	Name         string                        `json:"name"`
	ShortName    string                        `json:"short_name"`
	SeasonYear   int                           `json:"season_year"`
	SeasonType   int                           `json:"season_type"`
	SeasonSlug   string                        `json:"season_slug"`
	Week         *int                          `json:"week"`
	Status       string                        `json:"status"`
	StatusDetail string                        `json:"status_detail"`
	Clock        string                        `json:"clock"`
	Period       *int                          `json:"period"`
	Attendance   *int                          `json:"attendance"`
	Broadcasts   json.RawMessage               `json:"broadcasts"`
	Links        json.RawMessage               `json:"links"`
	League       teamdto.LeagueMinimalResponse `json:"league"`
	Venue        *VenueResponse                `json:"venue"`
	Competitors  []CompetitorResponse          `json:"competitors"`
	CreatedAt    drf.DateTime                  `json:"created_at"`
	UpdatedAt    drf.DateTime                  `json:"updated_at"`
}

// NewEventDetailResponse builds the detail serializer output with competitors.
func NewEventDetailResponse(r EventDetailRow, competitors []CompetitorResponse) EventDetailResponse {
	if competitors == nil {
		competitors = []CompetitorResponse{}
	}

	var venue *VenueResponse
	if r.VenueID != nil {
		venue = &VenueResponse{
			ID:        *r.VenueID,
			ESPNID:    derefString(r.VenueESPNID),
			Name:      derefString(r.VenueName),
			City:      derefString(r.VenueCity),
			State:     derefString(r.VenueState),
			Country:   derefString(r.VenueCountry),
			IsIndoor:  r.VenueIsIndoor != nil && *r.VenueIsIndoor,
			Capacity:  r.VenueCapacity,
			CreatedAt: drf.NewDateTime(r.VenueCreatedAt),
			UpdatedAt: drf.NewDateTime(r.VenueUpdatedAt),
		}
	}

	return EventDetailResponse{
		ID:           r.ID,
		ESPNID:       r.ESPNID,
		UID:          r.UID,
		Date:         drf.NewDateTimeValue(r.Date),
		Name:         r.Name,
		ShortName:    r.ShortName,
		SeasonYear:   r.SeasonYear,
		SeasonType:   r.SeasonType,
		SeasonSlug:   r.SeasonSlug,
		Week:         r.Week,
		Status:       r.Status,
		StatusDetail: r.StatusDetail,
		Clock:        r.Clock,
		Period:       r.Period,
		Attendance:   r.Attendance,
		Broadcasts:   drf.JSONBArray(r.Broadcasts),
		Links:        drf.JSONBArray(r.Links),
		League:       teamdto.NewLeagueMinimalResponse(r.LeagueMinimalRow),
		Venue:        venue,
		Competitors:  competitors,
		CreatedAt:    drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt:    drf.NewDateTimeValue(r.UpdatedAt),
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
