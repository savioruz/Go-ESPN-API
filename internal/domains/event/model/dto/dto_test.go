package dto

import "testing"

func intptr(i int) *int { return &i }

func TestComputeScoreInt(t *testing.T) {
	tests := []struct {
		score string
		want  *int
	}{
		{"110", intptr(110)},
		{"0", intptr(0)},
		{"", nil},
		{"abc", nil},
		{"1.5", nil},
		{"-3", intptr(-3)},
	}

	for _, tt := range tests {
		got := ComputeScoreInt(tt.score)
		if (got == nil) != (tt.want == nil) {
			t.Fatalf("score %q: nil mismatch got %v want %v", tt.score, got, tt.want)
		}

		if got != nil && *got != *tt.want {
			t.Fatalf("score %q: got %d want %d", tt.score, *got, *tt.want)
		}
	}
}

func TestGroupCompetitors(t *testing.T) {
	rows := []CompetitorRow{
		{EventID: 1, ID: 10, Order: 0, HomeAway: "away", Score: "90"},
		{EventID: 1, ID: 11, Order: 1, HomeAway: "home", Score: "95"},
		{EventID: 2, ID: 20, Order: 0, HomeAway: "away", Score: ""},
	}

	grouped := GroupCompetitors(rows)

	if len(grouped) != 2 {
		t.Fatalf("expected 2 event groups, got %d", len(grouped))
	}

	if len(grouped[1]) != 2 {
		t.Fatalf("expected 2 competitors for event 1, got %d", len(grouped[1]))
	}

	// Order is preserved from the query ordering (event_id, "order").
	if grouped[1][0].ID != 10 || grouped[1][1].ID != 11 {
		t.Fatalf("competitor order not preserved: %+v", grouped[1])
	}

	// score_int propagates through the response builder.
	if grouped[1][1].ScoreInt == nil || *grouped[1][1].ScoreInt != 95 {
		t.Fatalf("expected score_int 95, got %v", grouped[1][1].ScoreInt)
	}

	if grouped[2][0].ScoreInt != nil {
		t.Fatalf("expected nil score_int for empty score, got %v", *grouped[2][0].ScoreInt)
	}
}
