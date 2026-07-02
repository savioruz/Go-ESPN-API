// Package team wires the team HTTP endpoints.
package team

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/team/repository"
	"go-espn-api/internal/domains/team/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the team endpoints.
type Handler struct {
	service service.Team
	otel    otel.Otel
}

// New creates a new team handler.
func New(service service.Team, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers team routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/teams", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/espn/{espn_id}", h.GetByESPNID)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of teams.
//
// @Summary     List teams
// @Description Return a paginated list of teams.
// @Tags        Teams
// @Produce     json
// @Param       sport query string false "Filter by sport slug"
// @Param       league query string false "Filter by league slug"
// @Param       is_active query bool false "Filter by active status"
// @Param       abbreviation query string false "Filter by abbreviation"
// @Param       search query string false "Search term"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/teams/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".team.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Sport:        q.Get("sport"),
		League:       q.Get("league"),
		IsActive:     parseBoolPtr(q.Get("is_active")),
		Abbreviation: q.Get("abbreviation"),
		Search:       q.Get("search"),
		Ordering:     q.Get("ordering"),
	}

	items, count, err := h.service.List(ctx, filter, page, constant.DefaultPageSize)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	response.DRFList(w, r, count, page, constant.DefaultPageSize, items)
}

// GetByID returns a single team by id.
//
// @Summary     Get team by id
// @Description Return a single team by its id.
// @Tags        Teams
// @Produce     json
// @Param       id path int true "Team id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/teams/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".team.GetByID")
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

// GetByESPNID returns a single team by ESPN id.
//
// @Summary     Get team by ESPN id
// @Description Return a single team by its ESPN id.
// @Tags        Teams
// @Produce     json
// @Param       espn_id path string true "Team ESPN id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/teams/espn/{espn_id}/ [get]
func (h *Handler) GetByESPNID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".team.GetByESPNID")
	defer scope.End()

	espnID := chi.URLParam(r, "espn_id")

	item, err := h.service.GetByESPNID(ctx, espnID)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	if item == nil {
		response.DRFNotFound(w, "Team not found")

		return
	}

	response.DRFObject(w, http.StatusOK, item)
}

// parseBoolPtr parses a django-style boolean query param. Unrecognised values
// yield nil (filter skipped), matching django-filter's lenient behaviour.
func parseBoolPtr(raw string) *bool {
	switch raw {
	case "true", "True", "1":
		v := true

		return &v
	case "false", "False", "0":
		v := false

		return &v
	default:
		return nil
	}
}
