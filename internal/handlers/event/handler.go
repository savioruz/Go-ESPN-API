// Package event wires the event HTTP endpoints.
package event

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/event/repository"
	"go-espn-api/internal/domains/event/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the event endpoints.
type Handler struct {
	service service.Event
	otel    otel.Otel
}

// New creates a new event handler.
func New(service service.Event, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers event routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/events", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/espn/{espn_id}", h.GetByESPNID)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of events.
//
// @Summary     List events
// @Description Return a paginated list of events.
// @Tags        Events
// @Produce     json
// @Param       sport query string false "Filter by sport slug"
// @Param       league query string false "Filter by league slug"
// @Param       date query string false "Filter by date (YYYY-MM-DD)"
// @Param       date_from query string false "Filter by start date (YYYY-MM-DD)"
// @Param       date_to query string false "Filter by end date (YYYY-MM-DD)"
// @Param       status query string false "Filter by status"
// @Param       season_year query int false "Filter by season year"
// @Param       season_type query int false "Filter by season type"
// @Param       team query string false "Filter by team"
// @Param       search query string false "Search term"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/events/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".event.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Sport:      q.Get("sport"),
		League:     q.Get("league"),
		Date:       q.Get("date"),
		DateFrom:   q.Get("date_from"),
		DateTo:     q.Get("date_to"),
		Status:     q.Get("status"),
		SeasonYear: parseIntPtr(q.Get("season_year")),
		SeasonType: parseIntPtr(q.Get("season_type")),
		Team:       q.Get("team"),
		Search:     q.Get("search"),
		Ordering:   q.Get("ordering"),
	}

	items, count, err := h.service.List(ctx, filter, page, constant.DefaultPageSize)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	response.DRFList(w, r, count, page, constant.DefaultPageSize, items)
}

// GetByID returns a single event by id.
//
// @Summary     Get event by id
// @Description Return a single event by its id.
// @Tags        Events
// @Produce     json
// @Param       id path int true "Event id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/events/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".event.GetByID")
	defer scope.End()

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.DRFNotFound(w, "Not found.")

		return
	}

	item, err := h.service.GetByID(ctx, id)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	if item == nil {
		response.DRFNotFound(w, "Not found.")

		return
	}

	response.DRFObject(w, http.StatusOK, item)
}

// GetByESPNID returns a single event by ESPN id.
//
// @Summary     Get event by ESPN id
// @Description Return a single event by its ESPN id.
// @Tags        Events
// @Produce     json
// @Param       espn_id path string true "Event ESPN id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/events/espn/{espn_id}/ [get]
func (h *Handler) GetByESPNID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".event.GetByESPNID")
	defer scope.End()

	espnID := chi.URLParam(r, "espn_id")

	item, err := h.service.GetByESPNID(ctx, espnID)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	if item == nil {
		response.DRFNotFound(w, "Event not found")

		return
	}

	response.DRFObject(w, http.StatusOK, item)
}

// parseIntPtr parses an integer query param, yielding nil when empty or invalid
// (filter skipped), matching django-filter's NumberFilter leniency.
func parseIntPtr(raw string) *int {
	if raw == "" {
		return nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}

	return &n
}
