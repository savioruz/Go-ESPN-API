// Package ingest ports the ESPN write-side ingestion services from the Python
// espn_service. Each service fetches from the ESPN client and upserts into
// Postgres within a single transaction, mirroring Django's @transaction.atomic.
package ingest

import (
	"go-espn-api/infras/postgres"

	"github.com/jmoiron/sqlx"
)

// IngestionResult is the outcome of an ingestion operation. It mirrors the
// Python dataclass of the same name.
type IngestionResult struct {
	Created int
	Updated int
	Errors  int
	Details []string
}

// newResult returns a result with a non-nil (but empty) Details slice so it
// serialises as [] rather than null, matching the Python default.
func newResult() IngestionResult {
	return IngestionResult{Details: []string{}}
}

// TotalProcessed returns created + updated.
func (r IngestionResult) TotalProcessed() int {
	return r.Created + r.Updated
}

// withTx runs fn inside a write transaction, committing on success and rolling
// back on error. It mirrors Django's @transaction.atomic wrapping of an ingest
// run: any returned error aborts and rolls back the whole operation.
func withTx(db *postgres.Connection, fn func(tx *sqlx.Tx) error) error {
	tx, err := db.Write.Beginx()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()

		return err
	}

	return tx.Commit()
}
