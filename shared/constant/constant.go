// Package constant defines a collection of constant values used throughout the application for various purposes
//
//nolint:revive
package constant

import (
	"time"
)

const (
	// ContextGuest is used to indicate the request is made by a guest user without authentication
	ContextGuest = "guest"
)

// Context key types to avoid collisions
type contextKey string

const (
	// ContextKeyUserID is used to store the authenticated user's ID in the request context
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyUserEmail is used to store the authenticated user's email in the request context
	ContextKeyUserEmail contextKey = "user_email"
	// ContextKeyUserRole is used to store the authenticated user's role in the request context
	ContextKeyUserRole contextKey = "user_role"
	// ContextKeyTokenID is used to store the authentication token ID in the request context
	ContextKeyTokenID contextKey = "token_id"
)

const (
	// RoleSuperAdmin is used to indicate a user with the highest level of permissions, typically with access to all resources and administrative functions
	RoleSuperAdmin = "superadmin"
	// RoleAdmin is used to indicate a user with administrative permissions, typically with access to manage resources and perform administrative tasks
	RoleAdmin = "admin"
	// RoleUser is used to indicate a regular user with standard permissions, typically with access to their own resources and limited functionality
	RoleUser = "user"
)

const (
	// RequestParamPage is used to specify the page number for paginated requests
	RequestParamPage = "page"
	// RequestParamLimit is used to specify the number of items per page for paginated requests
	RequestParamLimit = "limit"
	// RequestParamSortBy is used to specify the field by which to sort results in a request
	RequestParamSortBy = "sort_by"
	// RequestParamSortDir is used to specify the direction (ascending or descending) for sorting results in a request
	RequestParamSortDir = "sort_dir"
)

const (
	// RequestParamID is used to specify the unique identifier for a resource in a request, typically used in URL path parameters (e.g., /users/{id})
	RequestParamID = "id"
	// RequestMaxMemory is used to specify the maximum amount of memory (in bytes) that a request can consume, typically used for limiting the size of request bodies (e.g., for file uploads)
	RequestMaxMemory = 1 << 20
)

const (
	// DefaultValuePage is used as the default page number for paginated requests when the page parameter is not provided or invalid
	DefaultValuePage = 1
	// DefaultValueLimit is used as the default number of items per page for paginated requests when the limit parameter is not provided or invalid
	DefaultValueLimit = 10
	// DefaultPageSize is the DRF-compatible page size (matches Django REST framework PAGE_SIZE=25)
	DefaultPageSize = 25
	// DefaultValueSortBy is used as the default field to sort results by in a request when the sort_by parameter is not provided or invalid
	DefaultValueSortBy = "created_at"
	// DefaultValueSortDir is used as the default direction (ascending or descending) for sorting results in a request when the sort_dir parameter is not provided or invalid
	DefaultValueSortDir = "DESC"
)

const (
	// FieldCreatedAt is used to specify the field name for the timestamp when a resource was created, typically used in database models and API responses
	FieldCreatedAt = "created_at"
	// FieldCreatedBy is used to specify the field name for the identifier of the user who created a resource, typically used in database models and API responses
	FieldCreatedBy = "created_by"
	// FieldModifiedAt is used to specify the field name for the timestamp when a resource was last modified, typically used in database models and API responses
	FieldModifiedAt = "modified_at"
	// FieldModifiedBy is used to specify the field name for the identifier of the user who last modified a resource, typically used in database models and API responses
	FieldModifiedBy = "modified_by"
)

const (
	// PqErrorCodeUniqueViolation is used to indicate a unique constraint violation error in PostgreSQL,
	// typically when trying to insert or update a record with a value that already exists in a column with a unique constraint
	PqErrorCodeUniqueViolation = "23505"
	// PqErrorCodeFkViolation is used to indicate a foreign key constraint violation error in PostgreSQL,
	// typically when trying to insert or update a record with a value that does not exist in the referenced table
	// or when trying to delete a record that is referenced by another table
	PqErrorCodeFkViolation = "23503"
)

const (
	// DateFormat is used to specify the standard format for representing date and time values in the application,
	// typically used for parsing and formatting timestamps in API requests and responses
	DateFormat = time.RFC3339
)

const (
	// MinutesToSeconds is used to convert minutes to seconds
	MinutesToSeconds = 60
)

