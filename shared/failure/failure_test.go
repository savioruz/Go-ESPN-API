package failure_test

import (
	"errors"
	"net/http"
	"go-espn-api/shared/failure"
	"testing"
)

func TestFailure_Error(t *testing.T) {
	f := &failure.Failure{
		Code:    http.StatusBadRequest,
		Message: "test error message",
	}

	if f.Error() != "test error message" {
		t.Errorf("expected error message to be 'test error message', got %s", f.Error())
	}
}

func TestPredefinedFailures(t *testing.T) {
	tests := []struct {
		name    string
		failure *failure.Failure
		code    int
		message string
	}{
		{
			name:    "InvalidPageParam",
			failure: failure.InvalidPageParam,
			code:    http.StatusBadRequest,
			message: "invalid page parameter",
		},
		{
			name:    "InvalidLimitParam",
			failure: failure.InvalidLimitParam,
			code:    http.StatusBadRequest,
			message: "invalid limit parameter",
		},
		{
			name:    "ForbiddenError",
			failure: failure.ForbiddenError,
			code:    http.StatusForbidden,
			message: "You don't have the required permissions",
		},
		{
			name:    "ResourceRestrictedError",
			failure: failure.ResourceRestrictedError,
			code:    http.StatusForbidden,
			message: "You don't have permission to access this resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.failure.Code != tt.code {
				t.Errorf("expected code to be %d, got %d", tt.code, tt.failure.Code)
			}
			if tt.failure.Message != tt.message {
				t.Errorf("expected message to be %s, got %s", tt.message, tt.failure.Message)
			}
		})
	}
}

func TestBadRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{
			name:     "with error",
			input:    errors.New("validation failed"),
			expected: &failure.Failure{Code: http.StatusBadRequest, Message: "validation failed"},
		},
		{
			name:     "with nil error",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := failure.BadRequest(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				f, ok := result.(*failure.Failure)
				if !ok {
					t.Errorf("expected result to be *failure.Failure, got %T", result)
				} else {
					expectedF := tt.expected.(*failure.Failure)
					if f.Code != expectedF.Code || f.Message != expectedF.Message {
						t.Errorf("expected %+v, got %+v", expectedF, f)
					}
				}
			}
		})
	}
}

func TestBadRequestFromString(t *testing.T) {
	result := failure.BadRequestFromString("custom bad request")

	f, ok := result.(*failure.Failure)
	if !ok {
		t.Errorf("expected result to be *failure.Failure, got %T", result)
	} else {
		if f.Code != http.StatusBadRequest {
			t.Errorf("expected code to be %d, got %d", http.StatusBadRequest, f.Code)
		}
		if f.Message != "custom bad request" {
			t.Errorf("expected message to be 'custom bad request', got %s", f.Message)
		}
	}
}

func TestUnauthorized(t *testing.T) {
	result := failure.Unauthorized("token expired")

	f, ok := result.(*failure.Failure)
	if !ok {
		t.Errorf("expected result to be *failure.Failure, got %T", result)
	} else {
		if f.Code != http.StatusUnauthorized {
			t.Errorf("expected code to be %d, got %d", http.StatusUnauthorized, f.Code)
		}
		if f.Message != "token expired" {
			t.Errorf("expected message to be 'token expired', got %s", f.Message)
		}
	}
}

func TestInternalError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{
			name:     "with error",
			input:    errors.New("database connection failed"),
			expected: &failure.Failure{Code: http.StatusInternalServerError, Message: "database connection failed"},
		},
		{
			name:     "with nil error",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := failure.InternalError(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				f, ok := result.(*failure.Failure)
				if !ok {
					t.Errorf("expected result to be *failure.Failure, got %T", result)
				} else {
					expectedF := tt.expected.(*failure.Failure)
					if f.Code != expectedF.Code || f.Message != expectedF.Message {
						t.Errorf("expected %+v, got %+v", expectedF, f)
					}
				}
			}
		})
	}
}

func TestUnimplemented(t *testing.T) {
	result := failure.Unimplemented("GetUserByID")

	f, ok := result.(*failure.Failure)
	if !ok {
		t.Errorf("expected result to be *failure.Failure, got %T", result)
	} else {
		if f.Code != http.StatusNotImplemented {
			t.Errorf("expected code to be %d, got %d", http.StatusNotImplemented, f.Code)
		}
		if f.Message != "GetUserByID" {
			t.Errorf("expected message to be 'GetUserByID', got %s", f.Message)
		}
	}
}

func TestNotFound(t *testing.T) {
	result := failure.NotFound("User not found")

	f, ok := result.(*failure.Failure)
	if !ok {
		t.Errorf("expected result to be *failure.Failure, got %T", result)
	} else {
		if f.Code != http.StatusNotFound {
			t.Errorf("expected code to be %d, got %d", http.StatusNotFound, f.Code)
		}
		if f.Message != "User not found" {
			t.Errorf("expected message to be 'User not found', got %s", f.Message)
		}
	}
}

func TestConflict(t *testing.T) {
	result := failure.Conflict("Email already exists")

	f, ok := result.(*failure.Failure)
	if !ok {
		t.Errorf("expected result to be *failure.Failure, got %T", result)
	} else {
		if f.Code != http.StatusConflict {
			t.Errorf("expected code to be %d, got %d", http.StatusConflict, f.Code)
		}
		if f.Message != "Email already exists" {
			t.Errorf("expected message to be 'Email already exists', got %s", f.Message)
		}
	}
}

func TestForbidden(t *testing.T) {
	result := failure.Forbidden("Access denied")

	f, ok := result.(*failure.Failure)
	if !ok {
		t.Errorf("expected result to be *failure.Failure, got %T", result)
	} else {
		if f.Code != http.StatusForbidden {
			t.Errorf("expected code to be %d, got %d", http.StatusForbidden, f.Code)
		}
		if f.Message != "Access denied" {
			t.Errorf("expected message to be 'Access denied', got %s", f.Message)
		}
	}
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected int
	}{
		{
			name:     "failure error",
			input:    &failure.Failure{Code: http.StatusBadRequest, Message: "test"},
			expected: http.StatusBadRequest,
		},
		{
			name:     "wrapped failure error",
			input:    failure.BadRequestFromString("test"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "regular error",
			input:    errors.New("regular error"),
			expected: http.StatusInternalServerError,
		},
		{
			name:     "nil error",
			input:    nil,
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := failure.GetCode(tt.input)
			if result != tt.expected {
				t.Errorf("expected code to be %d, got %d", tt.expected, result)
			}
		})
	}
}
