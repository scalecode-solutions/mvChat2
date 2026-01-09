package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func testConfig() Config {
	return Config{
		TokenKey:          []byte("test-secret-key-32-bytes-long!!!"),
		TokenExpiry:       time.Hour,
		MinUsernameLength: 4,
		MinPasswordLength: 6,
	}
}

func TestNew_DefaultValues(t *testing.T) {
	// Test with empty config - should use defaults
	a := New(Config{})

	if a.config.TokenExpiry != 14*24*time.Hour {
		t.Errorf("expected default TokenExpiry 2 weeks, got %v", a.config.TokenExpiry)
	}
	if a.config.MinUsernameLength != 4 {
		t.Errorf("expected default MinUsernameLength 4, got %d", a.config.MinUsernameLength)
	}
	if a.config.MinPasswordLength != 6 {
		t.Errorf("expected default MinPasswordLength 6, got %d", a.config.MinPasswordLength)
	}
}

func TestNew_CustomValues(t *testing.T) {
	cfg := Config{
		TokenKey:          []byte("custom-key"),
		TokenExpiry:       2 * time.Hour,
		MinUsernameLength: 5,
		MinPasswordLength: 8,
	}
	a := New(cfg)

	if a.config.TokenExpiry != 2*time.Hour {
		t.Errorf("expected TokenExpiry 2h, got %v", a.config.TokenExpiry)
	}
	if a.config.MinUsernameLength != 5 {
		t.Errorf("expected MinUsernameLength 5, got %d", a.config.MinUsernameLength)
	}
	if a.config.MinPasswordLength != 8 {
		t.Errorf("expected MinPasswordLength 8, got %d", a.config.MinPasswordLength)
	}
}

func TestHashPassword_Success(t *testing.T) {
	a := New(testConfig())

	hash, err := a.HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Should start with $argon2id$
	if len(hash) < 10 || hash[:9] != "$argon2id" {
		t.Errorf("hash should start with $argon2id$, got: %s", hash)
	}

	// Should have 3 parts
	parts := splitArgon2Hash(hash)
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}
}

func TestHashPassword_DifferentSalts(t *testing.T) {
	a := New(testConfig())

	// Same password should produce different hashes (different salts)
	hash1, _ := a.HashPassword("samepassword")
	hash2, _ := a.HashPassword("samepassword")

	if hash1 == hash2 {
		t.Error("same password should produce different hashes due to random salt")
	}
}

func TestVerifyPassword_Success(t *testing.T) {
	a := New(testConfig())

	password := "testpassword123"
	hash, _ := a.HashPassword(password)

	if !a.VerifyPassword(password, hash) {
		t.Error("VerifyPassword should return true for correct password")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	a := New(testConfig())

	hash, _ := a.HashPassword("correctpassword")

	if a.VerifyPassword("wrongpassword", hash) {
		t.Error("VerifyPassword should return false for wrong password")
	}
}

func TestVerifyPassword_InvalidFormat(t *testing.T) {
	a := New(testConfig())

	testCases := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"no prefix", "argon2id$salt$hash"},
		{"wrong prefix", "$bcrypt$salt$hash"},
		{"too few parts", "$argon2id$onlyonepart"},
		{"too short", "$arg"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if a.VerifyPassword("anypassword", tc.hash) {
				t.Errorf("VerifyPassword should return false for invalid format: %s", tc.hash)
			}
		})
	}
}

func TestVerifyPassword_InvalidBase64Salt(t *testing.T) {
	a := New(testConfig())

	// Invalid base64 in salt
	hash := "$argon2id$!!!invalid!!!$validhash"
	if a.VerifyPassword("password", hash) {
		t.Error("VerifyPassword should return false for invalid base64 salt")
	}
}

func TestVerifyPassword_InvalidBase64Hash(t *testing.T) {
	a := New(testConfig())

	// Valid base64 salt but invalid base64 hash
	hash := "$argon2id$dGVzdHNhbHQ$!!!invalid!!!"
	if a.VerifyPassword("password", hash) {
		t.Error("VerifyPassword should return false for invalid base64 hash")
	}
}

