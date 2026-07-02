package timezone_test

import (
	"go-espn-api/shared/timezone"
	"testing"
	"time"
)

func TestNowUTC(t *testing.T) {
	now := timezone.NowUTC()

	if now.Location() != time.UTC {
		t.Errorf("expected timezone to be UTC, got %s", now.Location())
	}

	// Verify it's actually recent (within 1 second)
	if time.Since(now) > time.Second {
		t.Errorf("expected NowUTC to return current time, got %v", now)
	}
}

func TestParseRFC3339(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(time.Time) bool
	}{
		{
			name:        "valid UTC time",
			input:       "2023-01-15T10:30:00Z",
			expectError: false,
			validate: func(t time.Time) bool {
				return t.Year() == 2023 && t.Month() == 1 && t.Day() == 15 &&
					t.Hour() == 10 && t.Minute() == 30 && t.Second() == 0 &&
					t.Location() == time.UTC
			},
		},
		{
			name:        "valid time with positive offset",
			input:       "2023-01-15T15:30:00+05:00",
			expectError: false,
			validate: func(t time.Time) bool {
				// Should be converted to UTC (15:30+05:00 = 10:30 UTC)
				return t.Hour() == 10 && t.Minute() == 30 && t.Location() == time.UTC
			},
		},
		{
			name:        "valid time with negative offset",
			input:       "2023-01-15T05:30:00-05:00",
			expectError: false,
			validate: func(t time.Time) bool {
				// Should be converted to UTC (05:30-05:00 = 10:30 UTC)
				return t.Hour() == 10 && t.Minute() == 30 && t.Location() == time.UTC
			},
		},
		{
			name:        "invalid format",
			input:       "2023-01-15 10:30:00",
			expectError: true,
			validate:    func(t time.Time) bool { return true },
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			validate:    func(t time.Time) bool { return true },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := timezone.ParseRFC3339(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.validate(result) {
				t.Errorf("validation failed for parsed time: %v", result)
			}
		})
	}
}

func TestFormatRFC3339(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "UTC time",
			input:    time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: "2023-01-15T10:30:00Z",
		},
		{
			name:     "time with non-UTC timezone should be converted to UTC",
			input:    time.Date(2023, 1, 15, 15, 30, 0, 0, time.FixedZone("EST", -5*3600)),
			expected: "2023-01-15T20:30:00Z", // 15:30 EST = 20:30 UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timezone.FormatRFC3339(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestConvertToUTC(t *testing.T) {
	// Create time in different timezone
	est := time.FixedZone("EST", -5*3600)
	localTime := time.Date(2023, 1, 15, 10, 30, 0, 0, est)

	utcTime := timezone.ConvertToUTC(localTime)

	if utcTime.Location() != time.UTC {
		t.Errorf("expected timezone to be UTC, got %s", utcTime.Location())
	}

	// Verify it's idempotent
	utcTime2 := timezone.ConvertToUTC(utcTime)
	if !utcTime.Equal(utcTime2) {
		t.Errorf("ConvertToUTC should be idempotent")
	}
}

func TestBeginningOfDay(t *testing.T) {
	input := time.Date(2023, 1, 15, 14, 30, 45, 123456789, time.UTC)
	result := timezone.BeginningOfDay(input)

	expected := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)

	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}

	if result.Location() != time.UTC {
		t.Errorf("expected timezone to be UTC, got %s", result.Location())
	}
}

func TestEndOfDay(t *testing.T) {
	input := time.Date(2023, 1, 15, 14, 30, 45, 123456789, time.UTC)
	result := timezone.EndOfDay(input)

	expected := time.Date(2023, 1, 15, 23, 59, 59, 999999999, time.UTC)

	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}

	if result.Location() != time.UTC {
		t.Errorf("expected timezone to be UTC, got %s", result.Location())
	}
}

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected bool
	}{
		{
			name:     "past time should be expired",
			input:    time.Now().UTC().Add(-1 * time.Hour),
			expected: true,
		},
		{
			name:     "future time should not be expired",
			input:    time.Now().UTC().Add(1 * time.Hour),
			expected: false,
		},
		{
			name:     "very recent past should be expired",
			input:    time.Now().UTC().Add(-1 * time.Millisecond),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timezone.IsExpired(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTimeUntil(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		validate func(time.Duration) bool
	}{
		{
			name:  "future time should return positive duration",
			input: time.Now().UTC().Add(1 * time.Hour),
			validate: func(d time.Duration) bool {
				return d > 59*time.Minute && d <= 61*time.Minute // Allow some tolerance
			},
		},
		{
			name:  "past time should return zero",
			input: time.Now().UTC().Add(-1 * time.Hour),
			validate: func(d time.Duration) bool {
				return d == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timezone.TimeUntil(tt.input)
			if !tt.validate(result) {
				t.Errorf("validation failed for duration: %v", result)
			}
		})
	}
}

func TestTimeSince(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		validate func(time.Duration) bool
	}{
		{
			name:  "past time should return positive duration",
			input: time.Now().UTC().Add(-1 * time.Hour),
			validate: func(d time.Duration) bool {
				return d > 59*time.Minute && d <= 61*time.Minute // Allow some tolerance
			},
		},
		{
			name:  "future time should return zero",
			input: time.Now().UTC().Add(1 * time.Hour),
			validate: func(d time.Duration) bool {
				return d == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timezone.TimeSince(tt.input)
			if !tt.validate(result) {
				t.Errorf("validation failed for duration: %v", result)
			}
		})
	}
}

func TestAddDuration(t *testing.T) {
	base := time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC)
	duration := 2 * time.Hour

	result := timezone.AddDuration(base, duration)

	expected := time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC)

	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}

	if result.Location() != time.UTC {
		t.Errorf("expected timezone to be UTC, got %s", result.Location())
	}
}

func TestSubDuration(t *testing.T) {
	base := time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC)
	duration := 2 * time.Hour

	result := timezone.SubDuration(base, duration)

	expected := time.Date(2023, 1, 15, 8, 0, 0, 0, time.UTC)

	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}

	if result.Location() != time.UTC {
		t.Errorf("expected timezone to be UTC, got %s", result.Location())
	}
}

// TestTimezoneConsistency ensures all functions consistently return UTC times
func TestTimezoneConsistency(t *testing.T) {
	// Create a time in a different timezone
	pst := time.FixedZone("PST", -8*3600)
	localTime := time.Date(2023, 1, 15, 10, 30, 0, 0, pst)

	t.Run("all functions return UTC", func(t *testing.T) {
		if timezone.ConvertToUTC(localTime).Location() != time.UTC {
			t.Error("ConvertToUTC did not return UTC")
		}
		if timezone.BeginningOfDay(localTime).Location() != time.UTC {
			t.Error("BeginningOfDay did not return UTC")
		}
		if timezone.EndOfDay(localTime).Location() != time.UTC {
			t.Error("EndOfDay did not return UTC")
		}
		if timezone.AddDuration(localTime, time.Hour).Location() != time.UTC {
			t.Error("AddDuration did not return UTC")
		}
		if timezone.SubDuration(localTime, time.Hour).Location() != time.UTC {
			t.Error("SubDuration did not return UTC")
		}
		if timezone.NowUTC().Location() != time.UTC {
			t.Error("NowUTC did not return UTC")
		}
	})
}
