// Package nhlgame wires the NHL game HTTP endpoints.
package nhlgame

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/nhlgame/repository"
	"go-espn-api/internal/domains/nhlgame/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the NHL game endpoints.
type Handler struct {
	service service.NHLGame
	otel    otel.Otel
}

// New creates a new NHL game handler.
func New(service service.NHLGame, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers NHL game routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/games", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of NHL games.
//
// @Summary     List NHL games
// @Description Return a paginated list of NHL games.
// @Tags        NHL Games
// @Produce     json
// @Param       season query string false "Filter by season (e.g. 20232024)"
// @Param       game_type query int false "Filter by game type (1=pre, 2=regular, 3=playoff)"
// @Param       status query string false "Filter by status"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/games/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_game.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Season:   q.Get("season"),
		GameType: parseIntPtr(q.Get("game_type")),
		Status:   q.Get("status"),
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

// GetByID returns a single NHL game by id.
//
// @Summary     Get NHL game by id
// @Description Return a single NHL game by its id.
// @Tags        NHL Games
// @Produce     json
// @Param       id path int true "NHL game id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/nhl/games/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".nhl_game.GetByID")
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

// parseIntPtr parses an integer query param; unparseable/empty yields nil (filter skipped).
func parseIntPtr(raw string) *int {
	if raw == "" {
		return nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}

	return &v
}
