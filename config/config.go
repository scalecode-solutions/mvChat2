// Package config provides typed configuration loading for mvChat2 server.
package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration structure for mvChat2 server.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Email    EmailConfig    `yaml:"email"`
	Auth     AuthConfig     `yaml:"auth"`
	Media    MediaConfig    `yaml:"media"`
	Limits   LimitsConfig   `yaml:"limits"`
	Debug    DebugConfig    `yaml:"debug"`
}

// ServerConfig contains HTTP/WebSocket server settings.
type ServerConfig struct {
	Listen             string `yaml:"listen"`
	APIPath            string `yaml:"api_path"`
	StaticMount        string `yaml:"static_mount"`
	StaticPath         string `yaml:"static_path"`
	UseXForwardedFor   bool   `yaml:"use_x_forwarded_for"`
	DefaultCountryCode string `yaml:"default_country_code"`
	// HTTP server timeouts (in seconds)
	ReadTimeout     int `yaml:"read_timeout"`
	WriteTimeout    int `yaml:"write_timeout"`
	IdleTimeout     int `yaml:"idle_timeout"`
	ShutdownTimeout int `yaml:"shutdown_timeout"`
	// CORS configuration
	// Use "*" to allow all origins (not recommended for production)
	// Use specific origins like ["https://example.com", "https://app.example.com"]
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// DatabaseConfig contains PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Name            string `yaml:"name"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	SSLMode         string `yaml:"ssl_mode"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
	SQLTimeout      int    `yaml:"sql_timeout"`
	UIDKey          string `yaml:"uid_key"`
	EncryptionKey   string `yaml:"encryption_key"`
}

// DSN returns a PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.SQLTimeout,
	)
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	APIKeySalt string          `yaml:"api_key_salt"`
	Basic      BasicAuthConfig `yaml:"basic"`
	Token      TokenAuthConfig `yaml:"token"`
}

// BasicAuthConfig contains basic (login/password) auth settings.
type BasicAuthConfig struct {
	MinLoginLength    int `yaml:"min_login_length"`
	MinPasswordLength int `yaml:"min_password_length"`
}

// TokenAuthConfig contains token auth settings.
type TokenAuthConfig struct {
	Key          string `yaml:"key"`
	ExpireIn     int    `yaml:"expire_in"`
	SerialNumber int    `yaml:"serial_number"`
}

// MediaConfig contains media/file upload settings.
type MediaConfig struct {
	MaxSize     int64  `yaml:"max_size"`
	UploadDir   string `yaml:"upload_dir"`
	GCPeriod    int    `yaml:"gc_period"`
	GCBlockSize int    `yaml:"gc_block_size"`
}

// LimitsConfig contains various size and count limits.
type LimitsConfig struct {
	MaxMessageSize      int `yaml:"max_message_size"`
	MaxSubscriberCount  int `yaml:"max_subscriber_count"`
	EditWindowMinutes   int `yaml:"edit_window_minutes"`
	UnsendWindowMinutes int `yaml:"unsend_window_minutes"`
	MaxEditCount        int `yaml:"max_edit_count"`

	// Rate limiting (per session/user)
	RateLimitMessages  int `yaml:"rate_limit_messages"`  // Max messages per second
	RateLimitAuth      int `yaml:"rate_limit_auth"`      // Max auth attempts per minute
	RateLimitUpload    int `yaml:"rate_limit_upload"`    // Max uploads per minute
}

// DebugConfig contains debugging endpoints configuration.
type DebugConfig struct {
	ExpvarPath string `yaml:"expvar_path"`
}

// RedisConfig contains Redis connection settings.
type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	NodeID   string `yaml:"node_id"`
}

// EmailConfig contains email/SMTP settings.
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	cfg.applyDefaults()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// expandEnvVars expands ${VAR} and ${VAR:default} patterns in the config.
func expandEnvVars(content string) string {
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		parts := re.FindStringSubmatch(match)
		envVar := parts[1]
		defaultVal := ""
		if len(parts) > 2 {
			defaultVal = parts[2]
		}

		if val := os.Getenv(envVar); val != "" {
			return val
		}
		return defaultVal
	})
}

