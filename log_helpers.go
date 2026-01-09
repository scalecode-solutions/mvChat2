package main

import (
	"strings"

	"github.com/google/uuid"
)

// maskEmail masks an email address for logging, showing only first char and domain.
// Example: "john.doe@example.com" -> "j***@example.com"
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	return string(email[0]) + "***" + email[at:]
}

// shortID returns a truncated UUID string for logging (first 8 chars).
// Example: "550e8400-e29b-41d4-a716-446655440000" -> "550e8400"
func shortID(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}
