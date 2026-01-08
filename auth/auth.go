// Package auth provides authentication for mvChat2.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrUserNotFound       = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrWeakPassword       = errors.New("password too weak")
	ErrWeakUsername       = errors.New("username too short")
)

// Config holds authentication configuration.
type Config struct {
	// JWT signing key
	TokenKey []byte
	// Token expiration duration (default 2 weeks)
	TokenExpiry time.Duration
	// Minimum username length
	MinUsernameLength int
	// Minimum password length
	MinPasswordLength int
}

// Auth handles authentication operations.
type Auth struct {
	config Config
}

// New creates a new Auth instance.
func New(cfg Config) *Auth {
	if cfg.TokenExpiry == 0 {
		cfg.TokenExpiry = 14 * 24 * time.Hour // 2 weeks
	}
	if cfg.MinUsernameLength == 0 {
		cfg.MinUsernameLength = 4
	}
	if cfg.MinPasswordLength == 0 {
		cfg.MinPasswordLength = 6
	}
	return &Auth{config: cfg}
}

// Claims represents JWT claims.
type Claims struct {
	UserID uuid.UUID `json:"uid"`
	jwt.RegisteredClaims
}

// HashPassword hashes a password using Argon2id.
func (a *Auth) HashPassword(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash with Argon2id
	// Parameters: time=1, memory=64MB, threads=4, keyLen=32
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Encode as base64: $argon2id$salt$hash
	return fmt.Sprintf("$argon2id$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword verifies a password against a hash.
func (a *Auth) VerifyPassword(password, encoded string) bool {
	// Parse the encoded hash
	var salt, hash []byte
	var err error

	// Expected format: $argon2id$salt$hash
	var saltStr, hashStr string
	_, err = fmt.Sscanf(encoded, "$argon2id$%s$%s", &saltStr, &hashStr)
	if err != nil {
		return false
	}

	// Handle the case where Sscanf doesn't split on $
	parts := splitArgon2Hash(encoded)
	if len(parts) != 3 {
		return false
	}
	saltStr = parts[1]
	hashStr = parts[2]

	salt, err = base64.RawStdEncoding.DecodeString(saltStr)
	if err != nil {
		return false
	}
	hash, err = base64.RawStdEncoding.DecodeString(hashStr)
	if err != nil {
		return false
	}

	// Compute hash with same parameters
	computed := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Constant-time comparison
	return subtle.ConstantTimeCompare(hash, computed) == 1
}

func splitArgon2Hash(encoded string) []string {
	// Split "$argon2id$salt$hash" into ["argon2id", "salt", "hash"]
	if len(encoded) < 10 || encoded[0] != '$' {
		return nil
	}
	result := make([]string, 0, 3)
	start := 1
	for i := 1; i < len(encoded); i++ {
		if encoded[i] == '$' {
			result = append(result, encoded[start:i])
			start = i + 1
		}
	}
	if start < len(encoded) {
		result = append(result, encoded[start:])
	}
	return result
}

// GenerateToken generates a JWT token for a user.
func (a *Auth) GenerateToken(userID uuid.UUID) (string, time.Time, error) {
	expiresAt := time.Now().Add(a.config.TokenExpiry)

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mvchat2",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.config.TokenKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.config.TokenKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateUsername checks if a username meets requirements.
func (a *Auth) ValidateUsername(username string) error {
	if len(username) < a.config.MinUsernameLength {
		return ErrWeakUsername
	}
	return nil
}

// ValidatePassword checks if a password meets requirements.
func (a *Auth) ValidatePassword(password string) error {
	if len(password) < a.config.MinPasswordLength {
		return ErrWeakPassword
	}
	return nil
}
