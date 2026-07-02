package password_test

import (
	"errors"
	"go-espn-api/shared/password"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestConstants(t *testing.T) {
	if password.DefaultCost != bcrypt.DefaultCost {
		t.Errorf("expected DefaultCost to be %d, got %d", bcrypt.DefaultCost, password.DefaultCost)
	}
}

func TestErrors(t *testing.T) {
	if password.ErrInvalidPassword.Error() != "invalid password" {
		t.Errorf("expected ErrInvalidPassword message to be 'invalid password', got %s", password.ErrInvalidPassword.Error())
	}
	if password.ErrEmptyPassword.Error() != "password cannot be empty" {
		t.Errorf("expected ErrEmptyPassword message to be 'password cannot be empty', got %s", password.ErrEmptyPassword.Error())
	}
	if password.ErrHashingPassword.Error() != "error hashing password" {
		t.Errorf("expected ErrHashingPassword message to be 'error hashing password', got %s", password.ErrHashingPassword.Error())
	}
	if password.ErrVerifyingPassword.Error() != "error verifying password" {
		t.Errorf("expected ErrVerifyingPassword message to be 'error verifying password', got %s", password.ErrVerifyingPassword.Error())
	}
}

func TestHash(t *testing.T) {
	tests := []struct {
		name          string
		password      string
		expectError   bool
		expectedError error
	}{
		{
			name:          "valid password",
			password:      "validPassword123",
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "empty password",
			password:      "",
			expectError:   true,
			expectedError: password.ErrEmptyPassword,
		},
		{
			name:          "short password",
			password:      "abc",
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "long password",
			password:      strings.Repeat("a", 100),
			expectError:   true,
			expectedError: password.ErrHashingPassword,
		},
		{
			name:          "password with special characters",
			password:      "P@ssw0rd!#$%^&*()",
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "unicode password",
			password:      "пароль123",
			expectError:   false,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := password.Hash(tt.password)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				if hash != "" {
					t.Errorf("expected empty hash when error occurs, got %s", hash)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if hash == "" {
					t.Error("expected non-empty hash, got empty string")
				}

				// Verify that the hash is a valid bcrypt hash
				if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") && !strings.HasPrefix(hash, "$2y$") {
					t.Errorf("expected bcrypt hash format, got %s", hash)
				}

				// Verify that the hash can be used to verify the original password
				if err := password.Verify(tt.password, hash); err != nil {
					t.Errorf("expected verification to succeed, got error: %v", err)
				}
			}
		})
	}
}

func TestVerify(t *testing.T) {
	// First, create a valid hash for testing
	testPassword := "testPassword123"
	validHash, err := password.Hash(testPassword)
	if err != nil {
		t.Fatalf("failed to create test hash: %v", err)
	}

	tests := []struct {
		name          string
		password      string
		hash          string
		expectError   bool
		expectedError error
	}{
		{
			name:          "valid password and hash",
			password:      testPassword,
			hash:          validHash,
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "wrong password",
			password:      "wrongPassword",
			hash:          validHash,
			expectError:   true,
			expectedError: password.ErrInvalidPassword,
		},
		{
			name:          "empty password",
			password:      "",
			hash:          validHash,
			expectError:   true,
			expectedError: password.ErrInvalidPassword,
		},
		{
			name:          "empty hash",
			password:      testPassword,
			hash:          "",
			expectError:   true,
			expectedError: password.ErrInvalidPassword,
		},
		{
			name:          "both empty",
			password:      "",
			hash:          "",
			expectError:   true,
			expectedError: password.ErrInvalidPassword,
		},
		{
			name:          "invalid hash format",
			password:      testPassword,
			hash:          "invalid_hash",
			expectError:   true,
			expectedError: password.ErrVerifyingPassword,
		},
		{
			name:          "truncated hash",
			password:      testPassword,
			hash:          validHash[:10],
			expectError:   true,
			expectedError: password.ErrVerifyingPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := password.Verify(tt.password, tt.hash)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestHashAndVerifyIntegration(t *testing.T) {
	passwords := []string{
		"simplePassword",
		"Complex!P@ssw0rd#123",
		"спец.символы_русский",
		"🚀🔐💻",
		strings.Repeat("a", 72),
	}

	for _, pwd := range passwords {
		t.Run("password_"+pwd[:minInt(len(pwd), 20)], func(t *testing.T) {
			// Hash the password
			hash, err := password.Hash(pwd)
			if err != nil {
				t.Fatalf("failed to hash password: %v", err)
			}

			// Verify the correct password
			if err := password.Verify(pwd, hash); err != nil {
				t.Errorf("failed to verify correct password: %v", err)
			}

			// Verify that wrong passwords fail
			wrongPasswords := []string{
				"wrong_password",
				"WRONG",
				"",
			}

			// For passwords shorter than 65 characters, also test with suffix
			if len(pwd) < 65 {
				wrongPasswords = append(wrongPasswords, pwd+"wrong", "wrong"+pwd)
			}

			for _, wrongPwd := range wrongPasswords {
				if wrongPwd == pwd {
					continue // Skip if it's the same as the original
				}
				if err := password.Verify(wrongPwd, hash); err == nil {
					t.Errorf("expected verification to fail for wrong password %q", wrongPwd)
				}
			}
		})
	}
}

func TestHashConsistency(t *testing.T) {
	pwd := "testPassword"

	// Generate multiple hashes for the same password
	hashes := make([]string, 5)
	for i := range hashes {
		hash, err := password.Hash(pwd)
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}
		hashes[i] = hash
	}

	// All hashes should be different (bcrypt uses salt)
	for i, hash1 := range hashes {
		for j, hash2 := range hashes {
			if i != j && hash1 == hash2 {
				t.Errorf("expected different hashes, got identical: %s", hash1)
			}
		}
	}

	// But all should verify the same password
	for _, hash := range hashes {
		if err := password.Verify(pwd, hash); err != nil {
			t.Errorf("failed to verify password with hash %s: %v", hash, err)
		}
	}
}

func TestHashAndVerifyLongPasswordError(t *testing.T) {
	// Test that passwords longer than 72 bytes fail
	longPassword := strings.Repeat("a", 100)

	_, err := password.Hash(longPassword)
	if err == nil {
		t.Error("expected error for password longer than 72 bytes")
	}

	if !errors.Is(err, password.ErrHashingPassword) {
		t.Errorf("expected ErrHashingPassword, got %v", err)
	}
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
