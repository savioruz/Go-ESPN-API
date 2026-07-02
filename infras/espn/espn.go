//nolint:revive
package espn

//go:generate go run go.uber.org/mock/mockgen -source=./espn.go -destination=./mocks/espn_mock.go -package=mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"go-espn-api/config"
	"go-espn-api/infras/otel"
	"go-espn-api/shared/constant"
)

// Domain identifies one of the six ESPN API base-URL domains.
type Domain int

const (
	// DomainSite is site.api.espn.com (scoreboard, teams, news, injuries, ...).
	DomainSite Domain = iota
	// DomainCore is sports.core.api.espn.com (core data, odds, play-by-play).
	DomainCore
	// DomainSiteV2 is site.api.espn.com/apis/v2/ (standings only).
	DomainSiteV2
	// DomainWebV3 is site.web.api.espn.com (athlete stats, gamelog, splits).
	DomainWebV3
	// DomainCDN is cdn.espn.com/core/ (full game packages).
	DomainCDN
	// DomainNow is now.core.api.espn.com/v1/ (real-time news).
	DomainNow
)

// Default base URLs for each ESPN domain.
const (
	baseURLSite  = "https://site.api.espn.com"
	baseURLCore  = "https://sports.core.api.espn.com"
	baseURLWebV3 = "https://site.web.api.espn.com"
	baseURLCDN   = "https://cdn.espn.com"
	baseURLNow   = "https://now.core.api.espn.com"

	defaultUserAgent = "ESPN-Service/1.0"

	relayHeaderTarget = "x-relay-target"
	relayHeaderPath   = "x-relay-path"

	// defaultTimeoutSeconds is the fallback HTTP client timeout when unconfigured.
	defaultTimeoutSeconds = 30
)

// Response wraps a parsed ESPN API response.
type Response struct {
	// Data is the raw JSON body.
	Data json.RawMessage
	// StatusCode is the upstream HTTP status code.
	StatusCode int
	// URL is the fully-built upstream request URL (for logging/tracing).
	URL string
}

// ESPN defines the interface for ESPN API interactions. Only the subset of
// methods reachable from the Python ingestion pipeline is exposed; Get is the
// generic escape hatch used to reach any other ESPN endpoint.
type ESPN interface {
	Get(ctx context.Context, path string, domain Domain, params map[string]string) (*Response, error)
	GetScoreboard(ctx context.Context, sport, league, date string, limit int) (*Response, error)
	GetTeams(ctx context.Context, sport, league string, limit int) (*Response, error)
	GetNews(ctx context.Context, sport, league string, limit int) (*Response, error)
	GetLeagueInjuries(ctx context.Context, sport, league string) (*Response, error)
	GetLeagueTransactions(ctx context.Context, sport, league string) (*Response, error)
	GetAthleteStats(ctx context.Context, sport, league, athleteID string, season, seasonType int) (*Response, error)
}

type espnImpl struct {
	Config *config.Config
	otel   otel.Otel

	httpClient *http.Client
	baseURLs   map[Domain]string
	relays     []string
	userAgent  string

	maxRetries int
	backoffMin time.Duration
	backoffMax time.Duration

	relayCursor uint64
}

// New creates a new ESPN client from configuration.
func New(cfg *config.Config, otl otel.Otel) ESPN {
	timeout := time.Duration(cfg.Services.ESPN.Timeout) * time.Second
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds * time.Second
	}

	maxRetries := cfg.Services.ESPN.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}

	backoffMin := time.Duration(cfg.Services.ESPN.RetryBackoffMin) * time.Second
	if backoffMin <= 0 {
		backoffMin = time.Second
	}

	backoffMax := time.Duration(cfg.Services.ESPN.RetryBackoffMax) * time.Second
	if backoffMax < backoffMin {
		backoffMax = backoffMin
	}

	relays := make([]string, 0, len(cfg.Services.ESPN.Relays))
	for _, r := range cfg.Services.ESPN.Relays {
		if trimmed := strings.TrimRight(strings.TrimSpace(r), "/"); trimmed != "" {
			relays = append(relays, trimmed)
		}
	}

	return &espnImpl{
		Config:     cfg,
		otel:       otl,
		httpClient: &http.Client{Timeout: timeout},
		baseURLs:   defaultBaseURLs(),
		relays:     relays,
		userAgent:  defaultUserAgent,
		maxRetries: maxRetries,
		backoffMin: backoffMin,
		backoffMax: backoffMax,
	}
}

func defaultBaseURLs() map[Domain]string {
	return map[Domain]string{
		DomainSite:   baseURLSite,
		DomainCore:   baseURLCore,
		DomainSiteV2: baseURLSite, // site/v2 shares the site host
		DomainWebV3:  baseURLWebV3,
		DomainCDN:    baseURLCDN,
		DomainNow:    baseURLNow,
	}
}

