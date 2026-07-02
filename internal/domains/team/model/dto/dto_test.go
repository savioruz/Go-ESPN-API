package dto

import (
	"encoding/json"
	"testing"
)

func TestComputePrimaryLogo(t *testing.T) {
	tests := []struct {
		name  string
		logos string
		want  *string
	}{
		{
			name:  "default rel wins over earlier non-default logo",
			logos: `[{"href":"https://a/first.png","rel":["full"]},{"href":"https://a/default.png","rel":["default","full"]}]`,
			want:  strptr("https://a/default.png"),
		},
		{
			name:  "fallback to first logo href when no default rel",
			logos: `[{"href":"https://a/first.png","rel":["full"]},{"href":"https://a/second.png","rel":["dark"]}]`,
			want:  strptr("https://a/first.png"),
		},
		{
			name:  "empty array -> nil",
			logos: `[]`,
			want:  nil,
		},
		{
			name:  "empty raw -> nil",
			logos: ``,
			want:  nil,
		},
		{
			name:  "default logo missing href -> nil",
			logos: `[{"rel":["default"]}]`,
			want:  nil,
		},
		{
			name:  "first logo missing href, no default -> nil",
			logos: `[{"rel":["full"]}]`,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.logos != "" {
				raw = json.RawMessage(tt.logos)
			}

			got := ComputePrimaryLogo(raw)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("nil mismatch: got %v want %v", got, tt.want)
			}

			if got != nil && *got != *tt.want {
				t.Fatalf("got %q want %q", *got, *tt.want)
			}
		})
	}
}

func strptr(s string) *string { return &s }
