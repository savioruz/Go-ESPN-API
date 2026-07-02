// Package middleware provides HTTP middleware utilities.
package middleware

import (
	"errors"
	"go-espn-api/shared"
	"go-espn-api/shared/cache"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"
	"net/http"
	"strconv"
	"strings"
)

const (
	cacheKeyRateLimit = "limiter"
)

func (a *appMiddleware) RateLimit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !a.config.App.RateLimiter.Enable {
				next.ServeHTTP(w, r)

				return
			}

			maxReqs := a.config.App.RateLimiter.MaxRequests
			windowSecs := a.config.App.RateLimiter.WindowSeconds

			userAgent := a.getUA(r)
			clientIP := a.getClientIP(r)
			cacheKey := shared.BuildCacheKey(cacheKeyRateLimit, clientIP, userAgent)

			var count int

			err := a.cache.Get(r.Context(), cacheKey, &count)
			if err != nil {
				if errors.Is(err, cache.Nil) {
					count = 1
				} else {
					// If cache fails, allow the request to continue
					next.ServeHTTP(w, r)

					return
				}
			} else {
				count++
			}

			if count > maxReqs {
				response.WithRequestLimitExceeded(w)

				return
			}

			err = a.cache.Save(r.Context(), cacheKey, count, windowSecs)
			if err != nil {
				// If cache save fails, allow the request to continue
				next.ServeHTTP(w, r)

				return
			}

			w.Header().Set(constant.RequestHeaderRateLimit, strconv.Itoa(maxReqs))
			w.Header().Set(constant.RequestHeaderRateLimitRemaining, strconv.Itoa(max(0, maxReqs-count)))
			w.Header().Set(constant.RequestHeaderRateLimitWindow, strconv.Itoa(windowSecs))

			next.ServeHTTP(w, r)
		})
	}
}

func (a *appMiddleware) getUA(r *http.Request) string {
	ua := r.Header.Get(constant.RequestHeaderUserAgent)
	if ua == "" {
		ua = "unknown"
	}

	return ua
}

func (a *appMiddleware) getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (most common proxy header)
	if xff := r.Header.Get(constant.RequestHeaderForwardedFor); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if commaIdx := strings.Index(xff, ","); commaIdx > 0 {
			return strings.TrimSpace(xff[:commaIdx])
		}

		return strings.TrimSpace(xff)
	}

	// Check for X-Real-IP header
	if xri := r.Header.Get(constant.RequestHeaderRealIP); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
