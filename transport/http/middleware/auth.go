package middleware

import (
	"context"
	"go-espn-api/config"
	"go-espn-api/infras/jwt"
	"go-espn-api/infras/otel"
	"go-espn-api/permissions"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
	"go-espn-api/transport/http/response"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/rs/zerolog/log"
)

// SkipAuthKey is the context key to skip authentication for a route.
type SkipAuthKey string

// PermissionsKey is the context key for storing user permissions.
type PermissionsKey string

// Auth defines the interface for authentication middleware
type Auth interface {
	Auth(http.Handler) http.Handler
	APIKey(http.Handler) http.Handler
	APIKeyRequired(http.Handler) http.Handler
}

// Role defines the interface for role-based access control middleware
type Role interface {
	RBAC(http.Handler) http.Handler
}

// AuthRole combines all middleware interfaces
type AuthRole interface {
	Auth
	Role
}

// authRoleImpl implements the AuthRole interface
type authRoleImpl struct {
	jwtService jwt.JWT
	otel       otel.Otel
	permission *permissions.PermissionData
	cfg        *config.Config
}

// NewAuthRoleMiddleware creates a new middleware instance
func NewAuthRoleMiddleware(jwtService jwt.JWT, otel otel.Otel, permissions *permissions.PermissionData, cfg *config.Config) AuthRole {
	return &authRoleImpl{
		jwtService: jwtService,
		otel:       otel,
		permission: permissions,
		cfg:        cfg,
	}
}

// Auth validates JWT tokens
// Requires valid authentication for all requests
func (m *authRoleImpl) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		_, scope := m.otel.NewScope(ctx, constant.OtelHandlerScopeName, "auth.middleware")

		skip, _ := request.Context().Value(SkipAuthKey("skip")).(bool)

		if skip {
			scope.End()
			next.ServeHTTP(writer, request)

			return
		}

		// Check if this endpoint should skip authentication based on permissions config
		if m.permission != nil {
			rctx := chi.RouteContext(ctx)
			method := request.Method
			path := rctx.Routes.Find(chi.NewRouteContext(), method, request.URL.Path)
			permission := m.permission.FindPermissions(path, method)

			if permission.Skip {
				scope.End()
				next.ServeHTTP(writer, request)

				return
			}
		}

		rctx := chi.RouteContext(ctx)
		method := request.Method
		path := rctx.Routes.Find(chi.NewRouteContext(), method, request.URL.Path)

		scope.SetAttributes(map[string]any{
			"middleware.type": "auth",
			"http.path":       path,
			"http.method":     method,
		})

		authHeader := request.Header.Get(constant.RequestHeaderAuthorization)
		if authHeader == "" {
			err := failure.Unauthorized("Missing authorization header")
			response.WithError(writer, err)

			scope.TraceError(err)
			scope.End()

			return
		}

		tokenString, err := jwt.ExtractTokenFromHeader(authHeader)
		if err != nil {
			err := failure.Unauthorized("Invalid authorization header format")
			response.WithError(writer, err)

			scope.TraceError(err)
			scope.End()

			return
		}

		claims, err := m.jwtService.ValidateToken(ctx, tokenString)
		if err != nil {
			errKey := errkey.ErrTokenInvalid
			message := "Token validation failed"

			errStr := err.Error()
			switch {
			case strings.Contains(errStr, "token_expired"):
				errKey = errkey.ErrTokenExpired
				message = "Token has expired"
			case strings.Contains(errStr, "invalid_token"):
				errKey = errkey.ErrTokenInvalid
				message = "Invalid token"
			case strings.Contains(errStr, "invalid_claim"):
				errKey = errkey.ErrInvalidClaim
				message = "Invalid token claims"
			case strings.Contains(errStr, "token_parse_failed"):
				errKey = errkey.ErrTokenInvalid
				message = "Token parse failed"
			case strings.Contains(errStr, "token_missing_kid"):
				errKey = errkey.ErrTokenInvalid
				message = "Token missing key ID"
			case strings.Contains(errStr, "token_invalid_claims"):
				errKey = errkey.ErrInvalidClaim
				message = "Token invalid claims"
			}

			err := failure.UnauthorizedWithKey(errKey, message)
			response.WithError(writer, err)

			scope.TraceError(err)
			scope.End()

			return
		}

		// Validate that required claims are not empty
		if claims.Subject == "" {
			log.Error().Msg("JWT claims: Subject is empty")

			response.WithError(writer, failure.Unauthorized("Invalid token claims"))
		}

		if claims.Email == "" {
			log.Error().Msg("JWT claims: Email is empty")

			response.WithError(writer, failure.Unauthorized("Invalid token claims"))
		}

		ctx = context.WithValue(ctx, constant.ContextKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, constant.ContextKeyUserEmail, claims.Email)
		ctx = context.WithValue(ctx, constant.ContextKeyUserRole, claims.Role)

		scope.End()

		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// RBAC checks if user has required role
