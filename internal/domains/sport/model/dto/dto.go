// Package dto contains request/response models for the sport domain.
package dto

import (
	"go-espn-api/internal/domains/sport/model"
	"go-espn-api/shared/drf"
)

// SportResponse matches DRF SportSerializer.
type SportResponse struct {
	ID        int64        `json:"id"`
	Slug      string       `json:"slug"`
	Name      string       `json:"name"`
	CreatedAt drf.DateTime `json:"created_at"`
	UpdatedAt drf.DateTime `json:"updated_at"`
}

// NewSportResponse builds a SportResponse from a sport model.
func NewSportResponse(m model.Sport) SportResponse {
	return SportResponse{
		ID:        m.ID,
		Slug:      m.Slug,
		Name:      m.Name,
		CreatedAt: drf.NewDateTimeValue(m.CreatedAt),
		UpdatedAt: drf.NewDateTimeValue(m.UpdatedAt),
	}
}
