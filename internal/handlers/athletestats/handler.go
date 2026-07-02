// Package athletestats wires the athlete-stats HTTP endpoints.
package athletestats

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/athletestats/repository"
	"go-espn-api/internal/domains/athletestats/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the athlete-stats endpoints.
type Handler struct {
	service service.AthleteStats
	otel    otel.Otel
}

// New creates a new athlete-stats handler.
func New(service service.AthleteStats, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers athlete-stats routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/athlete-stats", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of athlete season stats.
//
// @Summary     List athlete stats
// @Description Return a paginated list of athlete season stats.
// @Tags        Athlete Stats
// @Produce     json
// @Param       sport query string false "Filter by sport slug"
// @Param       league query string false "Filter by league slug"
// @Param       season query string false "Filter by season"
// @Param       athlete_espn_id query string false "Filter by athlete ESPN id"
// @Param       search query string false "Search term"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/athlete-stats/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".athlete_stats.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Sport:         q.Get("sport"),
		League:        q.Get("league"),
		Season:        q.Get("season"),
		AthleteESPNID: q.Get("athlete_espn_id"),
		Search:        q.Get("search"),
		Ordering:      q.Get("ordering"),
	}

	items, count, err := h.service.List(ctx, filter, page, constant.DefaultPageSize)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	response.DRFList(w, r, count, page, constant.DefaultPageSize, items)
}

// GetByID returns a single athlete season stats row by id.
//
// @Summary     Get athlete stats by id
// @Description Return a single athlete season stats row by its id.
// @Tags        Athlete Stats
// @Produce     json
// @Param       id path int true "Athlete stats id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/athlete-stats/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".athlete_stats.GetByID")
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
