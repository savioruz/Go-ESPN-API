// Package nhlstanding wires the NHL standing HTTP endpoints.
package nhlstanding

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlstanding/repository"
	"go-espn-api/internal/domains/nhlstanding/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the NHL standing endpoints.
type Handler struct {
	service service.NHLStanding
	otel    otel.Otel
}

// New creates a new NHL standing handler.
func New(service service.NHLStanding, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers NHL standing routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/standings", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of NHL standings.
//
// @Summary     List NHL standings
// @Description Return a paginated list of NHL standings.
// @Tags        NHL Standings
// @Produce     json
// @Param       date query string false "Filter by date (YYYY-MM-DD)"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/standings/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_standing.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Date:     q.Get("date"),
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

// GetByID returns a single NHL standing by id.
//
// @Summary     Get NHL standing by id
// @Description Return a single NHL standing by its id.
// @Tags        NHL Standings
// @Produce     json
// @Param       id path int true "NHL standing id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/standings/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_standing.GetByID")
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
