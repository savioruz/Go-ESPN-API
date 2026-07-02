// Package base64 provides utilities for working with base64-encoded strings.
// nolint:revive
package base64

import "strings"

// GetContentType extracts the content type from a base64-encoded string.
func GetContentType(file string) string {
	start := len("data:")
	end := strings.Index(file, ";base64,")

	if end == -1 || end < start {
		return ""
	}

	return file[start:end]
}
