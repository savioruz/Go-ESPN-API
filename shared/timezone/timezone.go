// Package timezone provides utility functions for handling time in UTC timezone.
package timezone

import (
	"time"
)

// NowUTC returns the current time in UTC timezone.
// This ensures consistency across the application regardless of server timezone configuration.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// ParseRFC3339 parses an RFC3339 formatted string and returns time in UTC.
// RFC3339 format: "2006-01-02T15:04:05Z07:00"
func ParseRFC3339(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}

	return t.UTC(), nil
}

// FormatRFC3339 formats a time.Time to RFC3339 string in UTC timezone.
func FormatRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ConvertToUTC converts any time.Time to UTC timezone.
// This is idempotent - calling it multiple times won't change the result.
func ConvertToUTC(t time.Time) time.Time {
	return t.UTC()
}

// BeginningOfDay returns the start of the day (00:00:00) in UTC for the given time.
func BeginningOfDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// EndOfDay returns the end of the day (23:59:59.999999999) in UTC for the given time.
func EndOfDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()

	return time.Date(year, month, day, 23, 59, 59, 999999999, time.UTC)
}

// IsExpired checks if the given time has passed relative to current UTC time.
func IsExpired(t time.Time) bool {
	return t.Before(NowUTC())
}

// TimeUntil returns the duration until the specified time from now (UTC).
// Returns 0 if the time has already passed.
func TimeUntil(t time.Time) time.Duration {
	duration := t.Sub(NowUTC())
	if duration < 0 {
		return 0
	}

	return duration
}

// TimeSince returns the duration since the specified time from now (UTC).
// Returns 0 if the time is in the future.
func TimeSince(t time.Time) time.Duration {
	duration := NowUTC().Sub(t)
	if duration < 0 {
		return 0
	}

	return duration
}

// AddDuration adds a duration to a time and ensures the result is in UTC.
func AddDuration(t time.Time, d time.Duration) time.Time {
	return t.Add(d).UTC()
}

// SubDuration subtracts a duration from a time and ensures the result is in UTC.
func SubDuration(t time.Time, d time.Duration) time.Time {
	return t.Add(-d).UTC()
}
