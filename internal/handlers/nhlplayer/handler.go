// Package nhlplayer wires the NHL player HTTP endpoints.
package nhlplayer

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlplayer/repository"
	"go-espn-api/internal/domains/nhlplayer/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the NHL player endpoints.
type Handler struct {
	service service.NHLPlayer
	otel    otel.Otel
}

// New creates a new NHL player handler.
func New(service service.NHLPlayer, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers NHL player routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/players", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of NHL players.
//
// @Summary     List NHL players
// @Description Return a paginated list of NHL players.
// @Tags        NHL Players
// @Produce     json
// @Param       search query string false "Search term"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/players/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_player.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Search:   q.Get("search"),
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

// GetByID returns a single NHL player by id.
//
// @Summary     Get NHL player by id
// @Description Return a single NHL player by its id.
// @Tags        NHL Players
// @Produce     json
// @Param       id path int true "NHL player id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/players/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_player.GetByID")
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
