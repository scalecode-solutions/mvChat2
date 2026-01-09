package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_RejectsEmptyKeys(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "empty api_key_salt",
			cfg: Config{
				Auth:     AuthConfig{APIKeySalt: "", Token: TokenAuthConfig{Key: strings.Repeat("x", 32)}},
				Database: DatabaseConfig{UIDKey: "x", EncryptionKey: strings.Repeat("x", 32)},
			},
			wantErr: "auth.api_key_salt is required",
		},
		{
			name: "empty token key",
			cfg: Config{
				Auth:     AuthConfig{APIKeySalt: strings.Repeat("x", 32), Token: TokenAuthConfig{Key: ""}},
				Database: DatabaseConfig{UIDKey: "x", EncryptionKey: strings.Repeat("x", 32)},
			},
			wantErr: "auth.token.key is required",
		},
		{
			name: "empty uid_key",
			cfg: Config{
				Auth:     AuthConfig{APIKeySalt: strings.Repeat("x", 32), Token: TokenAuthConfig{Key: strings.Repeat("x", 32)}},
				Database: DatabaseConfig{UIDKey: "", EncryptionKey: strings.Repeat("x", 32)},
			},
			wantErr: "database.uid_key is required",
		},
		{
			name: "empty encryption_key",
			cfg: Config{
				Auth:     AuthConfig{APIKeySalt: strings.Repeat("x", 32), Token: TokenAuthConfig{Key: strings.Repeat("x", 32)}},
				Database: DatabaseConfig{UIDKey: "x", EncryptionKey: ""},
			},
			wantErr: "database.encryption_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidate_RejectsInsecureDefaults(t *testing.T) {
	tests := []struct {
		name       string
		apiSalt    string
		tokenKey   string
		uidKey     string
		encryptKey string
		wantErr    string
	}{
		{
			name:       "insecure api_key_salt",
			apiSalt:    "T713/rYYgW7g4m3vG6zGRh7+FM1t0T8j13koXScOAj4=",
			tokenKey:   "newSecureTokenKeyThatIs32Chars!!",
			uidKey:     "newUIDKey1234567",
			encryptKey: "newEncryptionKeyThatIs32Chars!!",
			wantErr:    "auth.api_key_salt is using an insecure default value - generate a new key",
		},
		{
			name:       "insecure token key",
			apiSalt:    "newSecureApiSaltThatIs32Chars!!!",
			tokenKey:   "wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=",
			uidKey:     "newUIDKey1234567",
			encryptKey: "newEncryptionKeyThatIs32Chars!!",
			wantErr:    "auth.token.key is using an insecure default value - generate a new key",
		},
		{
			name:       "insecure uid_key",
			apiSalt:    "newSecureApiSaltThatIs32Chars!!!",
			tokenKey:   "newSecureTokenKeyThatIs32Chars!!",
			uidKey:     "la6YsO+bNX/+XIkOqc5Svw==",
			encryptKey: "newEncryptionKeyThatIs32Chars!!",
			wantErr:    "database.uid_key is using an insecure default value - generate a new key",
		},
		{
			name:       "insecure encryption_key",
			apiSalt:    "newSecureApiSaltThatIs32Chars!!!",
			tokenKey:   "newSecureTokenKeyThatIs32Chars!!",
			uidKey:     "newUIDKey1234567",
			encryptKey: "k8Jz9mN2pQ4rT6vX8yB1dF3hK5nP7sU0wA2cE4gI6jL=",
			wantErr:    "database.encryption_key is using an insecure default value - generate a new key",
		},
		{
			name:       "common placeholder - changeme",
			apiSalt:    "changeme",
			tokenKey:   "newSecureTokenKeyThatIs32Chars!!",
			uidKey:     "newUIDKey1234567",
			encryptKey: "newEncryptionKeyThatIs32Chars!!",
			wantErr:    "auth.api_key_salt is using an insecure default value - generate a new key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Auth: AuthConfig{
					APIKeySalt: tt.apiSalt,
					Token:      TokenAuthConfig{Key: tt.tokenKey},
				},
				Database: DatabaseConfig{
					UIDKey:        tt.uidKey,
					EncryptionKey: tt.encryptKey,
				},
			}
			err := cfg.validate()
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidate_RejectsShortKeys(t *testing.T) {
	tests := []struct {
		name       string
		tokenKey   string
		encryptKey string
		wantErr    string
	}{
		{
			name:       "short token key",
			tokenKey:   "shortkey",
			encryptKey: "validEncryptionKeyThatIs32Chars!",
			wantErr:    "auth.token.key must be at least 32 characters",
		},
		{
			name:       "short encryption key",
			tokenKey:   "validTokenKeyThatIsAtLeast32Char",
			encryptKey: "short",
			wantErr:    "database.encryption_key must be at least 32 characters (for AES-256)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Auth: AuthConfig{
					APIKeySalt: "validApiSaltThatIsNotInBlocklist",
					Token:      TokenAuthConfig{Key: tt.tokenKey},
				},
				Database: DatabaseConfig{
					UIDKey:        "validUIDKey12345",
					EncryptionKey: tt.encryptKey,
				},
			}
			err := cfg.validate()
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidate_AcceptsSecureConfig(t *testing.T) {
	cfg := Config{
		Auth: AuthConfig{
			APIKeySalt: "uniqueSecureApiSaltGenerated1234",
			Token:      TokenAuthConfig{Key: "uniqueSecureTokenKeyGenerated123"},
			Basic:      BasicAuthConfig{MinLoginLength: 4, MinPasswordLength: 6},
		},
		Database: DatabaseConfig{
			UIDKey:        "uniqueUIDKey1234",
			EncryptionKey: "uniqueSecureEncryptionKey1234567",
		},
		Limits: LimitsConfig{
			MaxMessageSize:     131072,
			MaxSubscriberCount: 128,
		},
		Media: MediaConfig{
			MaxSize: 8388608,
		},
	}
	err := cfg.validate()
	if err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestLoad_ExpandsEnvVars(t *testing.T) {
	// Create a temp config file
	content := `
auth:
  api_key_salt: ${TEST_API_SALT}
  token:
    key: ${TEST_TOKEN_KEY}
database:
  uid_key: ${TEST_UID_KEY}
  encryption_key: ${TEST_ENCRYPTION_KEY}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set environment variables
	os.Setenv("TEST_API_SALT", "envApiSaltValueThatIsLongEnough!")
	os.Setenv("TEST_TOKEN_KEY", "envTokenKeyValueThatIs32CharsLong")
	os.Setenv("TEST_UID_KEY", "envUIDKeyValue12")
	os.Setenv("TEST_ENCRYPTION_KEY", "envEncryptionKeyThatIs32CharsLng")
	defer func() {
		os.Unsetenv("TEST_API_SALT")
		os.Unsetenv("TEST_TOKEN_KEY")
		os.Unsetenv("TEST_UID_KEY")
		os.Unsetenv("TEST_ENCRYPTION_KEY")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Auth.APIKeySalt != "envApiSaltValueThatIsLongEnough!" {
		t.Errorf("expected api_key_salt from env, got %q", cfg.Auth.APIKeySalt)
	}
	if cfg.Auth.Token.Key != "envTokenKeyValueThatIs32CharsLong" {
		t.Errorf("expected token.key from env, got %q", cfg.Auth.Token.Key)
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:       "localhost",
		Port:       5432,
		Name:       "testdb",
		User:       "testuser",
		Password:   "testpass",
		SSLMode:    "disable",
		SQLTimeout: 10,
	}

	dsn := cfg.DSN()
	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable&connect_timeout=10"
	if dsn != expected {
		t.Errorf("DSN() = %q, want %q", dsn, expected)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	// Invalid YAML - tabs mixed with spaces incorrectly
	content := ":\n  bad yaml content [\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_ValidationFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	// Valid YAML but missing required fields - will fail validation
	content := `
auth:
  api_key_salt: ""
  token:
    key: ""
database:
  uid_key: ""
  encryption_key: ""
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestExpandEnvVars_WithDefault(t *testing.T) {
	content := "${NONEXISTENT_VAR:defaultvalue}"
	result := expandEnvVars(content)
	if result != "defaultvalue" {
		t.Errorf("expected 'defaultvalue', got %q", result)
	}
}

func TestExpandEnvVars_ExistingVar(t *testing.T) {
	os.Setenv("EXISTING_VAR", "actualvalue")
	defer os.Unsetenv("EXISTING_VAR")

	content := "${EXISTING_VAR:defaultvalue}"
	result := expandEnvVars(content)
	if result != "actualvalue" {
		t.Errorf("expected 'actualvalue', got %q", result)
	}
}

func TestExpandEnvVars_NoDefault(t *testing.T) {
	// Variable not set and no default - should return empty string
	content := "${NONEXISTENT_VAR_NO_DEFAULT}"
	result := expandEnvVars(content)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestValidate_NumericLimits(t *testing.T) {
	baseConfig := func() Config {
		return Config{
			Auth: AuthConfig{
				APIKeySalt: "uniqueSecureApiSaltGenerated1234",
				Token:      TokenAuthConfig{Key: "uniqueSecureTokenKeyGenerated123"},
				Basic:      BasicAuthConfig{MinLoginLength: 4, MinPasswordLength: 6},
			},
			Database: DatabaseConfig{
				UIDKey:        "uniqueUIDKey1234",
				EncryptionKey: "uniqueSecureEncryptionKey1234567",
			},
			Limits: LimitsConfig{
				MaxMessageSize:     131072,
				MaxSubscriberCount: 128,
			},
			Media: MediaConfig{
				MaxSize: 8388608,
			},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name:    "min_login_length <= 0",
			modify:  func(c *Config) { c.Auth.Basic.MinLoginLength = 0 },
			wantErr: "auth.basic.min_login_length must be > 0",
		},
		{
			name:    "min_password_length <= 0",
			modify:  func(c *Config) { c.Auth.Basic.MinPasswordLength = 0 },
			wantErr: "auth.basic.min_password_length must be > 0",
		},
		{
			name:    "max_message_size <= 0",
			modify:  func(c *Config) { c.Limits.MaxMessageSize = 0 },
			wantErr: "limits.max_message_size must be > 0",
		},
		{
			name:    "max_subscriber_count <= 0",
			modify:  func(c *Config) { c.Limits.MaxSubscriberCount = 0 },
			wantErr: "limits.max_subscriber_count must be > 0",
		},
		{
			name:    "media.max_size <= 0",
			modify:  func(c *Config) { c.Media.MaxSize = 0 },
			wantErr: "media.max_size must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			tt.modify(&cfg)
			err := cfg.validate()
			if err == nil {
				t.Errorf("expected error %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
