// Package news wires the news HTTP endpoints.
package news

import (
	"net/http"
	"strconv"

	"go-espn-api/infras/otel"
	"go-espn-api/internal/domains/news/repository"
	"go-espn-api/internal/domains/news/service"
	"go-espn-api/shared/constant"
	"go-espn-api/transport/http/response"

	"github.com/go-chi/chi/v5"
)

// Handler serves the news endpoints.
type Handler struct {
	service service.News
	otel    otel.Otel
}

// New creates a new news handler.
func New(service service.News, otl otel.Otel) Handler {
	return Handler{service: service, otel: otl}
}

// Router registers news routes.
func (h *Handler) Router(router chi.Router) {
	router.Route("/news", func(rg chi.Router) {
		rg.Get("/", h.List)
		rg.Get("/{id}", h.GetByID)
	})
}

// List returns a paginated list of news articles.
//
// @Summary     List news articles
// @Description Return a paginated list of news articles.
// @Tags        News
// @Produce     json
// @Param       sport query string false "Filter by sport slug"
// @Param       league query string false "Filter by league slug"
// @Param       date_from query string false "Filter by start date (YYYY-MM-DD)"
// @Param       search query string false "Search term"
// @Param       ordering query string false "Ordering field"
// @Param       page query int false "Page number"
// @Success     200 {object} response.Paginated
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/news/ [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".news.List")
	defer scope.End()

	page := response.ParsePage(r)
	q := r.URL.Query()

	filter := repository.ListFilter{
		Sport:    q.Get("sport"),
		League:   q.Get("league"),
		DateFrom: q.Get("date_from"),
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

// GetByID returns a single news article by id.
//
// @Summary     Get news article by id
// @Description Return a single news article by its id.
// @Tags        News
// @Produce     json
// @Param       id path int true "News article id"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} response.Error
// @Failure     403 {object} response.Error
// @Failure     404 {object} response.Error
// @Security    X-API-Key
// @Router      /api/v1/news/{id}/ [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, scope := h.otel.NewScope(r.Context(), constant.OtelHandlerScopeName, constant.OtelHandlerScopeName+".news.GetByID")
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
