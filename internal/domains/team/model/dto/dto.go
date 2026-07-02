// Package dto contains request/response models for the team domain.
package dto

import (
	"encoding/json"
	"time"

	"go-espn-api/shared/drf"
)

// ComputePrimaryLogo ports Team.primary_logo from the Django models. It parses
// the logos JSONB array and returns the href of the first logo whose rel array
// contains "default"; otherwise the first logo's href; otherwise nil.
func ComputePrimaryLogo(logos json.RawMessage) *string {
	if len(logos) == 0 {
		return nil
	}

	var arr []map[string]any
	if err := json.Unmarshal(logos, &arr); err != nil || len(arr) == 0 {
		return nil
	}

	for _, logo := range arr {
		rel, ok := logo["rel"].([]any)
		if !ok {
			continue
		}

		for _, r := range rel {
			if s, ok := r.(string); ok && s == "default" {
				return hrefOf(logo)
			}
		}
	}

	return hrefOf(arr[0])
}

// hrefOf returns the "href" string of a logo object, or nil when missing/non-string.
func hrefOf(logo map[string]any) *string {
	if href, ok := logo["href"].(string); ok {
		out := href

		return &out
	}

	return nil
}

// LeagueMinimalRow holds the columns for LeagueMinimalSerializer.
type LeagueMinimalRow struct {
	LeagueID           int64  `db:"league_id"`
	LeagueSlug         string `db:"league_slug"`
	LeagueName         string `db:"league_name"`
	LeagueAbbreviation string `db:"league_abbreviation"`
	SportSlug          string `db:"sport_slug"`
}

// LeagueMinimalResponse matches DRF LeagueMinimalSerializer.
type LeagueMinimalResponse struct {
	ID           int64  `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	SportSlug    string `json:"sport_slug"`
}

// NewLeagueMinimalResponse builds a LeagueMinimalResponse from row columns.
func NewLeagueMinimalResponse(r LeagueMinimalRow) LeagueMinimalResponse {
	return LeagueMinimalResponse{
		ID:           r.LeagueID,
		Slug:         r.LeagueSlug,
		Name:         r.LeagueName,
		Abbreviation: r.LeagueAbbreviation,
		SportSlug:    r.SportSlug,
	}
}

// TeamMinimalResponse matches DRF TeamMinimalSerializer.
type TeamMinimalResponse struct {
	ID               int64   `json:"id"`
	ESPNID           string  `json:"espn_id"`
	Abbreviation     string  `json:"abbreviation"`
	DisplayName      string  `json:"display_name"`
	ShortDisplayName string  `json:"short_display_name"`
	Location         string  `json:"location"`
	Color            string  `json:"color"`
	PrimaryLogo      *string `json:"primary_logo"`
}

// NewTeamMinimalResponse builds a TeamMinimalResponse.
func NewTeamMinimalResponse(id int64, espnID, abbr, displayName, shortDisplayName, location, color string, logos json.RawMessage) TeamMinimalResponse {
	return TeamMinimalResponse{
		ID:               id,
		ESPNID:           espnID,
		Abbreviation:     abbr,
		DisplayName:      displayName,
		ShortDisplayName: shortDisplayName,
		Location:         location,
		Color:            color,
		PrimaryLogo:      ComputePrimaryLogo(logos),
	}
}

// TeamListRow is a team joined with its league and sport slugs (list endpoint).
type TeamListRow struct {
	ID               int64           `db:"id"`
	ESPNID           string          `db:"espn_id"`
	Abbreviation     string          `db:"abbreviation"`
	DisplayName      string          `db:"display_name"`
	ShortDisplayName string          `db:"short_display_name"`
	Location         string          `db:"location"`
	Color            string          `db:"color"`
	Logos            json.RawMessage `db:"logos"`
	LeagueSlug       string          `db:"league_slug"`
	SportSlug        string          `db:"sport_slug"`
	IsActive         bool            `db:"is_active"`
}

// TeamListResponse matches DRF TeamListSerializer.
type TeamListResponse struct {
	ID               int64   `json:"id"`
	ESPNID           string  `json:"espn_id"`
	Abbreviation     string  `json:"abbreviation"`
	DisplayName      string  `json:"display_name"`
	ShortDisplayName string  `json:"short_display_name"`
	Location         string  `json:"location"`
	Color            string  `json:"color"`
	PrimaryLogo      *string `json:"primary_logo"`
	LeagueSlug       string  `json:"league_slug"`
	SportSlug        string  `json:"sport_slug"`
	IsActive         bool    `json:"is_active"`
}

// NewTeamListResponse builds the list serializer output.
func NewTeamListResponse(r TeamListRow) TeamListResponse {
	return TeamListResponse{
		ID:               r.ID,
		ESPNID:           r.ESPNID,
		Abbreviation:     r.Abbreviation,
		DisplayName:      r.DisplayName,
		ShortDisplayName: r.ShortDisplayName,
		Location:         r.Location,
		Color:            r.Color,
		PrimaryLogo:      ComputePrimaryLogo(r.Logos),
		LeagueSlug:       r.LeagueSlug,
		SportSlug:        r.SportSlug,
		IsActive:         r.IsActive,
	}
}

// TeamDetailRow is a team joined with its minimal league (detail endpoint).
type TeamDetailRow struct {
	ID               int64           `db:"id"`
	ESPNID           string          `db:"espn_id"`
	UID              string          `db:"uid"`
	Slug             string          `db:"slug"`
	Abbreviation     string          `db:"abbreviation"`
	DisplayName      string          `db:"display_name"`
	ShortDisplayName string          `db:"short_display_name"`
	Name             string          `db:"name"`
	Nickname         string          `db:"nickname"`
	Location         string          `db:"location"`
	Color            string          `db:"color"`
	AlternateColor   string          `db:"alternate_color"`
	IsActive         bool            `db:"is_active"`
	IsAllStar        bool            `db:"is_all_star"`
	Logos            json.RawMessage `db:"logos"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
	LeagueMinimalRow
}

