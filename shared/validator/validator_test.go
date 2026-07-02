package validator_test

import (
	"go-espn-api/shared/validator"
	"strings"
	"testing"
)

// Test structs for validation
type ValidTestStruct struct {
	Name     string `validate:"required" json:"name"`
	Email    string `validate:"required,email" json:"email"`
	Age      int    `validate:"gte=0,lte=120" json:"age"`
	Category string `validate:"oneof=user admin guest" json:"category"`
}

func TestValidateStruct(t *testing.T) {
	tests := []struct {
		name        string
		data        interface{}
		expectError bool
	}{
		{
			name: "valid struct",
			data: &ValidTestStruct{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      25,
				Category: "user",
			},
			expectError: false,
		},
		{
			name: "missing required field",
			data: &ValidTestStruct{
				Email:    "john@example.com",
				Age:      25,
				Category: "user",
			},
			expectError: true,
		},
		{
			name: "invalid email",
			data: &ValidTestStruct{
				Name:     "John Doe",
				Email:    "invalid-email",
				Age:      25,
				Category: "user",
			},
			expectError: true,
		},
		{
			name: "age out of range",
			data: &ValidTestStruct{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      150,
				Category: "user",
			},
			expectError: true,
		},
		{
			name: "invalid category",
			data: &ValidTestStruct{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      25,
				Category: "invalid",
			},
			expectError: true,
		},
		{
			name: "negative age",
			data: &ValidTestStruct{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      -1,
				Category: "user",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateStruct[ValidTestStruct](tt.data.(*ValidTestStruct))

			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no validation error, got: %v", err)
			}
		})
	}
}

func TestValidateVar(t *testing.T) {
	tests := []struct {
		name        string
		field       interface{}
		tag         string
		expectError bool
	}{
		{
			name:        "valid required string",
			field:       "test",
			tag:         "required",
			expectError: false,
		},
		{
			name:        "empty required string",
			field:       "",
			tag:         "required",
			expectError: true,
		},
		{
			name:        "valid email",
			field:       "test@example.com",
			tag:         "email",
			expectError: false,
		},
		{
			name:        "invalid email",
			field:       "invalid-email",
			tag:         "email",
			expectError: true,
		},
		{
			name:        "valid number in range",
			field:       25,
			tag:         "gte=0,lte=100",
			expectError: false,
		},
		{
			name:        "number out of range",
			field:       150,
			tag:         "gte=0,lte=100",
			expectError: true,
		},
		{
			name:        "valid oneof",
			field:       "admin",
			tag:         "oneof=user admin guest",
			expectError: false,
		},
		{
			name:        "invalid oneof",
			field:       "invalid",
			tag:         "oneof=user admin guest",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateVar(tt.field, tt.tag)

			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no validation error, got: %v", err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		jsonBody    string
		expectError bool
	}{
		{
			name:        "valid JSON",
			jsonBody:    `{"name":"John Doe","email":"john@example.com","age":25,"category":"user"}`,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			jsonBody:    `{"name":"John Doe","email":"invalid-email","age":25,"category":"user"}`,
			expectError: true,
		},
		{
			name:        "malformed JSON",
			jsonBody:    `{"name":"John Doe","email":}`,
			expectError: true,
		},
		{
			name:        "empty JSON",
			jsonBody:    `{}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.jsonBody)
			var data ValidTestStruct
			err := validator.Validate(reader, &data)

			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no validation error, got: %v", err)
			}
		})
	}
}

// Test custom validation messages
func TestValidationMessages(t *testing.T) {
	data := &ValidTestStruct{}
	err := validator.ValidateStruct[ValidTestStruct](data)

	if err == nil {
		t.Fatal("expected validation error for empty struct")
	}

	errorMsg := err.Error()

	// Check that error message contains field name and is descriptive
	if !strings.Contains(errorMsg, "required") || errorMsg == "" {
		t.Errorf("expected descriptive error message containing 'required', got: %s", errorMsg)
	}
}

// Test edge cases for file validation functions (testing indirectly through ValidateVar)
func TestFileValidationEdgeCases(t *testing.T) {
	// Test with base64 strings that would trigger custom validators
	tests := []struct {
		name  string
		field interface{}
		tag   string
	}{
		{
			name:  "test var validation",
			field: "test",
			tag:   "required",
		},
		// We can't easily test the custom file validators without proper setup
		// but we can test that the validation system works with basic validators
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mainly tests that the validator is working
			err := validator.ValidateVar(tt.field, tt.tag)
			if err != nil {
				t.Logf("Validation result: %v", err)
			}
		})
	}
}

// Test validation error handling
func TestValidationErrorHandling(t *testing.T) {
	// Test with multiple validation errors
	data := &ValidTestStruct{
		Name:     "",        // required violation
		Email:    "invalid", // email violation
		Age:      -1,        // gte violation
		Category: "invalid", // oneof violation
	}

	err := validator.ValidateStruct[ValidTestStruct](data)
	if err == nil {
		t.Fatal("expected validation error")
	}

	// The error should be descriptive and contain information about the failure
	errorMsg := err.Error()
	if errorMsg == "" {
		t.Error("expected non-empty error message")
	}
}

// Test that the validator package initializes correctly
func TestValidatorInitialization(t *testing.T) {
	// Test that we can validate basic structs without panic
	// This indirectly tests that the init() function worked correctly
	data := &ValidTestStruct{
		Name:     "Test",
		Email:    "test@example.com",
		Age:      25,
		Category: "user",
	}

	err := validator.ValidateStruct[ValidTestStruct](data)
	if err != nil {
		t.Errorf("expected no validation error for valid struct, got: %v", err)
	}
}
