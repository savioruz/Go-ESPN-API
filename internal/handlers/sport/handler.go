// Package sport wires the sport HTTP endpoints.
package sport

import (
	"net/http"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/sport/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the sport endpoints.
type Handler struct {
	service service.Sport
	otel    otel.Otel
}

// New creates a new sport handler.
func New(service service.Sport, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers sport routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/sports", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{slug}", h.GetBySlug)
	})
}

// List returns a paginated list of sports.
//
// @Summary     List sports
// @Description Return a paginated list of sports.
// @Tags        Sports
// @Produce     json
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/sports/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".sport.List")
	defer scope.End()

	page := response.ParsePage(r)

	items, count, err := h.service.List(ctx, page, constant.DefaultPageSize)
	if err != nil {
		scope.TraceError(err)
		response.WithError(w, err)

		return
	}

	response.DRFList(w, r, count, page, constant.DefaultPageSize, items)
}

// GetBySlug returns a single sport by slug.
//
// @Summary     Get sport by slug
// @Description Return a single sport by its slug.
// @Tags        Sports
// @Produce     json
// @Param       slug path string true "Sport slug"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/sports/{slug}/ [get]
func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".sport.GetBySlug")
	defer scope.End()

	slug := chi.URLParam(r, "slug")

	item, err := h.service.GetBySlug(ctx, slug)
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
