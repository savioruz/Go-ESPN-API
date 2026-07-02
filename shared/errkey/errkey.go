// Package errkey provides a standardized way to define and manage error keys for API responses.
package errkey

import (
	"fmt"
	"strings"
	"unicode"
)

// ErrorKey represents a unique error identifier that can be translated by the frontend
type ErrorKey string

// Common error keys
const (
	// Validation errors
	ErrValidationFailed   ErrorKey = "validation.failed"
	ErrInvalidPageParam   ErrorKey = "validation.invalid_page"
	ErrInvalidLimitParam  ErrorKey = "validation.invalid_limit"
	ErrRequiredField      ErrorKey = "validation.required"
	ErrInvalidFormat      ErrorKey = "validation.invalid_format"
	ErrMinLength          ErrorKey = "validation.min"
	ErrMaxLength          ErrorKey = "validation.max"
	ErrGreaterThanEqual   ErrorKey = "validation.gte"
	ErrLessThanEqual      ErrorKey = "validation.lte"
	ErrOneOf              ErrorKey = "validation.oneof"
	ErrInvalidURL         ErrorKey = "validation.url"
	ErrInvalidEmail       ErrorKey = "validation.email"
	ErrInvalidImageFormat ErrorKey = "validation.mimetypes"
	ErrFileTooLarge       ErrorKey = "validation.maxfilesize"
	ErrEmptyFile          ErrorKey = "validation.empty"
	ErrMissingFile        ErrorKey = "validation.missing_file"
	ErrDive               ErrorKey = "validation.dive"

	// Authentication & Authorization errors
	ErrUnauthorized           ErrorKey = "auth.unauthorized"
	ErrForbidden              ErrorKey = "auth.forbidden"
	ErrInvalidCredentials     ErrorKey = "auth.invalid_credentials"
	ErrTokenExpired           ErrorKey = "auth.token_expired"
	ErrTokenInvalid           ErrorKey = "auth.token_invalid"
	ErrResourceRestricted     ErrorKey = "auth.resource_restricted"
	ErrInsufficientPermission ErrorKey = "auth.insufficient_permission"
	ErrEmailAlreadyExists     ErrorKey = "auth.email_already_exists"
	ErrAccountDeactivated     ErrorKey = "auth.account_deactivated"
	ErrJWKSFetchFailed        ErrorKey = "auth.jwks_fetch_failed"
	ErrInvalidToken           ErrorKey = "auth.invalid_token"
	ErrInvalidClaim           ErrorKey = "auth.invalid_claim"
	ErrAuthHeaderMissing      ErrorKey = "auth.header_missing"
	ErrInvalidAuthHeader      ErrorKey = "auth.invalid_header"

	// Resource errors
	ErrNotFound       ErrorKey = "resource.not_found"
	ErrAlreadyExists  ErrorKey = "resource.already_exists"
	ErrConflict       ErrorKey = "resource.conflict"
	ErrResourceLocked ErrorKey = "resource.locked"

	// Database errors
	ErrDatabaseQuery       ErrorKey = "database.query_failed"
	ErrDatabaseConnection  ErrorKey = "database.connection_failed"
	ErrDatabaseTransaction ErrorKey = "database.transaction_failed"
	ErrDatabaseConstraint  ErrorKey = "database.constraint_violated"

	// Server errors
	ErrInternalServer     ErrorKey = "server.internal_error"
	ErrServiceUnavailable ErrorKey = "server.service_unavailable"
	ErrTimeout            ErrorKey = "server.timeout"
	ErrRateLimitExceeded  ErrorKey = "server.rate_limit_exceeded"
	ErrMaintenanceMode    ErrorKey = "server.maintenance_mode"
	ErrShuttingDown       ErrorKey = "server.shutting_down"

	// External service errors
	ErrExternalService ErrorKey = "external.service_error"
	ErrS3Upload        ErrorKey = "external.s3_upload_failed"
	ErrS3Delete        ErrorKey = "external.s3_delete_failed"
	ErrCacheOperation  ErrorKey = "external.cache_operation_failed"

	// Gallery-specific errors
	ErrGalleryNotFound       ErrorKey = "gallery.not_found"
	ErrGalleryCreateFailed   ErrorKey = "gallery.create_failed"
	ErrGalleryUpdateFailed   ErrorKey = "gallery.update_failed"
	ErrGalleryDeleteFailed   ErrorKey = "gallery.delete_failed"
	ErrGalleryListFailed     ErrorKey = "gallery.list_failed"
	ErrImageUploadFailed     ErrorKey = "gallery.image_upload_failed"
	ErrImageDeleteFailed     ErrorKey = "gallery.image_delete_failed"
	ErrImageProcessingFailed ErrorKey = "gallery.image_processing_failed"

	// Todo-specific errors
	ErrTodoNotFound     ErrorKey = "todo.not_found"
	ErrTodoCreateFailed ErrorKey = "todo.create_failed"
	ErrTodoUpdateFailed ErrorKey = "todo.update_failed"
	ErrTodoDeleteFailed ErrorKey = "todo.delete_failed"
	ErrTodoListFailed   ErrorKey = "todo.list_failed"

	// Userprofile-specific errors
	ErrUserprofileNotFound     ErrorKey = "userprofile.not_found"
	ErrUserprofileCreateFailed ErrorKey = "userprofile.create_failed"
	ErrUserprofileUpdateFailed ErrorKey = "userprofile.update_failed"
)

// ValidationTagToKey maps validation tags to error keys
var ValidationTagToKey = map[string]ErrorKey{
	"required":    ErrRequiredField,
	"min":         ErrMinLength,
	"max":         ErrMaxLength,
	"gte":         ErrGreaterThanEqual,
	"lte":         ErrLessThanEqual,
	"oneof":       ErrOneOf,
	"url":         ErrInvalidURL,
	"email":       ErrInvalidEmail,
	"mimetypes":   ErrInvalidImageFormat,
	"maxfilesize": ErrFileTooLarge,
	"empty":       ErrEmptyFile,
	"dive":        ErrDive,
}

// String returns the string representation of the error key
func (e ErrorKey) String() string {
	return string(e)
}

// ErrorWithDetails creates an error key with additional details for debugging (optional)
type ErrorWithDetails struct {
	Key     ErrorKey               `json:"key"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewError creates a new error with just a key
func NewError(key ErrorKey) *ErrorWithDetails {
	return &ErrorWithDetails{
		Key: key,
	}
}

// WithDetail adds a detail to the error
func (e *ErrorWithDetails) WithDetail(key string, value interface{}) *ErrorWithDetails {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}

	e.Details[key] = value

	return e
}

// ToSnakeCase converts a field name from PascalCase/camelCase to snake_case
func ToSnakeCase(str string) string {
	var result strings.Builder

	for i, r := range str {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}

			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// FormatFieldError creates a field-specific validation error key
// e.g., "Title" + "required" -> "validation.required.title"
func FormatFieldError(tag string, fieldName string) ErrorKey {
	baseKey, ok := ValidationTagToKey[tag]
	if !ok {
		baseKey = ErrValidationFailed
	}

	snakeField := ToSnakeCase(fieldName)

	return ErrorKey(fmt.Sprintf("%s.%s", baseKey, snakeField))
}
