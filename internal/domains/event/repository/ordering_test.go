package repository

import (
	"testing"

	"go-espn-api/shared/drf"
)

// TestEventOrdering pins the ORDER BY resolution the List query relies on: the
// default is descending date (newest first) and ?ordering=date flips it to
// ascending so page 1 surfaces the most imminent games.
func TestEventOrdering(t *testing.T) {
	const dflt = "e.date DESC"

	tests := []struct {
		ordering string
		want     string
	}{
		{"", "e.date DESC"},      // default (no param) -> -date
		{"date", "e.date ASC"},   // ascending
		{"-date", "e.date DESC"}, // explicit descending
		{"created_at", "e.created_at ASC"},
		{"-created_at", "e.created_at DESC"},
		{"name", "e.date DESC"}, // not whitelisted -> default
	}

	for _, tt := range tests {
		got := drf.ResolveOrdering(tt.ordering, eventOrdering, dflt)
		if got != tt.want {
			t.Fatalf("ordering %q: got %q want %q", tt.ordering, got, tt.want)
		}
	}
}
