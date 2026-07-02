//nolint:revive
package espn

import "errors"

// Sentinel errors returned by the ESPN client. They mirror the Python
// ESPNNotFoundError / ESPNRateLimitError / ESPNClientError exceptions and are
// intended to be inspected with errors.Is.
var (
	// ErrNotFound indicates the ESPN resource was not found (HTTP 404).
	ErrNotFound = errors.New("espn: resource not found")
	// ErrRateLimit indicates the ESPN API rate limit was exceeded (HTTP 429).
	ErrRateLimit = errors.New("espn: rate limit exceeded")
	// ErrClient indicates a generic ESPN API/client error (other 4xx, 5xx,
	// transport failures, or JSON parse failures).
	ErrClient = errors.New("espn: client error")
)
