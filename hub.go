package main

import (
	"sync"

	"github.com/google/uuid"
)

// Hub maintains active sessions and routes messages between them.
type Hub struct {
	// Sessions indexed by session ID
	sessions map[string]*Session
	// Sessions indexed by user ID (a user can have multiple sessions)
	userSessions map[uuid.UUID][]*Session
	// Online status by user ID
	online map[uuid.UUID]bool

	mu sync.RWMutex

	// Channels for session management
	register   chan *Session
	unregister chan *Session
	shutdown   chan struct{}
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		sessions:     make(map[string]*Session),
		userSessions: make(map[uuid.UUID][]*Session),
		online:       make(map[uuid.UUID]bool),
		register:     make(chan *Session, 256),
		unregister:   make(chan *Session, 256),
		shutdown:     make(chan struct{}),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case sess := <-h.register:
			h.addSession(sess)

		case sess := <-h.unregister:
			h.removeSession(sess)

		case <-h.shutdown:
			h.closeAllSessions()
			return
		}
	}
}

// Shutdown gracefully shuts down the hub.
func (h *Hub) Shutdown() {
	close(h.shutdown)
}

// Register adds a session to the hub.
func (h *Hub) Register(sess *Session) {
	h.register <- sess
}

// Unregister removes a session from the hub.
func (h *Hub) Unregister(sess *Session) {
	h.unregister <- sess
}

func (h *Hub) addSession(sess *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sessions[sess.id] = sess

	// If authenticated, add to user sessions
	if sess.userID != uuid.Nil {
		h.userSessions[sess.userID] = append(h.userSessions[sess.userID], sess)
		h.online[sess.userID] = true
	}
}

func (h *Hub) removeSession(sess *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.sessions, sess.id)

	// Remove from user sessions if authenticated
	if sess.userID != uuid.Nil {
		sessions := h.userSessions[sess.userID]
		for i, s := range sessions {
			if s.id == sess.id {
				h.userSessions[sess.userID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
		// If no more sessions for this user, mark offline
		if len(h.userSessions[sess.userID]) == 0 {
			delete(h.userSessions, sess.userID)
			delete(h.online, sess.userID)
			// TODO: Broadcast offline presence
		}
	}
}

func (h *Hub) closeAllSessions() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sess := range h.sessions {
		sess.Close()
	}
	h.sessions = make(map[string]*Session)
	h.userSessions = make(map[uuid.UUID][]*Session)
	h.online = make(map[uuid.UUID]bool)
}

// GetSession returns a session by ID.
func (h *Hub) GetSession(id string) *Session {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[id]
}

// GetUserSessions returns all sessions for a user.
func (h *Hub) GetUserSessions(userID uuid.UUID) []*Session {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.userSessions[userID]
}

// IsOnline checks if a user has any active sessions.
func (h *Hub) IsOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.online[userID]
}

// SessionCount returns the total number of active sessions.
func (h *Hub) SessionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// OnlineCount returns the number of online users.
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.online)
}

// SendToUser sends a message to all sessions of a user.
func (h *Hub) SendToUser(userID uuid.UUID, msg *ServerMessage) {
	h.mu.RLock()
	sessions := h.userSessions[userID]
	h.mu.RUnlock()

	for _, sess := range sessions {
		sess.Send(msg)
	}
}

// SendToUsers sends a message to all sessions of multiple users.
func (h *Hub) SendToUsers(userIDs []uuid.UUID, msg *ServerMessage, skipSession string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range userIDs {
		for _, sess := range h.userSessions[userID] {
			if sess.id != skipSession {
				sess.Send(msg)
			}
		}
	}
}

// AuthenticateSession associates a session with a user ID.
func (h *Hub) AuthenticateSession(sess *Session, userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from old user if re-authenticating
	if sess.userID != uuid.Nil && sess.userID != userID {
		sessions := h.userSessions[sess.userID]
		for i, s := range sessions {
			if s.id == sess.id {
				h.userSessions[sess.userID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
		if len(h.userSessions[sess.userID]) == 0 {
			delete(h.userSessions, sess.userID)
			delete(h.online, sess.userID)
		}
	}

	sess.userID = userID
	h.userSessions[userID] = append(h.userSessions[userID], sess)
	h.online[userID] = true
}
