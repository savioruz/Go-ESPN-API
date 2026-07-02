// Package drf provides helpers for producing output byte-compatible with the
// upstream Django REST Framework service (datetime formatting, ordering
// whitelisting, and JSONB pass-through guards).
package drf

import (
	"encoding/json"
	"strings"
	"time"
)

// DateTime marshals a nullable timestamp using DRF's ISO-8601 representation.
//
// DRF calls value.isoformat() and rewrites a trailing "+00:00" to "Z". Values
// are rendered in UTC. Fractional seconds are emitted with 6 digits (matching
// Python's microsecond precision) only when non-zero.
type DateTime struct {
	T *time.Time
}

// NewDateTime wraps a nullable timestamp pointer.
func NewDateTime(t *time.Time) DateTime {
	return DateTime{T: t}
}

// NewDateTimeValue wraps a non-null timestamp value.
func NewDateTimeValue(t time.Time) DateTime {
	return DateTime{T: &t}
}

// MarshalJSON implements json.Marshaler.
func (d DateTime) MarshalJSON() ([]byte, error) {
	if d.T == nil {
		return []byte("null"), nil
	}

	t := d.T.UTC()

	var s string
	if t.Nanosecond() == 0 {
		s = t.Format("2006-01-02T15:04:05") + "Z"
	} else {
		s = t.Format("2006-01-02T15:04:05.000000") + "Z"
	}

	return []byte(`"` + s + `"`), nil
}

// Date marshals a nullable Django DateField as "YYYY-MM-DD" or null.
type Date struct {
	T *time.Time
}

// NewDate wraps a nullable date pointer.
func NewDate(t *time.Time) Date {
	return Date{T: t}
}

// MarshalJSON implements json.Marshaler.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.T == nil {
		return []byte("null"), nil
	}

	return []byte(`"` + d.T.Format("2006-01-02") + `"`), nil
}

// ResolveOrdering parses an ?ordering value ("field" or "-field") against a
// whitelist mapping allowed field names to fully-qualified SQL column
// expressions. It returns an ORDER BY body (without the "ORDER BY" keyword).
// Unknown or empty values fall back to defaultClause.
func ResolveOrdering(ordering string, allowed map[string]string, defaultClause string) string {
	if ordering == "" {
		return defaultClause
	}

	field := ordering
	desc := false

	if strings.HasPrefix(field, "-") {
		desc = true
		field = field[1:]
	}

	col, ok := allowed[field]
	if !ok {
		return defaultClause
	}

	if desc {
		return col + " DESC"
	}

	return col + " ASC"
}

// JSONBArray returns raw as-is, defaulting to an empty array when empty/null.
func JSONBArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}

	return raw
}

// JSONBObject returns raw as-is, defaulting to an empty object when empty/null.
func JSONBObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}

	return raw
}