const (
	// OtelServiceScopeName is used to specify the scope name for tracing service layer operations in OpenTelemetry
	OtelServiceScopeName = "service"
	// OtelRepositoryScopeName is used to specify the scope name for tracing repository layer operations in OpenTelemetry
	OtelRepositoryScopeName = "repository"
	// OtelHandlerScopeName is used to specify the scope name for tracing handler layer operations in OpenTelemetry
	OtelHandlerScopeName = "handler"
	// OtelEventScopeName is used to specify the scope name for tracing custom events in OpenTelemetry
	OtelEventScopeName = "event"
	// OtelExternalScopeName is used to specify the scope name for tracing external calls (e.g., HTTP requests to other services) in OpenTelemetry
	OtelExternalScopeName = "external"
	// OtelQueryAttributeKey is used to specify the attribute key for query parameters in OpenTelemetry traces
	OtelQueryAttributeKey = "query"
	// OtelS3ScopeName is used to specify the scope name for tracing S3 operations in OpenTelemetry
	OtelS3ScopeName = "s3"
	// OtelESPNScopeName is used to specify the scope name for tracing ESPN API client operations in OpenTelemetry
	OtelESPNScopeName = "espn_client"
	// OtelNHLScopeName is used to specify the scope name for tracing NHL API client operations in OpenTelemetry
	OtelNHLScopeName = "nhl_client"
)

const (
	// RequestHeaderAuthorization is used to specify the HTTP header key for passing authentication credentials (e.g., Bearer token) in API requests
	RequestHeaderAuthorization = "Authorization"
	// RequestHeaderUserAgent is used to specify the HTTP header key for passing the user agent string in API requests
	RequestHeaderUserAgent = "User-Agent"
	// RequestHeaderContentType is used to specify the HTTP header key for indicating the media type of the request body in API requests
	RequestHeaderContentType = "Content-Type"
	// RequestHeaderRateLimit is used to specify the HTTP header key for indicating the maximum number of requests allowed in a given time frame for rate limiting purposes in API responses
	RequestHeaderRateLimit = "X-RateLimit-Limit"
	// RequestHeaderRateLimitRemaining is used to specify the HTTP header key for indicating the number of requests remaining in the current rate limit window for rate limiting purposes in API responses
	RequestHeaderRateLimitRemaining = "X-RateLimit-Remaining"
	// RequestHeaderRateLimitWindow is used to specify the HTTP header key for indicating the time window (in seconds) for the current rate limit in API responses
	RequestHeaderRateLimitWindow = "X-RateLimit-Window"
	// RequestHeaderRequestID is used to specify the HTTP header key for passing a unique identifier for the request, typically used for tracing and debugging purposes in API requests
	RequestHeaderRequestID = "X-Request-ID"
	// RequestHeaderForwardedFor is used to specify the HTTP header key for passing the original client IP address when the request is forwarded by a proxy or load balancer in API requests
	RequestHeaderForwardedFor = "X-Forwarded-For"
	// RequestHeaderForwardedProto is used to specify the HTTP header key for passing the original request scheme (http/https) when the request is forwarded by a proxy or load balancer
	RequestHeaderForwardedProto = "X-Forwarded-Proto"
	// RequestHeaderRealIP is used to specify the HTTP header key for passing the real client IP address when the request is forwarded by a proxy or load balancer in API requests
	RequestHeaderRealIP = "X-Real-IP"
	// RequestHeaderAPIKey is used to specify the HTTP header key for passing an API key for authentication in API requests
	RequestHeaderAPIKey = "X-API-Key"
)

const (
	// ContentTypeJSON is used for JSON payloads in request and response bodies
	ContentTypeJSON = "application/json"
	// ContentTypeFormURLEncoded is used for simple form submissions without file uploads
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
	// ContentTypeMultipartFormData is used for file uploads and form submissions that include files
	ContentTypeMultipartFormData = "multipart/form-data"
	// FormFile is used as the key for retrieving file data from multipart form data
	FormFile = "file"
)

const (
	// ResponseErrorPrepareShutdown is used to indicate the server is preparing to shut down and cannot accept new requests
	ResponseErrorPrepareShutdown = "SERVER PREPARING TO SHUT DOWN"
	// ResponseErrorUnhealthy is used to indicate the server is unhealthy and cannot process requests
	ResponseErrorUnhealthy = "SERVER UNHEALTHY"
	// ResponseErrorRequestLimitExceeded is used to indicate the client has exceeded the allowed number of requests in a given time frame
	ResponseErrorRequestLimitExceeded = "REQUEST LIMIT EXCEEDED"
)

const (
	// ServerEnvDevelopment is used to indicate the server is running in development environment
	ServerEnvDevelopment = "development"
	// ServerEnvStaging is used to indicate the server is running in staging environment
	ServerEnvStaging = "staging"
	// ServerEnvProduction is used to indicate the server is running in production environment
	ServerEnvProduction = "production"
)

const (
	// Asterix is used in cache keys to represent wildcard
	Asterix = "*"
	// EmptyString is used to represent an empty string value
	EmptyString = ""
)
