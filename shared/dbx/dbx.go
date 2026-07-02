// Package dbx provides small helpers for named-parameter reads used by the
// custom join repositories.
package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go-espn-api/infras/postgres"
	"go-espn-api/shared/logger"

	"github.com/jmoiron/sqlx"
)

// NamedGet runs a single-row named query. It returns sql.ErrNoRows unwrapped so
// callers can detect "not found".
func NamedGet(ctx context.Context, db *postgres.Connection, query string, args map[string]any, dest any) error {
	stmt, err := db.Read.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)

		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func(s *sqlx.NamedStmt) { _ = s.Close() }(stmt)

	if err := stmt.GetContext(ctx, dest, args); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}

		logger.ErrorWithStack(err)

		return fmt.Errorf("get: %w", err)
	}

	return nil
}

// JSONOrDefault returns raw JSON as a string so lib/pq binds it to a JSONB
// column as text (a raw []byte would be sent as bytea and rejected). It falls
// back to def (e.g. "{}" or "[]") when the payload is empty.
func JSONOrDefault(raw []byte, def string) string {
	if len(raw) == 0 {
		return def
	}

	return string(raw)
}

// NamedPreparer is the subset of database handles able to prepare a named
// statement. It is satisfied by both *sqlx.DB and *sqlx.Tx, allowing the ingest
// write helpers to run against a plain connection or inside a transaction.
type NamedPreparer interface {
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
}

// NamedGetP runs a single-row named query against the given preparer (DB or Tx).
// It returns sql.ErrNoRows unwrapped so callers can detect "not found". arg may
// be a struct or a map.
func NamedGetP(ctx context.Context, p NamedPreparer, query string, arg any, dest any) error {
	stmt, err := p.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)

		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func(s *sqlx.NamedStmt) { _ = s.Close() }(stmt)

	if err := stmt.GetContext(ctx, dest, arg); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}

		logger.ErrorWithStack(err)

		return fmt.Errorf("get: %w", err)
	}

	return nil
}

// NamedExecP runs a mutating named query against the given preparer (DB or Tx).
// arg may be a struct or a map.
func NamedExecP(ctx context.Context, p NamedPreparer, query string, arg any) error {
	stmt, err := p.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)

		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func(s *sqlx.NamedStmt) { _ = s.Close() }(stmt)

	if _, err := stmt.ExecContext(ctx, arg); err != nil {
		logger.ErrorWithStack(err)

		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

// NamedSelect runs a multi-row named query into a slice destination.
func NamedSelect(ctx context.Context, db *postgres.Connection, query string, args map[string]any, dest any) error {
	stmt, err := db.Read.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)

		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func(s *sqlx.NamedStmt) { _ = s.Close() }(stmt)

	if err := stmt.SelectContext(ctx, dest, args); err != nil {
		logger.ErrorWithStack(err)

		return fmt.Errorf("select: %w", err)
	}

	return nil
}
