package main

import "github.com/google/uuid"

// SessionInterface defines the methods handlers need from a session.
// This interface enables mocking sessions in tests.
type SessionInterface interface {
	ID() string
	UserID() uuid.UUID
	UserAgent() string
	IsAuthenticated() bool
	RequireAuth(msgID string) bool
	Send(msg *ServerMessage)
}

// Compile-time check that Session implements SessionInterface.
var _ SessionInterface = (*Session)(nil)
