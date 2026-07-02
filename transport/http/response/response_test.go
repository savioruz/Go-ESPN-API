package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"go-espn-api/shared/errkey"
	"go-espn-api/shared/failure"
	"go-espn-api/transport/http/response"
)

func TestWithError_AlwaysReturnsErrorKey(t *testing.T) {
	tests := []struct {
		name          string
		errorCode     int
		errorKey      errkey.ErrorKey
		errorMessage  string
		expectedError string // Always the error key
	}{
		{
			name:          "Internal Server Error",
			errorCode:     500,
			errorKey:      errkey.ErrInternalServer,
			errorMessage:  "database connection failed: timeout at host 10.0.1.5",
			expectedError: string(errkey.ErrInternalServer),
		},
		{
			name:          "Database Query Error",
			errorCode:     500,
			errorKey:      errkey.ErrDatabaseQuery,
			errorMessage:  "failed to execute query: constraint violation details",
			expectedError: string(errkey.ErrDatabaseQuery),
		},
		{
			name:          "Service Unavailable",
			errorCode:     503,
			errorKey:      errkey.ErrServiceUnavailable,
			errorMessage:  "service is temporarily unavailable due to maintenance",
			expectedError: string(errkey.ErrServiceUnavailable),
		},
		{
			name:          "Gallery Not Found",
			errorCode:     404,
			errorKey:      errkey.ErrGalleryNotFound,
			errorMessage:  "gallery with id 'abc-123' not found in database",
			expectedError: string(errkey.ErrGalleryNotFound),
		},
		{
			name:          "Validation Failed",
			errorCode:     400,
			errorKey:      errkey.ErrValidationFailed,
			errorMessage:  "validation failed: title is required, must be at least 3 characters",
			expectedError: string(errkey.ErrValidationFailed),
		},
		{
			name:          "Unauthorized",
			errorCode:     401,
			errorKey:      errkey.ErrUnauthorized,
			errorMessage:  "authentication failed: invalid JWT token signature",
			expectedError: string(errkey.ErrUnauthorized),
		},
		{
			name:          "Forbidden",
			errorCode:     403,
			errorKey:      errkey.ErrForbidden,
			errorMessage:  "access denied: user lacks required permissions",
			expectedError: string(errkey.ErrForbidden),
		},
		{
			name:          "Gallery Create Failed",
			errorCode:     500,
			errorKey:      errkey.ErrGalleryCreateFailed,
			errorMessage:  "failed to create gallery: database insert error",
			expectedError: string(errkey.ErrGalleryCreateFailed),
		},
		{
			name:          "Todo Not Found",
			errorCode:     404,
			errorKey:      errkey.ErrTodoNotFound,
			errorMessage:  "todo item with specified ID does not exist",
			expectedError: string(errkey.ErrTodoNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test error with full message
			var testErr error
			switch tt.errorCode {
			case 500:
				testErr = failure.InternalErrorWithKey(tt.errorKey, tt.errorMessage)
			case 503:
				testErr = failure.ServiceUnavailableWithKey(tt.errorKey, tt.errorMessage)
			case 404:
				testErr = failure.NotFoundWithKey(tt.errorKey, tt.errorMessage)
			case 400:
				testErr = failure.BadRequestWithKey(tt.errorKey, tt.errorMessage)
			case 401:
				testErr = failure.UnauthorizedWithKey(tt.errorKey, tt.errorMessage)
			case 403:
				testErr = failure.ForbiddenWithKey(tt.errorKey, tt.errorMessage)
			default:
				testErr = failure.InternalErrorWithKey(tt.errorKey, tt.errorMessage)
			}

			// Create response recorder
			recorder := httptest.NewRecorder()

			// Call WithError
			response.WithError(recorder, testErr)

			// Check status code
			assert.Equal(t, tt.errorCode, recorder.Code)

			// Parse response
			var errorResponse struct {
				Error *string `json:"error,omitempty"`
			}
			err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
			assert.NoError(t, err)

			// Verify error field always contains the error key
			assert.NotNil(t, errorResponse.Error, "Expected error field to be present")
			assert.Equal(t, tt.expectedError, *errorResponse.Error, "Error field should always contain the error key")

			// Verify the response does NOT contain the detailed message
			assert.NotContains(t, *errorResponse.Error, "database connection", "Should not expose internal details")
			assert.NotContains(t, *errorResponse.Error, "failed to", "Should not expose internal details")
		})
	}
}

func TestWithError_ErrorKeyFormat(t *testing.T) {
	// Test that error keys follow the expected pattern
	testErr := failure.InternalErrorWithKey(errkey.ErrDatabaseQuery, "some internal error message")
	recorder := httptest.NewRecorder()

	response.WithError(recorder, testErr)

	var errorResponse struct {
		Error *string `json:"error,omitempty"`
	}
	err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)

	// Error keys should follow pattern: category.error_name
	assert.Regexp(t, `^[a-z]+\.[a-z_]+$`, *errorResponse.Error, "Error key should follow pattern: category.error_name")
}

