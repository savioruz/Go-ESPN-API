// Package http provides HTTP server and routing utilities.
//
// nolint:revive
package http

import (
	"fmt"
	"go-espn-api/config"
	"go-espn-api/infras/postgres"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/logger"
	httpMiddleware "go-espn-api/transport/http/middleware"
	"go-espn-api/transport/http/response"
	"go-espn-api/transport/http/router"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/rs/zerolog/log"

	scalargo "github.com/bdpiprava/scalar-go"
	"github.com/bdpiprava/scalar-go/model"
)

// ServerState represents the state of the HTTP server.
type ServerState int

const (
	// ServerStateReady indicates the server is ready to accept connections.
	ServerStateReady ServerState = iota + 1
	// ServerStateInGracePeriod indicates the server is in grace period.
	ServerStateInGracePeriod
	// ServerStateInCleanupPeriod indicates the server is in cleanup period.
	ServerStateInCleanupPeriod

	// RouteHealthCheck is the health check endpoint.
	RouteHealthCheck = "/health"
	// RouteHealthCheckZ is the Django-compatible health check endpoint.
	RouteHealthCheckZ = "/healthz"
	// RouteOpenAPIDocs is the OpenAPI documentation endpoint.
	RouteOpenAPIDocs = "/docs"
)

// HTTP represents the HTTP server configuration and routing.
type HTTP struct {
	Config         *config.Config
	Router         router.Router
	State          ServerState
	mux            *chi.Mux
	DB             *postgres.Connection
	appMiddleware  httpMiddleware.AppMiddleware
	authMiddleware httpMiddleware.AuthRole
}

// New creates a new HTTP server instance.
func New(cfg *config.Config, r router.Router, db *postgres.Connection, appMiddleware httpMiddleware.AppMiddleware, authMiddleware httpMiddleware.AuthRole) *HTTP {
	return &HTTP{
		Config:         cfg,
		Router:         r,
		DB:             db,
		appMiddleware:  appMiddleware,
		authMiddleware: authMiddleware,
	}
}

// Serve starts the HTTP server and listens for incoming requests.
func (h *HTTP) Serve() {
	h.setup()

	log.Info().Str("port", h.Config.Server.Port).Msg("Starting up HTTP server.")

	if err := http.ListenAndServe(net.JoinHostPort("0.0.0.0", h.Config.Server.Port), h.mux); err != nil {
		logger.ErrorWithStack(err)
	}
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.setup()
	h.mux.ServeHTTP(w, r)
}

func (h *HTTP) setup() {
	h.setupChi()
	h.setupMiddlewares()
	h.setupNotFoundHandler()
	h.setupRoutes()
	h.setupOpenAPIDocs()
	h.setupGracefulShutdown()
	h.State = ServerStateReady
}

func (h *HTTP) setupChi() {
	h.mux = chi.NewRouter()
}

func (h *HTTP) setupRoutes() {
	// Public health endpoints. Django exposes /healthz (no trailing slash);
	// /health is kept for backward compatibility with oil tooling.
	h.mux.Get(RouteHealthCheck, h.healthCheck)
	h.mux.Get(RouteHealthCheckZ, h.healthCheck)

	// The DRF-compatible API is mounted under /api/v1 and protected by ONLY the
	// mandatory X-API-Key check (no JWT Auth/RBAC), matching the Django service.
	h.mux.
		With(h.authMiddleware.APIKeyRequired).
		Group(func(rc chi.Router) {
			h.Router.SetupRoutes(rc)
		})
}

func (h *HTTP) setupMiddlewares() {
	h.setupLogger()
	h.setupCORS()
	h.setupServerState()
	h.setupCleanPaths()
	h.setupIdentity()
	h.setupRecover()
	h.setupTracing()
	h.setupRateLimit()

	h.logCORSConfigInfo()
}

func (h *HTTP) setupCleanPaths() {
	h.mux.Use(middleware.CleanPath)
}

func (h *HTTP) setupIdentity() {
	h.mux.Use(middleware.RequestID)
	h.mux.Use(middleware.ClientIPFromRemoteAddr)
}

func (h *HTTP) setupRecover() {
	h.mux.Use(middleware.Recoverer)
}

