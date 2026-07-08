// Package middleware provides HTTP middleware utilities.
package middleware

import (
	"fmt"
	"go-espn-api/config"
	"go-espn-api/infras/otel"
	"go-espn-api/shared/cache"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	otelHTTPScopeName = "http"
)

// AppMiddleware defines the interface for application-level middleware.
type AppMiddleware interface {
	Tracing(http.Handler) http.Handler
	RateLimit() func(http.Handler) http.Handler
}

type appMiddleware struct {
	otel   otel.Otel
	config *config.Config
	cache  cache.RedisCache
}

// NewAppMiddleware creates a new AppMiddleware instance.
func NewAppMiddleware(otel otel.Otel, config *config.Config, cache cache.RedisCache) AppMiddleware {
	return &appMiddleware{
		otel:   otel,
		config: config,
		cache:  cache,
	}
}

func (a *appMiddleware) Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Extract any incoming W3C trace context so this request continues the
		// caller's distributed trace instead of starting an orphan root trace.
		ctx := otelapi.GetTextMapPropagator().Extract(
			request.Context(), propagation.HeaderCarrier(request.Header),
		)

		rctx := chi.RouteContext(ctx)
		method := request.Method
		path := rctx.Routes.Find(chi.NewRouteContext(), method, request.URL.Path)
		userAgent := a.getUA(request)
		spanName := fmt.Sprintf("%s %s", method, path)

		ctx, scope := a.otel.NewScope(ctx, otelHTTPScopeName, spanName)
		defer scope.End()

		// Set comprehensive request attributes
		attrs := map[string]any{
			"app.name":            a.config.App.Name,
			"http.path":           path,
			"http.route":          path,
			"http.method":         method,
			"http.user_agent":     userAgent,
			"http.host":           request.Host,
			"http.source":         request.RemoteAddr,
			"http.request_id":     middleware.GetReqID(ctx),
			"http.client_ip":      a.getClientIP(request),
			"http.proto":          request.Proto,
			"http.content_length": request.ContentLength,
		}

		// Add optional headers if present
		if scheme := request.URL.Scheme; scheme != "" {
			attrs["http.scheme"] = scheme
		}

		if referer := request.Referer(); referer != "" {
			attrs["http.referer"] = referer
		}

		if contentType := request.Header.Get("Content-Type"); contentType != "" {
			attrs["http.content_type"] = contentType
		}

		if queryParams := request.URL.RawQuery; queryParams != "" {
			attrs["http.query_params"] = queryParams
		}

		if xff := request.Header.Get("X-Forwarded-For"); xff != "" {
			attrs["http.x_forwarded_for"] = xff
		}

		scope.SetAttributes(attrs)

		ww := middleware.NewWrapResponseWriter(writer, request.ProtoMajor)
		next.ServeHTTP(ww, request.WithContext(ctx))

		// Set response attributes
		scope.SetAttributes(map[string]any{
			"http.status_code":   ww.Status(),
			"http.response_size": ww.BytesWritten(),
		})
	})
}
