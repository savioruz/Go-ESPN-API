// Package nhl provides HTTP clients for the public NHL APIs.
//
//nolint:revive
package nhl

//go:generate go run go.uber.org/mock/mockgen -source=./nhl.go -destination=./mocks/nhl_mock.go -package=mocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"go-espn-api/config"
	"go-espn-api/infras/otel"
	"go-espn-api/shared/constant"
)

// Base URLs for the two NHL API surfaces.
const (
	baseURLWeb   = "https://api-web.nhle.com"
	baseURLStats = "https://api.nhle.com/stats/rest"

	// defaultTimeoutSeconds is the fallback HTTP client timeout when unconfigured.
	defaultTimeoutSeconds = 30
	// backoffMinSeconds / backoffMaxSeconds mirror the Python
	// wait_exponential(multiplier=1, min=2, max=10).
	backoffMinSeconds = 2
	backoffMaxSeconds = 10
)

// ErrClient indicates a generic NHL API/client error (transport failure, non-2xx
// status, or invalid JSON). Inspect with errors.Is.
var ErrClient = errors.New("nhl: client error")

// Response wraps a parsed NHL API response.
type Response struct {
	// Data is the raw JSON body.
	Data json.RawMessage
	// StatusCode is the upstream HTTP status code.
	StatusCode int
	// URL is the fully-built upstream request URL.
	URL string
}

// NHL exposes the NHLWebClient (api-web.nhle.com) and NHLStatsClient
// (api.nhle.com/stats/rest) endpoints reachable from the Python ingestion.
type NHL interface {
	// Web client (api-web.nhle.com).
	GetStandings(ctx context.Context) (*Response, error)
	GetRoster(ctx context.Context, teamAbbrev string) (*Response, error)
	GetPlayerLanding(ctx context.Context, playerID string) (*Response, error)
	GetSchedule(ctx context.Context, date string) (*Response, error)
	GetBoxscore(ctx context.Context, gameID string) (*Response, error)

	// Stats client (api.nhle.com/stats/rest).
	GetTeams(ctx context.Context) (*Response, error)
	GetSkaterSummary(ctx context.Context, seasonID string, limit int) (*Response, error)
	GetGoalieSummary(ctx context.Context, seasonID string, limit int) (*Response, error)
}

type nhlImpl struct {
	Config *config.Config
	otel   otel.Otel

	httpClient  *http.Client
	webBaseURL  string
	statBaseURL string

	maxRetries int
	backoffMin time.Duration
	backoffMax time.Duration
}

// New creates a new NHL client from configuration.
func New(cfg *config.Config, otl otel.Otel) NHL {
	timeout := time.Duration(cfg.Services.NHL.Timeout) * time.Second
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds * time.Second
	}

	maxRetries := cfg.Services.NHL.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}

	return &nhlImpl{
		Config: cfg,
		otel:   otl,
		httpClient: &http.Client{
			Timeout: timeout,
			// Auto-instrument outbound calls: emit client spans and inject the
			// W3C traceparent so downstream NHL requests continue the trace.
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		webBaseURL:  baseURLWeb,
		statBaseURL: baseURLStats,
		maxRetries:  maxRetries,
		// Mirrors the Python wait_exponential(multiplier=1, min=2, max=10).
		backoffMin: backoffMinSeconds * time.Second,
		backoffMax: backoffMaxSeconds * time.Second,
	}
}

// --------------------- Web client methods ---------------------

// GetStandings gets the current NHL standings.
func (c *nhlImpl) GetStandings(ctx context.Context) (*Response, error) {
	return c.getWeb(ctx, "GetStandings", "v1/standings/now", nil)
}

// GetRoster gets the current roster for a team.
func (c *nhlImpl) GetRoster(ctx context.Context, teamAbbrev string) (*Response, error) {
	return c.getWeb(ctx, "GetRoster", fmt.Sprintf("v1/roster/%s/current", teamAbbrev), nil)
}

// GetPlayerLanding gets the full player profile.
func (c *nhlImpl) GetPlayerLanding(ctx context.Context, playerID string) (*Response, error) {
	return c.getWeb(ctx, "GetPlayerLanding", fmt.Sprintf("v1/player/%s/landing", playerID), nil)
}

// GetSchedule gets the schedule (now, or a specific YYYY-MM-DD date).
func (c *nhlImpl) GetSchedule(ctx context.Context, date string) (*Response, error) {
	endpoint := "v1/schedule/now"
	if date != "" {
		endpoint = "v1/schedule/" + date
	}

	return c.getWeb(ctx, "GetSchedule", endpoint, nil)
}

