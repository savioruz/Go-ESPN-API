// Package nhlskaterstats wires the NHL skater-stats HTTP endpoints.
package nhlskaterstats

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlskaterstats/repository"
	"go-espn-api/internal/domains/nhlskaterstats/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the NHL skater-stats endpoints.
type Handler struct {
	service service.NHLSkaterStats
	otel    otel.Otel
}

// New creates a new NHL skater-stats handler.
func New(service service.NHLSkaterStats, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers NHL skater-stats routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/skater-stats", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of NHL skater season stats.
//
// @Summary     List NHL skater season stats
// @Description Return a paginated list of NHL skater season stats.
// @Tags        NHL Stats
// @Produce     json
// @Param       season query string false "Filter by season (e.g. 20232024)"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/skater-stats/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_skater_stats.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Season:   q.Get("season"),
		Ordering: q.Get("ordering"),
	}

	items, count, err := h.service.List(ctx, filter, page, constant.DefaultPageSize)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	response.DRFList(w, r, count, page, constant.DefaultPageSize, items)
}

// GetByID returns a single NHL skater season stats row by id.
//
// @Summary     Get NHL skater season stats by id
// @Description Return a single NHL skater season stats row by its id.
// @Tags        NHL Stats
// @Produce     json
// @Param       id path int true "NHL skater stats id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/skater-stats/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_skater_stats.GetByID")
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
