// Package middleware provides HTTP middleware functions.
package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int // Preflight cache duration in seconds
}

// CORS creates a middleware that handles CORS headers.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	// Parse allowed origins for matching
	allowAll := len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*"

	// Default methods if not specified
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}

	// Default headers if not specified
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Content-Type", "Authorization", "X-Requested-With"}
	}

	// Default max age
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 86400 // 24 hours
	}

	methodsStr := strings.Join(cfg.AllowedMethods, ", ")
	headersStr := strings.Join(cfg.AllowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Always set Vary header for caching
			w.Header().Add("Vary", "Origin")

			// Check if origin is allowed
			allowedOrigin := ""
			if origin != "" {
				if allowAll {
					allowedOrigin = origin
				} else {
					for _, allowed := range cfg.AllowedOrigins {
						if allowed == origin {
							allowedOrigin = origin
							break
						}
						// Support wildcard subdomains (e.g., "https://*.example.com")
						if strings.Contains(allowed, "*") {
							if matchWildcardOrigin(allowed, origin) {
								allowedOrigin = origin
								break
							}
						}
					}
				}
			}

			// If origin not allowed, don't add CORS headers
			if allowedOrigin == "" && origin != "" {
				// Still process the request but without CORS headers
				// This allows same-origin requests to work
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Set CORS headers
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", methodsStr)
				w.Header().Set("Access-Control-Allow-Headers", headersStr)
				w.Header().Set("Access-Control-Max-Age", itoa(cfg.MaxAge))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// matchWildcardOrigin checks if origin matches a pattern with wildcard.
// e.g., "https://*.example.com" matches "https://app.example.com"
func matchWildcardOrigin(pattern, origin string) bool {
	// Simple wildcard matching for subdomains
	// Pattern: https://*.example.com
	// Origin: https://app.example.com

	if !strings.Contains(pattern, "*") {
		return pattern == origin
	}

	// Split pattern and origin by "://"
	patternParts := strings.SplitN(pattern, "://", 2)
	originParts := strings.SplitN(origin, "://", 2)

	if len(patternParts) != 2 || len(originParts) != 2 {
		return false
	}

	// Schemes must match
	if patternParts[0] != originParts[0] {
		return false
	}

	// Check host matching with wildcard
	patternHost := patternParts[1]
	originHost := originParts[1]

	// Remove port if present for matching
	if idx := strings.Index(patternHost, ":"); idx != -1 {
		patternHost = patternHost[:idx]
	}
	if idx := strings.Index(originHost, ":"); idx != -1 {
		originHost = originHost[:idx]
	}

	// Replace * with regex-like matching
	// "*.example.com" should match "app.example.com" but not "example.com"
	if strings.HasPrefix(patternHost, "*.") {
		suffix := patternHost[1:] // ".example.com"
		if strings.HasSuffix(originHost, suffix) && len(originHost) > len(suffix) {
			return true
		}
	}

	return false
}

// CheckOrigin returns a function for WebSocket origin checking.
func CheckOrigin(allowedOrigins []string) func(*http.Request) bool {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	if len(allowedOrigins) == 0 {
		allowAll = true // Default to allow all if not configured
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// No origin header (same-origin request)
		if origin == "" {
			return true
		}

		// Allow all origins
		if allowAll {
			return true
		}

		// Check against allowed list
		for _, allowed := range allowedOrigins {
			if allowed == origin {
				return true
			}
			if strings.Contains(allowed, "*") && matchWildcardOrigin(allowed, origin) {
				return true
			}
		}

		return false
	}
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var buf [20]byte
	pos := len(buf)
	negative := i < 0
	if negative {
		i = -i
	}

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}
