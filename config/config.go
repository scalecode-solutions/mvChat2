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
}

// DebugConfig contains debugging endpoints configuration.
type DebugConfig struct {
	ExpvarPath string `yaml:"expvar_path"`
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
}

// validate checks that required fields are set.
func (c *Config) validate() error {
	if c.Auth.APIKeySalt == "" {
		return fmt.Errorf("auth.api_key_salt is required")
	}
	if c.Auth.Token.Key == "" {
		return fmt.Errorf("auth.token.key is required")
	}
	if c.Database.UIDKey == "" {
		return fmt.Errorf("database.uid_key is required")
	}
	return nil
}
