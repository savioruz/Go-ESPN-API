package dto

import (
	"go-espn-api/shared/model"
	"go-espn-api/shared/timezone"
)

// Metadata represents the common metadata fields for all entities in the DTO layer.
type Metadata struct {
	CreatedAt  string `json:"created_at"`
	ModifiedAt string `json:"modified_at"`
	CreatedBy  string `json:"created_by"`
	ModifiedBy string `json:"modified_by"`
}

// FromModel populates the Metadata DTO from the given model.Metadata.
func (m *Metadata) FromModel(model model.Metadata) {
	m.CreatedAt = timezone.FormatRFC3339(model.CreatedAt)
	m.ModifiedAt = timezone.FormatRFC3339(model.ModifiedAt)
	m.CreatedBy = model.CreatedBy
	m.ModifiedBy = model.ModifiedBy
}