func TestSplitArgon2Hash(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"valid", "$argon2id$salt$hash", []string{"argon2id", "salt", "hash"}},
		{"empty", "", nil},
		{"too short", "$short", nil},
		{"no leading $", "argon2id$salt$hash", nil},
		{"single part too short", "$argon2id", nil}, // Less than 10 chars
		{"two parts", "$argon2id$salt", []string{"argon2id", "salt"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := splitArgon2Hash(tc.input)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("expected %d parts, got %d", len(tc.expected), len(result))
				return
			}
			for i, part := range result {
				if part != tc.expected[i] {
					t.Errorf("part %d: expected %s, got %s", i, tc.expected[i], part)
				}
			}
		})
	}
}

func TestGenerateToken_Success(t *testing.T) {
	a := New(testConfig())
	userID := uuid.New()

	token, expiresAt, err := a.GenerateToken(userID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}

	// ExpiresAt should be about 1 hour from now (our test config)
	expectedExpiry := time.Now().Add(time.Hour)
	if expiresAt.Before(expectedExpiry.Add(-time.Minute)) || expiresAt.After(expectedExpiry.Add(time.Minute)) {
		t.Errorf("expiresAt should be ~1 hour from now, got %v", expiresAt)
	}
}

func TestGenerateToken_EmptyKey(t *testing.T) {
	a := New(Config{TokenKey: []byte{}})
	userID := uuid.New()

	// Should still work with empty key (though not secure)
	token, _, err := a.GenerateToken(userID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
}

func TestValidateToken_Success(t *testing.T) {
	a := New(testConfig())
	userID := uuid.New()

	token, _, _ := a.GenerateToken(userID)

	claims, err := a.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, claims.UserID)
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	// Create auth with very short expiry
	a := New(Config{
		TokenKey:    []byte("test-key"),
		TokenExpiry: -time.Hour, // Already expired
	})
	userID := uuid.New()

	token, _, _ := a.GenerateToken(userID)

	_, err := a.ValidateToken(token)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestValidateToken_InvalidToken(t *testing.T) {
	a := New(testConfig())

	testCases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not.a.valid.token"},
		{"malformed", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.ValidateToken(tc.token)
			if err != ErrInvalidToken {
				t.Errorf("expected ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestValidateToken_WrongKey(t *testing.T) {
	a1 := New(Config{TokenKey: []byte("key-one")})
	a2 := New(Config{TokenKey: []byte("key-two")})

	token, _, _ := a1.GenerateToken(uuid.New())

	// Try to validate with different key
	_, err := a2.ValidateToken(token)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for wrong key, got %v", err)
	}
}

func TestValidateUsername_Valid(t *testing.T) {
	a := New(Config{MinUsernameLength: 4})

	testCases := []string{"user", "username", "verylongusername"}
	for _, username := range testCases {
		if err := a.ValidateUsername(username); err != nil {
			t.Errorf("username %q should be valid, got error: %v", username, err)
		}
	}
}

func TestValidateUsername_TooShort(t *testing.T) {
	a := New(Config{MinUsernameLength: 4})

	testCases := []string{"", "a", "ab", "abc"}
	for _, username := range testCases {
		if err := a.ValidateUsername(username); err != ErrWeakUsername {
			t.Errorf("username %q should be too short, got: %v", username, err)
		}
	}
}

func TestValidatePassword_Valid(t *testing.T) {
	a := New(Config{MinPasswordLength: 6})

	testCases := []string{"123456", "password", "verylongpassword"}
	for _, password := range testCases {
		if err := a.ValidatePassword(password); err != nil {
			t.Errorf("password %q should be valid, got error: %v", password, err)
		}
	}
}

func TestValidatePassword_TooShort(t *testing.T) {
	a := New(Config{MinPasswordLength: 6})

	testCases := []string{"", "a", "12345"}
	for _, password := range testCases {
		if err := a.ValidatePassword(password); err != ErrWeakPassword {
			t.Errorf("password %q should be too short, got: %v", password, err)
		}
	}
}

func TestNewValidator(t *testing.T) {
	a := New(testConfig())
	v := NewValidator(a)

	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
	if v.auth != a {
		t.Error("validator should reference the auth instance")
	}
}

func TestValidator_ValidateToken_Success(t *testing.T) {
	a := New(testConfig())
	v := NewValidator(a)
	userID := uuid.New()

	token, _, _ := a.GenerateToken(userID)

	resultID, err := v.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if resultID != userID {
		t.Errorf("expected userID %s, got %s", userID, resultID)
	}
}

func TestValidator_ValidateToken_Invalid(t *testing.T) {
	a := New(testConfig())
	v := NewValidator(a)

	_, err := v.ValidateToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}