// GetBoxscore gets the boxscore for a game.
func (c *nhlImpl) GetBoxscore(ctx context.Context, gameID string) (*Response, error) {
	return c.getWeb(ctx, "GetBoxscore", fmt.Sprintf("v1/gamecenter/%s/boxscore", gameID), nil)
}

// --------------------- Stats client methods ---------------------

// GetTeams gets all NHL teams.
func (c *nhlImpl) GetTeams(ctx context.Context) (*Response, error) {
	return c.getStats(ctx, "GetTeams", "en/team", nil)
}

// GetSkaterSummary gets the skater scoring summary for a season. limit < 0 means
// no cap (ESPN/NHL sentinel -1).
func (c *nhlImpl) GetSkaterSummary(ctx context.Context, seasonID string, limit int) (*Response, error) {
	params := map[string]string{
		"cayenneExp": "seasonId=" + seasonID,
		"limit":      strconv.Itoa(limit),
	}

	return c.getStats(ctx, "GetSkaterSummary", "en/skater/summary", params)
}

// GetGoalieSummary gets the goalie summary for a season.
func (c *nhlImpl) GetGoalieSummary(ctx context.Context, seasonID string, limit int) (*Response, error) {
	params := map[string]string{
		"cayenneExp": "seasonId=" + seasonID,
		"limit":      strconv.Itoa(limit),
	}

	return c.getStats(ctx, "GetGoalieSummary", "en/goalie/summary", params)
}

// --------------------- HTTP core ---------------------

func (c *nhlImpl) getWeb(ctx context.Context, method, endpoint string, params map[string]string) (*Response, error) {
	return c.do(ctx, method, c.webBaseURL, endpoint, params)
}

func (c *nhlImpl) getStats(ctx context.Context, method, endpoint string, params map[string]string) (*Response, error) {
	return c.do(ctx, method, c.statBaseURL, endpoint, params)
}

func (c *nhlImpl) do(ctx context.Context, method, base, endpoint string, params map[string]string) (resp *Response, err error) {
	ctx, scope := c.otel.NewScope(ctx, constant.OtelNHLScopeName, constant.OtelNHLScopeName+"."+method)
	defer scope.End()
	defer scope.TraceIfError(err)

	fullURL := buildURL(base, endpoint, params)
	scope.SetAttributes(map[string]any{constant.OtelQueryAttributeKey: fullURL})

	return c.requestWithRetry(ctx, fullURL)
}

func buildURL(base, endpoint string, params map[string]string) string {
	full := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
	if len(params) == 0 {
		return full
	}

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}

	return full + "?" + q.Encode()
}

// requestWithRetry mirrors the Python tenacity retry on the NHL clients:
// raise_for_status makes any non-2xx (and transport errors) retryable, with
// exponential backoff. Respects context cancellation.
func (c *nhlImpl) requestWithRetry(ctx context.Context, fullURL string) (*Response, error) {
	var lastErr error

	for attempt := range c.maxRetries {
		if attempt > 0 {
			if werr := c.waitBackoff(ctx, attempt); werr != nil {
				return nil, werr
			}
		}

		resp, err := c.attempt(ctx, fullURL)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		log.Debug().Str("url", fullURL).Int("attempt", attempt+1).Err(err).Msg("nhl_request_retry")
	}

	log.Error().Str("url", fullURL).Int("retries", c.maxRetries).Err(lastErr).Msg("nhl_request_failed_after_retries")

	return nil, lastErr
}

func (c *nhlImpl) attempt(ctx context.Context, fullURL string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrClient, err)
	}

	req.Header.Set("Accept", constant.ContentTypeJSON)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: transport error: %w", ErrClient, err)
	}

	defer func() { _ = httpResp.Body.Close() }()

	// raise_for_status equivalent: any 4xx/5xx is an error (and retryable).
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		log.Warn().Str("url", fullURL).Int("status_code", httpResp.StatusCode).Msg("nhl_non_2xx")

		return nil, fmt.Errorf("%w: status %d", ErrClient, httpResp.StatusCode)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %w", ErrClient, err)
	}

	if !json.Valid(body) {
		return nil, fmt.Errorf("%w: invalid JSON response", ErrClient)
	}

	return &Response{Data: body, StatusCode: httpResp.StatusCode, URL: fullURL}, nil
}

func (c *nhlImpl) waitBackoff(ctx context.Context, attempt int) error {
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