func (h *HTTP) setupServerState() {
	h.mux.Use(h.serverStateMiddleware)
}

func (h *HTTP) setupLogger() {
	h.mux.Use(h.customJSONLogger())
}

func (h *HTTP) customJSONLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a wrapped response writer to capture status code and bytes
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				duration := time.Since(start)

				// Build log event with comprehensive request/response metadata
				event := log.Info().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("query", r.URL.RawQuery).
					Str("proto", r.Proto).
					Str("scheme", r.URL.Scheme).
					Str("remote_addr", r.RemoteAddr).
					Str("user_agent", r.UserAgent()).
					Str("referer", r.Referer()).
					Str("request_id", middleware.GetReqID(r.Context())).
					Int("status", ww.Status()).
					Int("bytes_written", ww.BytesWritten()).
					Dur("duration", duration).
					Str("content_type", r.Header.Get("Content-Type"))

				// Add content length if present
				if r.ContentLength > 0 {
					event = event.Int64("content_length", r.ContentLength)
				}

				// Add custom headers if present
				if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
					event = event.Str("x_forwarded_for", xff)
				}

				if xri := r.Header.Get("X-Real-IP"); xri != "" {
					event = event.Str("x_real_ip", xri)
				}

				event.Msg("HTTP Request")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func (h *HTTP) setupTracing() {
	h.mux.Use(h.appMiddleware.Tracing)
}

func (h *HTTP) setupRateLimit() {
	rateLimitConfig := h.Config.App.RateLimiter

	if rateLimitConfig.Enable {
		rateLimitMiddleware := h.appMiddleware.RateLimit()
		h.mux.Use(rateLimitMiddleware)

		log.Info().
			Bool("enabled", rateLimitConfig.Enable).
			Int("max_requests", rateLimitConfig.MaxRequests).
			Int("window_seconds", rateLimitConfig.WindowSeconds).
			Str("storage", "cache-redis").
			Msg("Rate limiting enabled with Redis cache storage")
	} else {
		log.Info().Msg("Rate limiting disabled")
	}
}

func (h *HTTP) setupOpenAPIDocs() {
	if h.Config.Server.Env == constant.ServerEnvDevelopment {
		h.mux.Get("/docs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			html, err := scalargo.NewV2(
				scalargo.WithSpecDir("docs"),
				scalargo.WithBaseFileName("openapi.json"),
				scalargo.WithMetaDataOpts(
					scalargo.WithTitle(h.Config.App.Name+" API Reference"),
				),
				scalargo.WithSpecModifier(func(spec *model.Spec) *model.Spec {
					spec.Info.Version = h.Config.Server.Env
					buildTime := time.Now().Format(constant.DateFormat)
					specBuildInfo := "**Last Updated:** " + buildTime
					spec.Info.Description = &specBuildInfo

					return spec
				}),
				scalargo.WithCustomHeadJS(`
					(
					function() {
						var link1 = document.createElement('link');
						link1.rel = 'icon';
						link1.href = 'https://scalar.com/favicon.svg';
						document.head.appendChild(link1);

						var link2 = document.createElement('link');
						link2.rel = 'icon alternate';
						link2.href = 'https://scalar.com/favicon.png';
						document.head.appendChild(link2);
					}
					)();
				`),
				scalargo.WithAuthenticationOpts(
					scalargo.WithSecurityScheme("api_key",
						scalargo.APIKeyScheme("X-API-Key", scalargo.APIKeyLocationHeader, "default-key"),
					),
					scalargo.WithSecurityScheme("bearer_auth",
						scalargo.BearerScheme("default-token"),
					),
					scalargo.WithPreferredSecurityScheme("bearer_auth"),
				),
			)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, html)
		}))

		log.Info().Str("url", fmt.Sprintf("http://localhost:%s%s", h.Config.Server.Port, RouteOpenAPIDocs)).Msg("OpenAPI docs available at")

		return
	}
}

func (h *HTTP) setupGracefulShutdown() {
	serverStateCh := make(chan os.Signal, 1)

	signal.Notify(serverStateCh, os.Interrupt, syscall.SIGTERM)

	go h.respondToSigterm(serverStateCh)
}

