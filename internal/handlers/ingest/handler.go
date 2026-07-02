// Package ingest wires the ESPN write-side ingestion HTTP endpoints.
package ingest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/ingest"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// defaultNewsLimit is the fallback news limit when the request omits it.
const defaultNewsLimit = 50

// Handler serves the POST /api/v1/ingest/* endpoints.
type Handler struct {
	scoreboard   *ingest.ScoreboardService
	teams        *ingest.TeamsService
	news         *ingest.NewsService
	injuries     *ingest.InjuriesService
	transactions *ingest.TransactionsService
	otel         otel.Otel
}

// New creates a new ingest handler.
func New(
	scoreboard *ingest.ScoreboardService,
	teams *ingest.TeamsService,
	news *ingest.NewsService,
	injuries *ingest.InjuriesService,
	transactions *ingest.TransactionsService,
	otl otel.Otel,
) Handler {
	return Handler{
		scoreboard:   scoreboard,
		teams:        teams,
		news:         news,
		injuries:     injuries,
		transactions: transactions,
		otel:         otl,
	}
}

// Router registers the ingest routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/ingest", func(rg chi.Router) {
		rg.Post("/scoreboard", h.Scoreboard)
		rg.Post("/teams", h.Teams)
		rg.Post("/news", h.News)
		rg.Post("/injuries", h.Injuries)
		rg.Post("/transactions", h.Transactions)
	})
}

// ingestResponse mirrors IngestionResultSerializer. Field order is fixed to
// match the Django JSON body exactly.
type ingestResponse struct {
	Created        int      `json:"created"`
	Updated        int      `json:"updated"`
	Errors         int      `json:"errors"`
	TotalProcessed int      `json:"total_processed"`
	Details        []string `json:"details"`
}

func toResponse(r ingest.IngestionResult) ingestResponse {
	return ingestResponse{
		Created:        r.Created,
		Updated:        r.Updated,
		Errors:         r.Errors,
		TotalProcessed: r.TotalProcessed(),
		Details:        r.Details,
	}
}

type sportLeagueRequest struct {
	Sport  string `json:"sport"`
	League string `json:"league"`
}

type scoreboardRequest struct {
	Sport  string `json:"sport"`
	League string `json:"league"`
	Date   string `json:"date"`
}

type newsRequest struct {
	Sport  string `json:"sport"`
	League string `json:"league"`
	Limit  *int   `json:"limit"`
}

// Scoreboard ingests scoreboard data from ESPN.
//
// @Summary     Ingest scoreboard data
// @Description Fetch scoreboard data from ESPN for a sport, league and date, then upsert events and competitors.
// @Tags        Ingest
// @Accept      json
// @Produce     json
// @Param       request body scoreboardRequest true "Ingest scoreboard request"
// @Success     200 {object} ingestResponse
// @Failure     400 {object} response.Error
// @Failure     502 {object} response.Error
// @Router      /api/v1/ingest/scoreboard/ [post]
// @Security    X-API-Key
func (h *Handler) Scoreboard(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".ingest.Scoreboard")
	defer scope.End()

	var req scoreboardRequest
	if err := decodeBody(r.Body, &req); err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	sport, league, err := requireSportLeague(req.Sport, req.League)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	date, err := validateDate(req.Date)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	result, err := h.scoreboard.IngestScoreboard(ctx, sport, league, date)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, ingestError(err))

		return
	}

	response.DRFObject(w, http.StatusOK, toResponse(result))
}

// Teams ingests all teams for a sport and league.
//
// @Summary     Ingest teams data
// @Description Fetch all teams from ESPN for a sport and league, then upsert them.
// @Tags        Ingest
// @Accept      json
// @Produce     json
// @Param       request body sportLeagueRequest true "Ingest teams request"
// @Success     200 {object} ingestResponse
// @Failure     400 {object} response.Error
// @Failure     502 {object} response.Error
// @Router      /api/v1/ingest/teams/ [post]
// @Security    X-API-Key
func (h *Handler) Teams(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".ingest.Teams")
	defer scope.End()

	sport, league, err := h.decodeSportLeague(r.Body)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	result, err := h.teams.IngestTeams(ctx, sport, league)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, ingestError(err))

		return
	}

	response.DRFObject(w, http.StatusOK, toResponse(result))
}