// Requires prior authentication via Auth middleware
func (m *authRoleImpl) RBAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		_, scope := m.otel.NewScope(ctx, constant.OtelHandlerScopeName, "rbac.middleware")

		skip, _ := request.Context().Value(SkipAuthKey("skip")).(bool)
		if skip {
			scope.End()
			next.ServeHTTP(writer, request)

			return
		}

		if m.permission == nil {
			scope.End()
			response.WithError(writer, failure.ForbiddenError)

			return
		}

		if m.permission.Skip {
			scope.End()
			next.ServeHTTP(writer, request)

			return
		}

		rctx := chi.RouteContext(request.Context())
		path := rctx.Routes.Find(chi.NewRouteContext(), request.Method, request.URL.Path)
		permission := m.permission.FindPermissions(path, request.Method)

		if permission.Skip {
			scope.End()
			next.ServeHTTP(writer, request)

			return
		}

		// Get user role from context
		userRole, _ := ctx.Value(constant.ContextKeyUserRole).(string)

		// Check if user role is allowed (permissions field now contains roles)
		if len(permission.Permissions) > 0 {
			if !slices.Contains(permission.Permissions, userRole) {
				err := failure.ForbiddenError
				scope.TraceError(err)
				scope.SetAttributes(map[string]any{
					"user_role":     userRole,
					"allowed_roles": permission.Permissions,
					"reason":        "role_not_allowed",
				})
				scope.End()
				response.WithError(writer, err)

				return
			}
		}

		scope.End()
		next.ServeHTTP(writer, request)
	})
}

// APIKeyRequired enforces a valid X-API-Key header on every request, matching
// the Django service which mounts the public API behind a mandatory API key
// (no JWT). Requests with a missing or invalid key are rejected with 403.
func (m *authRoleImpl) APIKeyRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		_, scope := m.otel.NewScope(ctx, constant.OtelHandlerScopeName, "api_key_required.middleware")

		apiKey := request.Header.Get(constant.RequestHeaderAPIKey)

		if apiKey == "" || apiKey != m.cfg.App.APIKey {
			err := failure.ForbiddenError

			response.WithError(writer, err)

			scope.TraceError(err)
			scope.End()

			return
		}

		ctx = context.WithValue(ctx, SkipAuthKey("skip"), true)

		scope.End()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// APIKey for internal service-to-service authentication using API key
func (m *authRoleImpl) APIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		_, scope := m.otel.NewScope(ctx, constant.OtelHandlerScopeName, "api_key.middleware")

		ctx = context.WithValue(ctx, SkipAuthKey("skip"), false)
		apiKey := request.Header.Get(constant.RequestHeaderAPIKey)

		if apiKey == "" {
			scope.SetAttribute("http.source", "client")
			scope.End()
			next.ServeHTTP(writer, request.WithContext(ctx))

			return
		}

		scope.SetAttribute("http.source", "internal")

		if apiKey != m.cfg.App.APIKey {
			err := failure.ForbiddenError

			response.WithError(writer, failure.ForbiddenError)

			scope.TraceError(err)
			scope.End()

			return
		}

		ctx = context.WithValue(ctx, SkipAuthKey("skip"), true)

		scope.End()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}