func (h *HTTP) respondToSigterm(done chan os.Signal) {
	<-done

	defer os.Exit(0)

	if h.Config.Server.Env == constant.ServerEnvDevelopment {
		log.Warn().Msg("Received SIGTERM. Shutting down now.")

		return
	}

	shutdownConfig := h.Config.Server.Shutdown

	log.Info().Msg("Received SIGTERM.")
	log.Info().Int64("seconds", shutdownConfig.GracePeriodSeconds).Msg("Entering grace period.")

	h.State = ServerStateInGracePeriod

	time.Sleep(time.Duration(shutdownConfig.GracePeriodSeconds) * time.Second)

	log.Info().Int64("seconds", shutdownConfig.CleanupPeriodSeconds).Msg("Entering cleanup period.")

	h.State = ServerStateInCleanupPeriod

	time.Sleep(time.Duration(shutdownConfig.CleanupPeriodSeconds) * time.Second)

	log.Info().Msg("Cleaning up completed. Shutting down now.")
}

func (h *HTTP) serverStateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch h.State {
		case ServerStateReady:
			// Server is ready to serve, don't do anything.
			next.ServeHTTP(writer, request)
		case ServerStateInGracePeriod:
			// Server is in grace period. Issue a warning message and continue
			// serving as usual.
			log.Warn().Msg("SERVER IS IN GRACE PERIOD")
			next.ServeHTTP(writer, request)
		case ServerStateInCleanupPeriod:
			// Server is in cleanup period. Stop the request from actually
			// invoking any domain services and respond appropriately.
			response.WithPreparingShutdown(writer)
		}
	})
}

func (h *HTTP) setupCORS() {
	corsConfig := h.Config.App.CORS
	if corsConfig.Enable {
		h.mux.Use(cors.Handler(cors.Options{
			AllowedOrigins:   corsConfig.AllowedOrigins,
			AllowedMethods:   corsConfig.AllowedMethods,
			AllowedHeaders:   corsConfig.AllowedHeaders,
			AllowCredentials: corsConfig.AllowCredentials,
			MaxAge:           corsConfig.MaxAgeSeconds,
		}))
	}
}

func (h *HTTP) logCORSConfigInfo() {
	corsConfig := h.Config.App.CORS
	corsHeaderInfo := "CORS Header"

	if corsConfig.Enable {
		log.Info().Msg("CORS Headers and Handlers are enabled.")
		log.Info().Str(corsHeaderInfo, fmt.Sprintf("Access-Control-Allow-Credentials: %t", corsConfig.AllowCredentials)).Msg("")
		log.Info().Str(corsHeaderInfo, "Access-Control-Allow-Headers: "+strings.Join(corsConfig.AllowedHeaders, ", ")).Msg("")
		log.Info().Str(corsHeaderInfo, "Access-Control-Allow-Methods: "+strings.Join(corsConfig.AllowedMethods, ", ")).Msg("")
		log.Info().Str(corsHeaderInfo, "Access-Control-Allow-Origin: "+strings.Join(corsConfig.AllowedOrigins, ", ")).Msg("")
		log.Info().Str(corsHeaderInfo, fmt.Sprintf("Access-Control-Max-Age: %d", corsConfig.MaxAgeSeconds)).Msg("")
	} else {
		log.Info().Msg("CORS Headers are disabled.")
	}
}

func (h *HTTP) setupNotFoundHandler() {
	h.mux.NotFound(func(writer http.ResponseWriter, _ *http.Request) {
		response.WithMessage(writer, http.StatusNotFound, string(errkey.ErrNotFound))
	})
}

// HealthCheck performs a health check on the server.
// @Summary Health Check
// @Description Health Check APIEndpoint
// @Tags service
// @Produce json
// @Accept json
// @Success 200 {object} response.Message
// @Router /health [get]
func (h *HTTP) healthCheck(writer http.ResponseWriter, _ *http.Request) {
	if err := h.DB.Read.Ping(); err != nil {
		logger.ErrorWithStack(err)
		response.WithUnhealthy(writer)

		return
	}

	response.WithMessage(writer, http.StatusOK, "ok")
}