// TeamDetailResponse matches DRF TeamSerializer.
type TeamDetailResponse struct {
	ID               int64                 `json:"id"`
	ESPNID           string                `json:"espn_id"`
	UID              string                `json:"uid"`
	Slug             string                `json:"slug"`
	Abbreviation     string                `json:"abbreviation"`
	DisplayName      string                `json:"display_name"`
	ShortDisplayName string                `json:"short_display_name"`
	Name             string                `json:"name"`
	Nickname         string                `json:"nickname"`
	Location         string                `json:"location"`
	Color            string                `json:"color"`
	AlternateColor   string                `json:"alternate_color"`
	IsActive         bool                  `json:"is_active"`
	IsAllStar        bool                  `json:"is_all_star"`
	Logos            json.RawMessage       `json:"logos"`
	PrimaryLogo      *string               `json:"primary_logo"`
	League           LeagueMinimalResponse `json:"league"`
	CreatedAt        drf.DateTime          `json:"created_at"`
	UpdatedAt        drf.DateTime          `json:"updated_at"`
}

// NewTeamDetailResponse builds the detail serializer output.
func NewTeamDetailResponse(r TeamDetailRow) TeamDetailResponse {
	return TeamDetailResponse{
		ID:               r.ID,
		ESPNID:           r.ESPNID,
		UID:              r.UID,
		Slug:             r.Slug,
		Abbreviation:     r.Abbreviation,
		DisplayName:      r.DisplayName,
		ShortDisplayName: r.ShortDisplayName,
		Name:             r.Name,
		Nickname:         r.Nickname,
		Location:         r.Location,
		Color:            r.Color,
		AlternateColor:   r.AlternateColor,
		IsActive:         r.IsActive,
		IsAllStar:        r.IsAllStar,
		Logos:            drf.JSONBArray(r.Logos),
		PrimaryLogo:      ComputePrimaryLogo(r.Logos),
		League:           NewLeagueMinimalResponse(r.LeagueMinimalRow),
		CreatedAt:        drf.NewDateTimeValue(r.CreatedAt),
		UpdatedAt:        drf.NewDateTimeValue(r.UpdatedAt),
	}
}