// News ingests news articles for a sport and league.
//
// @Summary     Ingest news articles
// @Description Fetch news articles from ESPN for a sport and league, then upsert them.
// @Tags        Ingest
// @Accept      json
// @Produce     json
// @Param       request body newsRequest true "Ingest news request"
// @Success     200 {object} ingestResponse
// @Failure     400 {object} response.Error
// @Failure     502 {object} response.Error
// @Router      /api/v1/ingest/news/ [post]
// @Security    X-API-Key
func (h *Handler) News(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".ingest.News")
	defer scope.End()

	var req newsRequest
	if err := decodeBody(r.Body, &req); err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	sport, league, err := requireSportLeague(req.Sport, req.League)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	limit, err := validateLimit(req.Limit)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	result, err := h.news.IngestNews(ctx, sport, league, limit)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, ingestError(err))

		return
	}

	response.DRFObject(w, http.StatusOK, toResponse(result))
}

// Injuries refreshes the league injury snapshot.
//
// @Summary     Ingest injury report
// @Description Fetch the current league injury report from ESPN and refresh the DB snapshot.
// @Tags        Ingest
// @Accept      json
// @Produce     json
// @Param       request body sportLeagueRequest true "Ingest injuries request"
// @Success     200 {object} ingestResponse
// @Failure     400 {object} response.Error
// @Failure     502 {object} response.Error
// @Router      /api/v1/ingest/injuries/ [post]
// @Security    X-API-Key
func (h *Handler) Injuries(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".ingest.Injuries")
	defer scope.End()

	sport, league, err := h.decodeSportLeague(r.Body)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	result, err := h.injuries.IngestInjuries(ctx, sport, league)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, ingestError(err))

		return
	}

	response.DRFObject(w, http.StatusOK, toResponse(result))
}

// Transactions ingests recent transactions for a sport and league.
//
// @Summary     Ingest transactions
// @Description Fetch the latest transactions from ESPN for a sport and league, then upsert them.
// @Tags        Ingest
// @Accept      json
// @Produce     json
// @Param       request body sportLeagueRequest true "Ingest transactions request"
// @Success     200 {object} ingestResponse
// @Failure     400 {object} response.Error
// @Failure     502 {object} response.Error
// @Router      /api/v1/ingest/transactions/ [post]
// @Security    X-API-Key
func (h *Handler) Transactions(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".ingest.Transactions")
	defer scope.End()

	sport, league, err := h.decodeSportLeague(r.Body)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	result, err := h.transactions.IngestTransactions(ctx, sport, league)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, ingestError(err))

		return
	}

	response.DRFObject(w, http.StatusOK, toResponse(result))
}

// decodeSportLeague decodes and validates a {sport, league} body.
func (h *Handler) decodeSportLeague(body io.Reader) (string, string, error) {
	var req sportLeagueRequest
	if err := decodeBody(body, &req); err != nil {
		return "", "", err
	}

	return requireSportLeague(req.Sport, req.League)
}

// decodeBody decodes JSON, mapping any decode failure to a 400.
func decodeBody(body io.Reader, dst any) error {
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		return failure.BadRequestWithKey(errkey.ErrValidationFailed, "Invalid request body")
	}

	return nil
}

// requireSportLeague lowercases + trims sport/league and rejects blanks (400).
func requireSportLeague(sport, league string) (string, string, error) {
	sport = strings.ToLower(strings.TrimSpace(sport))
	league = strings.ToLower(strings.TrimSpace(league))

	if sport == "" {
		return "", "", failure.BadRequestWithKey(errkey.ErrValidationFailed, "sport is required")
	}

	if league == "" {
		return "", "", failure.BadRequestWithKey(errkey.ErrValidationFailed, "league is required")
	}

	return sport, league, nil
}

// validateDate accepts an empty date or an 8-digit YYYYMMDD string.
func validateDate(date string) (string, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return "", nil
	}

	if len(date) != 8 || !isDigits(date) {
		return "", failure.BadRequestWithKey(errkey.ErrValidationFailed, "Date must be in YYYYMMDD format (e.g., '20241215')")
	}

	return date, nil
}

// validateLimit defaults to 50 and enforces the 1..200 range.
func validateLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultNewsLimit, nil
	}

	if *limit < 1 || *limit > 200 {
		return 0, failure.BadRequestWithKey(errkey.ErrValidationFailed, "limit must be between 1 and 200")
	}

	return *limit, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// ingestError maps an ingestion failure to a 502 (matches the Django endpoint's
// documented "ESPN API error" response).
func ingestError(err error) error {
	return failure.NewWithKey(errkey.ErrExternalService, http.StatusBadGateway, err.Error())
}
