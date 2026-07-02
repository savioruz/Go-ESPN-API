// Package model contains the common data models used across the application,
// including Metadata which captures the creation and modification details of entities.
package model

import "time"

// Metadata represents the common metadata fields for all entities.
type Metadata struct {
	CreatedAt  time.Time `db:"created_at"`
	ModifiedAt time.Time `db:"modified_at"`
	CreatedBy  string    `db:"created_by"`
	ModifiedBy string    `db:"modified_by"`
}
