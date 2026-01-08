package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()

	cors(handler).ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected origin https://example.com, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_BlocksUnconfiguredOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()

	cors(handler).ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_AllowsAllWithWildcard(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := CORS(CORSConfig{
		AllowedOrigins: []string{"*"},
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://any-site.com")
	rr := httptest.NewRecorder()

	cors(handler).ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://any-site.com" {
		t.Errorf("expected origin https://any-site.com, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_HandlesPreflight(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	cors(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
}

func TestCORS_RejectsPreflightForBlockedOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cors := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	cors(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestMatchWildcardOrigin(t *testing.T) {
	tests := []struct {
		pattern string
		origin  string
		want    bool
	}{
		{"https://*.example.com", "https://app.example.com", true},
		{"https://*.example.com", "https://api.example.com", true},
		{"https://*.example.com", "https://example.com", false},           // No subdomain
		{"https://*.example.com", "https://evil.com", false},              // Different domain
		{"https://*.example.com", "http://app.example.com", false},        // Different scheme
		{"https://example.com", "https://example.com", true},              // Exact match (no wildcard)
		{"https://example.com", "https://other.com", false},               // No match
		{"https://*.example.com", "https://sub.sub.example.com", true},    // Nested subdomain
	}

	for _, tt := range tests {
		got := matchWildcardOrigin(tt.pattern, tt.origin)
		if got != tt.want {
			t.Errorf("matchWildcardOrigin(%q, %q) = %v, want %v", tt.pattern, tt.origin, got, tt.want)
		}
	}
}

func TestCheckOrigin(t *testing.T) {
	tests := []struct {
		name     string
		allowed  []string
		origin   string
		want     bool
	}{
		{"allows configured origin", []string{"https://example.com"}, "https://example.com", true},
		{"blocks unconfigured origin", []string{"https://example.com"}, "https://evil.com", false},
		{"allows all with wildcard", []string{"*"}, "https://any.com", true},
		{"allows wildcard subdomain", []string{"https://*.example.com"}, "https://app.example.com", true},
		{"allows no origin header", []string{"https://example.com"}, "", true},
		{"allows all when empty config", []string{}, "https://any.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkFn := CheckOrigin(tt.allowed)
			req := httptest.NewRequest("GET", "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			got := checkFn(req)
			if got != tt.want {
				t.Errorf("CheckOrigin(%v)(%q) = %v, want %v", tt.allowed, tt.origin, got, tt.want)
			}
		})
	}
}
