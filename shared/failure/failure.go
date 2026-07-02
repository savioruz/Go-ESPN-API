// Package failure provides error types and helper functions for handling HTTP errors.
package failure

import (
	"errors"
	"go-espn-api/shared/errkey"
	"net/http"
)

// Failure is a wrapper for error messages and codes using standard HTTP response codes.
type Failure struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Key     errkey.ErrorKey `json:"key,omitempty"` // Error key for frontend translation
}

// ValidationFieldError represents a single field validation error
type ValidationFieldError struct {
	Field   string          `json:"field"`           // Field name (e.g., "title", "images[0]")
	Message string          `json:"message"`         // Human-readable message
	Key     errkey.ErrorKey `json:"key"`             // Machine-readable error key (e.g., "validation.required.title")
	Param   string          `json:"param,omitempty"` // Validation parameter if applicable (e.g., "3" for min=3)
}

// ValidationError represents a validation error with multiple field errors
type ValidationError struct {
	Code   int                    `json:"code"`
	Fields []ValidationFieldError `json:"fields"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if len(e.Fields) > 0 {
		return e.Fields[0].Message
	}

	return "validation failed"
}

// InvalidPageParam is a predefined error for invalid page parameter in pagination.
var InvalidPageParam = &Failure{Code: http.StatusBadRequest, Message: "invalid page parameter", Key: errkey.ErrInvalidPageParam}

// InvalidLimitParam is a predefined error for invalid limit parameter in pagination.
var InvalidLimitParam = &Failure{Code: http.StatusBadRequest, Message: "invalid limit parameter", Key: errkey.ErrInvalidLimitParam}

// ForbiddenError is a predefined error for forbidden access due to insufficient permissions.
var ForbiddenError = &Failure{Code: http.StatusForbidden, Message: "You don't have the required permissions", Key: errkey.ErrForbidden}

// ResourceRestrictedError is a predefined error for forbidden access to a specific resource due to insufficient permissions.
var ResourceRestrictedError = &Failure{Code: http.StatusForbidden, Message: "You don't have permission to access this resource", Key: errkey.ErrResourceRestricted}

// Error returns the error code and message in a formatted string.
func (e *Failure) Error() string {
	return e.Message
}

// BadRequest returns a new Failure with code for bad requests.
func BadRequest(err error) error {
	if err != nil {
		return &Failure{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
	}

	return nil
}

// BadRequestFromString returns a new Failure with code for bad requests with message set from string.
func BadRequestFromString(msg string) error {
	return &Failure{
		Code:    http.StatusBadRequest,
		Message: msg,
	}
}

// UnprocessableEntity returns a new Failure with code for unprocessable entity and message derived from an error interface.
func UnprocessableEntity(err error) error {
	if err != nil {
		return &Failure{
			Code:    http.StatusUnprocessableEntity,
			Message: err.Error(),
		}
	}

	return nil
}

// UnprocessableEntityFromString returns a new Failure with code for unprocessable entity with message set from string.
func UnprocessableEntityFromString(msg string) error {
	return &Failure{
		Code:    http.StatusUnprocessableEntity,
		Message: msg,
	}
}

// Unauthorized returns a new Failure with code for unauthorized requests.
func Unauthorized(msg string) error {
	return &Failure{
		Code:    http.StatusUnauthorized,
		Message: msg,
	}
}

// InternalError returns a new Failure with code for internal error and message derived from an error interface.
func InternalError(err error) error {
	if err != nil {
		return &Failure{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}
	}

	return nil
}

// Unimplemented returns a new Failure with code for unimplemented method.
func Unimplemented(methodName string) error {
	return &Failure{
		Code:    http.StatusNotImplemented,
		Message: methodName,
	}
}

// NotFound returns a new Failure with code for entity not found.
func NotFound(entityName string) error {
	return &Failure{
		Code:    http.StatusNotFound,
		Message: entityName,
	}
}

// Conflict returns a new Failure with code for conflict situations.
func Conflict(message string) error {
	return &Failure{
		Code:    http.StatusConflict,
		Message: message,
	}
}

// Forbidden returns a new Failure with code for forbidden access and message derived from an error interface.
func Forbidden(msg string) error {
	return &Failure{
		Code:    http.StatusForbidden,
		Message: msg,
	}
}

// GetCode returns the error code of an error interface.
func GetCode(err error) int {
	// Check for ValidationError first
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		return valErr.Code
	}

	var fail *Failure
	if errors.As(err, &fail) {
		return fail.Code
	}

	return http.StatusInternalServerError
}

// GetKey returns the error key of an error interface.
func GetKey(err error) errkey.ErrorKey {
	// Check for ValidationError first (has multiple field errors)
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		// For multi-field validation errors, return generic validation.failed
		// The actual field-level keys are in the Fields array
		return errkey.ErrValidationFailed
	}

	var fail *Failure
	if errors.As(err, &fail) {
		// If the key is empty, return a default based on the status code
		if fail.Key == "" {
			// Return appropriate default key based on status code
			switch {
			case fail.Code == http.StatusUnprocessableEntity:
				return errkey.ErrValidationFailed
			case fail.Code == http.StatusBadRequest:
				return errkey.ErrValidationFailed
			case fail.Code == http.StatusNotFound:
				return errkey.ErrNotFound
			case fail.Code == http.StatusUnauthorized:
				return errkey.ErrUnauthorized
			case fail.Code == http.StatusForbidden:
				return errkey.ErrForbidden
			case fail.Code >= http.StatusInternalServerError:
				return errkey.ErrInternalServer
			default:
				return errkey.ErrInternalServer
			}
		}

		return fail.Key
	}

	return errkey.ErrInternalServer
}

// NewWithKey creates a new Failure with a key, code, and message.
func NewWithKey(key errkey.ErrorKey, code int, message string) error {
	return &Failure{
		Code:    code,
		Message: message,
		Key:     key,
	}
}

// BadRequestWithKey returns a new Failure with bad request code and error key.
func BadRequestWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusBadRequest,
		Message: message,
		Key:     key,
	}
}

// NotFoundWithKey returns a new Failure with not found code and error key.
func NotFoundWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusNotFound,
		Message: message,
		Key:     key,
	}
}

// InternalErrorWithKey returns a new Failure with internal error code and error key.
func InternalErrorWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusInternalServerError,
		Message: message,
		Key:     key,
	}
}

// UnauthorizedWithKey returns a new Failure with unauthorized code and error key.
func UnauthorizedWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusUnauthorized,
		Message: message,
		Key:     key,
	}
}

// ForbiddenWithKey returns a new Failure with forbidden code and error key.
func ForbiddenWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusForbidden,
		Message: message,
		Key:     key,
	}
}

// ConflictWithKey returns a new Failure with conflict code and error key.
func ConflictWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusConflict,
		Message: message,
		Key:     key,
	}
}

// ServiceUnavailableWithKey returns a new Failure with service unavailable code and error key.
func ServiceUnavailableWithKey(key errkey.ErrorKey, message string) error {
	return &Failure{
		Code:    http.StatusServiceUnavailable,
		Message: message,
		Key:     key,
	}
}