func (c *espnImpl) buildURL(domain Domain, path string, params map[string]string) string {
	base := strings.TrimRight(c.baseURLs[domain], "/")
	full := base + "/" + strings.TrimLeft(path, "/")

	if len(params) == 0 {
		return full
	}

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}

	return full + "?" + q.Encode()
}

// Get makes a GET request to the ESPN API with retry, relay failover, and
// structured error handling.
func (c *espnImpl) Get(ctx context.Context, path string, domain Domain, params map[string]string) (resp *Response, err error) {
	ctx, scope := c.otel.NewScope(ctx, constant.OtelESPNScopeName, constant.OtelESPNScopeName+".Get")
	defer scope.End()
	defer scope.TraceIfError(err)

	fullURL := c.buildURL(domain, path, params)
	scope.SetAttributes(map[string]any{constant.OtelQueryAttributeKey: fullURL})

	return c.requestWithRetry(ctx, http.MethodGet, fullURL)
}

// requestWithRetry mirrors the Python _request_with_retry: exponential backoff
// on network errors, timeouts, HTTP 429, and 5xx; give up after maxRetries. A
// 404 short-circuits to ErrNotFound and other 4xx to ErrClient without retry.
func (c *espnImpl) requestWithRetry(ctx context.Context, method, fullURL string) (*Response, error) {
	var lastErr error

	for attempt := range c.maxRetries {
		if attempt > 0 {
			if werr := c.waitBackoff(ctx, attempt); werr != nil {
				return nil, werr
			}
		}

		resp, retryable, err := c.attempt(ctx, method, fullURL)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !retryable {
			return nil, err
		}

		log.Debug().Str("url", fullURL).Int("attempt", attempt+1).Err(err).Msg("espn_request_retry")
	}

	log.Error().Str("url", fullURL).Int("retries", c.maxRetries).Err(lastErr).Msg("espn_request_failed_after_retries")

	return nil, lastErr
}

// attempt performs a single send + response handling. The bool reports whether
// the returned error is retryable.
func (c *espnImpl) attempt(ctx context.Context, method, fullURL string) (*Response, bool, error) {
	httpResp, err := c.send(ctx, method, fullURL)
	if err != nil {
		return nil, true, fmt.Errorf("%w: transport error: %w", ErrClient, err)
	}

	defer func() { _ = httpResp.Body.Close() }()

	return c.handleResponse(httpResp, fullURL)
}

// handleResponse maps HTTP status codes to Response / sentinel errors.
func (c *espnImpl) handleResponse(httpResp *http.Response, fullURL string) (*Response, bool, error) {
	switch {
	case httpResp.StatusCode == http.StatusNotFound:
		log.Warn().Str("url", fullURL).Msg("espn_resource_not_found")

		return nil, false, fmt.Errorf("%w: %s", ErrNotFound, fullURL)

	case httpResp.StatusCode == http.StatusTooManyRequests:
		log.Warn().Str("url", fullURL).Msg("espn_rate_limited")

		return nil, true, fmt.Errorf("%w: %s", ErrRateLimit, fullURL)

	case httpResp.StatusCode >= http.StatusInternalServerError:
		log.Error().Str("url", fullURL).Int("status_code", httpResp.StatusCode).Msg("espn_server_error")

		return nil, true, fmt.Errorf("%w: server error %d", ErrClient, httpResp.StatusCode)

	case httpResp.StatusCode >= http.StatusBadRequest:
		log.Error().Str("url", fullURL).Int("status_code", httpResp.StatusCode).Msg("espn_client_error")

		return nil, false, fmt.Errorf("%w: status %d", ErrClient, httpResp.StatusCode)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("%w: read body: %w", ErrClient, err)
	}

	if !json.Valid(body) {
		log.Error().Str("url", fullURL).Msg("espn_json_parse_error")

		return nil, false, fmt.Errorf("%w: invalid JSON response", ErrClient)
	}

	return &Response{Data: body, StatusCode: httpResp.StatusCode, URL: fullURL}, false, nil
}

// send delivers the request via the relay pool (round-robin with failover),
// falling back to a direct request only if every relay fails. Mirrors the
// Python _send.
func (c *espnImpl) send(ctx context.Context, method, fullURL string) (*http.Response, error) {
	if resp, ok := c.sendViaRelays(ctx, method, fullURL); ok {
		return resp, nil
	}

	req, err := c.newRequest(ctx, method, fullURL)
	if err != nil {
		return nil, err
	}

	return c.httpClient.Do(req)
}

