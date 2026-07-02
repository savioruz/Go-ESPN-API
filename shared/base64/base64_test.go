package base64_test

import (
	"go-espn-api/shared/base64"
	"testing"
)

func TestGetContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid image png",
			input:    "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expected: "image/png",
		},
		{
			name:     "valid image jpeg",
			input:    "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/2wBDAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwA/wJ8A",
			expected: "image/jpeg",
		},
		{
			name:     "valid text plain",
			input:    "data:text/plain;base64,SGVsbG8gV29ybGQ=",
			expected: "text/plain",
		},
		{
			name:     "valid application json",
			input:    "data:application/json;base64,eyJuYW1lIjoiSm9obiBEb2UifQ==",
			expected: "application/json",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid format - no data prefix",
			input:    "image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expected: "/png",
		},
		{
			name:     "invalid format - no base64 marker",
			input:    "data:image/png,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expected: "",
		},
		{
			name:     "invalid format - missing semicolon",
			input:    "data:image/pngbase64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expected: "",
		},
		{
			name:     "only data prefix",
			input:    "data:",
			expected: "",
		},
		{
			name:     "only data prefix with base64",
			input:    "data:;base64,",
			expected: "",
		},
		{
			name:     "complex content type",
			input:    "data:image/svg+xml;charset=utf-8;base64,PHN2ZyB3aWR0aD0iMTAiIGhlaWdodD0iMTAiPjwvc3ZnPg==",
			expected: "image/svg+xml;charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base64.GetContentType(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