// applyDefaults sets default values for unset fields.
func (c *Config) applyDefaults() {
	// Server defaults
	if c.Server.Listen == "" {
		c.Server.Listen = ":6060"
	}
	if c.Server.APIPath == "" {
		c.Server.APIPath = "/"
	}
	if c.Server.DefaultCountryCode == "" {
		c.Server.DefaultCountryCode = "US"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 15 // 15 seconds
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 15 // 15 seconds
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 60 // 60 seconds
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 30 // 30 seconds for graceful shutdown
	}

	// Database defaults
	if c.Database.Host == "" {
		c.Database.Host = "localhost"
	}
	if c.Database.Port == 0 {
		c.Database.Port = 5432
	}
	if c.Database.Name == "" {
		c.Database.Name = "mvchat2"
	}
	if c.Database.User == "" {
		c.Database.User = "postgres"
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 50
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 50
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 60
	}
	if c.Database.SQLTimeout == 0 {
		c.Database.SQLTimeout = 10
	}

	// Auth defaults
	if c.Auth.Basic.MinLoginLength == 0 {
		c.Auth.Basic.MinLoginLength = 4
	}
	if c.Auth.Basic.MinPasswordLength == 0 {
		c.Auth.Basic.MinPasswordLength = 6
	}
	if c.Auth.Token.ExpireIn == 0 {
		c.Auth.Token.ExpireIn = 1209600 // 2 weeks
	}
	if c.Auth.Token.SerialNumber == 0 {
		c.Auth.Token.SerialNumber = 1
	}

	// Media defaults
	if c.Media.MaxSize == 0 {
		c.Media.MaxSize = 8388608 // 8MB
	}
	if c.Media.UploadDir == "" {
		c.Media.UploadDir = "./uploads"
	}
	if c.Media.GCPeriod == 0 {
		c.Media.GCPeriod = 60
	}
	if c.Media.GCBlockSize == 0 {
		c.Media.GCBlockSize = 100
	}

	// Limits defaults
	if c.Limits.MaxMessageSize == 0 {
		c.Limits.MaxMessageSize = 131072 // 128KB
	}
	if c.Limits.MaxSubscriberCount == 0 {
		c.Limits.MaxSubscriberCount = 128
	}
	if c.Limits.EditWindowMinutes == 0 {
		c.Limits.EditWindowMinutes = 15
	}
	if c.Limits.UnsendWindowMinutes == 0 {
		c.Limits.UnsendWindowMinutes = 10
	}
	if c.Limits.MaxEditCount == 0 {
		c.Limits.MaxEditCount = 10
	}
	if c.Limits.RateLimitMessages == 0 {
		c.Limits.RateLimitMessages = 30 // 30 messages per second
	}
	if c.Limits.RateLimitAuth == 0 {
		c.Limits.RateLimitAuth = 5 // 5 auth attempts per minute
	}
	if c.Limits.RateLimitUpload == 0 {
		c.Limits.RateLimitUpload = 10 // 10 uploads per minute
	}
}

// knownInsecureKeys contains default keys that must not be used in production.
// These are checked to prevent accidental deployment with example/default keys.
var knownInsecureKeys = map[string]bool{
	"la6YsO+bNX/+XIkOqc5Svw==":                         true, // Default UID key
	"k8Jz9mN2pQ4rT6vX8yB1dF3hK5nP7sU0wA2cE4gI6jL=":     true, // Default encryption key
	"T713/rYYgW7g4m3vG6zGRh7+FM1t0T8j13koXScOAj4=":     true, // Default API key salt
	"wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=":     true, // Default token key
	"your-256-bit-secret-key-here":                     true, // Common placeholder
	"your-secret-key":                                  true, // Common placeholder
	"changeme":                                         true, // Common placeholder
	"secret":                                           true, // Common placeholder
}

// validate checks that required fields are set and secure.
func (c *Config) validate() error {
	// Validate API key salt
	if c.Auth.APIKeySalt == "" {
		return fmt.Errorf("auth.api_key_salt is required")
	}
	if knownInsecureKeys[c.Auth.APIKeySalt] {
		return fmt.Errorf("auth.api_key_salt is using an insecure default value - generate a new key")
	}

	// Validate token key
	if c.Auth.Token.Key == "" {
		return fmt.Errorf("auth.token.key is required")
	}
	if knownInsecureKeys[c.Auth.Token.Key] {
		return fmt.Errorf("auth.token.key is using an insecure default value - generate a new key")
	}
	if len(c.Auth.Token.Key) < 32 {
		return fmt.Errorf("auth.token.key must be at least 32 characters")
	}

	// Validate UID key
	if c.Database.UIDKey == "" {
		return fmt.Errorf("database.uid_key is required")
	}
	if knownInsecureKeys[c.Database.UIDKey] {
		return fmt.Errorf("database.uid_key is using an insecure default value - generate a new key")
	}

	// Validate encryption key
	if c.Database.EncryptionKey == "" {
		return fmt.Errorf("database.encryption_key is required")
	}
	if knownInsecureKeys[c.Database.EncryptionKey] {
		return fmt.Errorf("database.encryption_key is using an insecure default value - generate a new key")
	}
	if len(c.Database.EncryptionKey) < 32 {
		return fmt.Errorf("database.encryption_key must be at least 32 characters (for AES-256)")
	}

	// Validate numeric limits
	if c.Auth.Basic.MinLoginLength <= 0 {
		return fmt.Errorf("auth.basic.min_login_length must be > 0")
	}
	if c.Auth.Basic.MinPasswordLength <= 0 {
		return fmt.Errorf("auth.basic.min_password_length must be > 0")
	}
	if c.Limits.MaxMessageSize <= 0 {
		return fmt.Errorf("limits.max_message_size must be > 0")
	}
	if c.Limits.MaxSubscriberCount <= 0 {
		return fmt.Errorf("limits.max_subscriber_count must be > 0")
	}
	if c.Media.MaxSize <= 0 {
		return fmt.Errorf("media.max_size must be > 0")
	}

	return nil
}