// sendViaRelays attempts the request through the relay pool (round-robin with
// per-relay failover). The bool reports whether a relay returned a 2xx; on
// false the caller falls back to a direct request.
func (c *espnImpl) sendViaRelays(ctx context.Context, method, fullURL string) (*http.Response, bool) {
	if len(c.relays) == 0 {
		return nil, false
	}

	parsed, err := url.Parse(fullURL)
	if err != nil {
		return nil, false
	}

	target := parsed.Scheme + "://" + parsed.Host
	relayPath := parsed.RequestURI()

	n := len(c.relays)
	start := int(atomic.AddUint64(&c.relayCursor, 1)-1) % n

	for i := range n {
		relay := c.relays[(start+i)%n]

		req, rerr := c.newRequest(ctx, method, relay)
		if rerr != nil {
			log.Warn().Str("url", fullURL).Str("relay", relay).Err(rerr).Msg("espn_relay_error")

			continue
		}

		req.Header.Set(relayHeaderTarget, target)
		req.Header.Set(relayHeaderPath, relayPath)

		resp, derr := c.httpClient.Do(req)
		if derr != nil {
			log.Warn().Str("url", fullURL).Str("relay", relay).Err(derr).Msg("espn_relay_error")

			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, true
		}

		log.Warn().Str("url", fullURL).Str("relay", relay).Int("relay_status", resp.StatusCode).Msg("espn_relay_non_2xx")
		_ = resp.Body.Close()
	}

	log.Warn().Str("url", fullURL).Int("relays", n).Msg("espn_relay_pool_exhausted_fallback_direct")

	return nil, false
}

func (c *espnImpl) newRequest(ctx context.Context, method, reqURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set(constant.RequestHeaderUserAgent, c.userAgent)
	req.Header.Set("Accept", constant.ContentTypeJSON)

	return req, nil
}

// waitBackoff sleeps with exponential backoff clamped to [backoffMin, backoffMax],
// respecting context cancellation. Mirrors tenacity wait_exponential.
func (c *espnImpl) waitBackoff(ctx context.Context, attempt int) error {
	// attempt is 1-based for the backoff sequence.
	delay := c.backoffMin << (attempt - 1)
	if delay < c.backoffMin {
		delay = c.backoffMin
	}

	if delay > c.backoffMax {
		delay = c.backoffMax
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// --------------------- Ported ESPN endpoint methods ---------------------

// GetScoreboard gets the scoreboard/schedule for a sport and league. date is an
// optional YYYYMMDD string; limit <= 0 omits the limit param.
func (c *espnImpl) GetScoreboard(ctx context.Context, sport, league, date string, limit int) (*Response, error) {
	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/scoreboard", sport, league)

	params := map[string]string{}
	if date != "" {
		params["dates"] = date
	}

	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}

	log.Info().Str("sport", sport).Str("league", league).Str("date", date).Msg("fetching_scoreboard")

	return c.Get(ctx, path, DomainSite, params)
}

// GetTeams gets all teams for a sport and league.
func (c *espnImpl) GetTeams(ctx context.Context, sport, league string, limit int) (*Response, error) {
	if limit <= 0 {
		limit = 100
	}

	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/teams", sport, league)
	log.Info().Str("sport", sport).Str("league", league).Msg("fetching_teams")

	return c.Get(ctx, path, DomainSite, map[string]string{"limit": strconv.Itoa(limit)})
}

// GetNews gets news for a sport and league.
func (c *espnImpl) GetNews(ctx context.Context, sport, league string, limit int) (*Response, error) {
	if limit <= 0 {
		limit = 25
	}

	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/news", sport, league)
	log.Info().Str("sport", sport).Str("league", league).Msg("fetching_news")

	return c.Get(ctx, path, DomainSite, map[string]string{"limit": strconv.Itoa(limit)})
}

// GetLeagueInjuries gets the league-wide injury report (all teams).
func (c *espnImpl) GetLeagueInjuries(ctx context.Context, sport, league string) (*Response, error) {
	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/injuries", sport, league)
	log.Info().Str("sport", sport).Str("league", league).Msg("fetching_league_injuries")

	return c.Get(ctx, path, DomainSite, nil)
}

// GetLeagueTransactions gets recent league-wide transactions.
func (c *espnImpl) GetLeagueTransactions(ctx context.Context, sport, league string) (*Response, error) {
	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/transactions", sport, league)
	log.Info().Str("sport", sport).Str("league", league).Msg("fetching_league_transactions")

	return c.Get(ctx, path, DomainSite, nil)
}

// GetAthleteStats gets season stats for an athlete via the common/v3 API.
// season / seasonType <= 0 omit their params.
func (c *espnImpl) GetAthleteStats(ctx context.Context, sport, league, athleteID string, season, seasonType int) (*Response, error) {
	path := fmt.Sprintf("/apis/common/v3/sports/%s/%s/athletes/%s/stats", sport, league, athleteID)

	params := map[string]string{}
	if season > 0 {
		params["season"] = strconv.Itoa(season)
	}

	if seasonType > 0 {
		params["seasontype"] = strconv.Itoa(seasonType)
	}

	log.Info().Str("sport", sport).Str("league", league).Str("athlete_id", athleteID).Msg("fetching_athlete_stats")

	return c.Get(ctx, path, DomainWebV3, params)
}
