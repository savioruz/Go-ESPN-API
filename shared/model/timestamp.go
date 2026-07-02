package model

import "time"

// Timestamp represents the common created_at / updated_at fields for
// machine-ingested entities that have no authenticated author.
type Timestamp struct {
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
