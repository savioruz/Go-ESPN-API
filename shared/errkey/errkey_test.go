package errkey_test

import (
	"encoding/json"
	"go-espn-api/shared/errkey"
	"go-espn-api/transport/http/response"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorKeyInResponse(t *testing.T) {
	// Create error response
	errMsg := "gallery not found"

	errResponse := response.Error{
		Error: &errMsg,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(errResponse)
	assert.NoError(t, err)

	// Check JSON contains error
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	assert.NoError(t, err)

	assert.Equal(t, "gallery not found", result["error"])
}

func TestErrorKeyConstants(t *testing.T) {
	tests := []struct {
		name     string
		key      errkey.ErrorKey
		expected string
	}{
		{"Gallery not found", errkey.ErrGalleryNotFound, "gallery.not_found"},
		{"Validation failed", errkey.ErrValidationFailed, "validation.failed"},
		{"Unauthorized", errkey.ErrUnauthorized, "auth.unauthorized"},
		{"Internal server error", errkey.ErrInternalServer, "server.internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.key.String())
		})
	}
}