func TestWithError_ValidationErrorWithFields(t *testing.T) {
	// Create a validation error with multiple field details
	valErr := &failure.ValidationError{
		Code: 422,
		Fields: []failure.ValidationFieldError{
			{
				Field:   "title",
				Message: "validation.required",
				Key:     "validation.required.title",
				Param:   "",
			},
			{
				Field:   "images[0]",
				Message: "validation.url",
				Key:     "validation.url.images",
				Param:   "",
			},
		},
	}

	recorder := httptest.NewRecorder()
	response.WithError(recorder, valErr)

	// Check status code
	assert.Equal(t, 422, recorder.Code)

	// Parse response - should have errors array, not error field
	var errorResponse struct {
		Errors []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors,omitempty"`
		Error *string `json:"error,omitempty"`
	}
	err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)

	// Verify errors array is present (not error field)
	assert.Nil(t, errorResponse.Error, "Should not have error field for validation errors")
	assert.NotNil(t, errorResponse.Errors, "Should have errors array")
	assert.Len(t, errorResponse.Errors, 2, "Should have 2 field errors")

	// Verify first field error
	assert.Equal(t, "title", errorResponse.Errors[0].Field)
	assert.Equal(t, "validation.required", errorResponse.Errors[0].Message)

	// Verify second field error
	assert.Equal(t, "images[0]", errorResponse.Errors[1].Field)
	assert.Equal(t, "validation.url", errorResponse.Errors[1].Message)
}

func TestValidationErrorExamples(t *testing.T) {
	t.Run("Single field validation error - required field", func(t *testing.T) {
		valErr := &failure.ValidationError{
			Code: http.StatusUnprocessableEntity,
			Fields: []failure.ValidationFieldError{
				{
					Field:   "title",
					Message: "validation.required",
					Key:     errkey.ErrorKey("validation.required.title"),
					Param:   "",
				},
			},
		}

		recorder := httptest.NewRecorder()
		response.WithError(recorder, valErr)

		// Parse and pretty print the response
		var result map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &result)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

		// Should have errors array
		errors := result["errors"].([]interface{})
		assert.Len(t, errors, 1)

		firstError := errors[0].(map[string]interface{})
		assert.Equal(t, "title", firstError["field"])
		assert.Equal(t, "validation.required", firstError["message"])
	})

	t.Run("Single field validation error - invalid URL", func(t *testing.T) {
		valErr := &failure.ValidationError{
			Code: http.StatusUnprocessableEntity,
			Fields: []failure.ValidationFieldError{
				{
					Field:   "images[0]",
					Message: "validation.url",
					Key:     errkey.ErrorKey("validation.url.images"),
					Param:   "",
				},
			},
		}

		recorder := httptest.NewRecorder()
		response.WithError(recorder, valErr)

		var result map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &result)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

		errors := result["errors"].([]interface{})
		assert.Len(t, errors, 1)

		firstError := errors[0].(map[string]interface{})
		assert.Equal(t, "images[0]", firstError["field"])
		assert.Equal(t, "validation.url", firstError["message"])
	})

	t.Run("Single field validation error - min length", func(t *testing.T) {
		valErr := &failure.ValidationError{
			Code: http.StatusUnprocessableEntity,
			Fields: []failure.ValidationFieldError{
				{
					Field:   "title",
					Message: "validation.min",
					Key:     errkey.ErrorKey("validation.min.title"),
					Param:   "3",
				},
			},
		}

		recorder := httptest.NewRecorder()
		response.WithError(recorder, valErr)

		var result map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &result)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

		errors := result["errors"].([]interface{})
		assert.Len(t, errors, 1)

		firstError := errors[0].(map[string]interface{})
		assert.Equal(t, "title", firstError["field"])
		assert.Equal(t, "validation.min", firstError["message"])
	})

	t.Run("Multiple field validation errors", func(t *testing.T) {
		valErr := &failure.ValidationError{
			Code: http.StatusUnprocessableEntity,
			Fields: []failure.ValidationFieldError{
				{
					Field:   "title",
					Message: "validation.required",
					Key:     errkey.ErrorKey("validation.required.title"),
					Param:   "",
				},
				{
					Field:   "images",
					Message: "validation.required",
					Key:     errkey.ErrorKey("validation.required.images"),
					Param:   "",
				},
			},
		}

		recorder := httptest.NewRecorder()
		response.WithError(recorder, valErr)

		var result map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &result)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

		// Should return ALL field errors in array
		errors := result["errors"].([]interface{})
		assert.Len(t, errors, 2)

		firstError := errors[0].(map[string]interface{})
		assert.Equal(t, "title", firstError["field"])
		assert.Equal(t, "validation.required", firstError["message"])

		secondError := errors[1].(map[string]interface{})
		assert.Equal(t, "images", secondError["field"])
		assert.Equal(t, "validation.required", secondError["message"])
	})

	t.Run("Non-validation error - gallery not found", func(t *testing.T) {
		err := failure.NotFoundWithKey(errkey.ErrGalleryNotFound, "gallery not found")

		recorder := httptest.NewRecorder()
		response.WithError(recorder, err)

		var result map[string]interface{}
		json.Unmarshal(recorder.Body.Bytes(), &result)

		assert.Equal(t, http.StatusNotFound, recorder.Code)

		// Should have error field, not errors array
		assert.Equal(t, "gallery.not_found", result["error"])
		assert.Nil(t, result["errors"])
	})
}
