package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/redis"
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

	// Presence manager (set after initialization)
	presence *PresenceManager

	// Redis client for pub/sub (optional, nil if not enabled)
	redis *redis.Client
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

// SetPresence sets the presence manager.
func (h *Hub) SetPresence(p *PresenceManager) {
	h.presence = p
}

// SetRedis sets the Redis client for pub/sub.
func (h *Hub) SetRedis(r *redis.Client) {
	h.redis = r
}

// PubSubPayload wraps a server message with routing info for pub/sub.
type PubSubPayload struct {
	UserID  string         `json:"userId"`
	Message *ServerMessage `json:"message"`
}

// HandlePubSubMessage handles messages received from Redis pub/sub.
func (h *Hub) HandlePubSubMessage(msg *redis.Message) {
	var payload PubSubPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("hub: failed to unmarshal pub/sub message: %v", err)
		return
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		log.Printf("hub: invalid user ID in pub/sub message: %v", err)
		return
	}

	// Deliver to local sessions for this user
	h.SendToUser(userID, payload.Message)
}

// PublishToUser publishes a message to a user via Redis (for cross-node delivery).
func (h *Hub) PublishToUser(ctx context.Context, userID uuid.UUID, msg *ServerMessage) error {
	if h.redis == nil {
		return nil
	}
	return h.redis.Publish(ctx, "user:"+userID.String(), "data", msg)
}

// Register adds a session to the hub.
// Non-blocking: if buffer is full, spawns goroutine to retry.
func (h *Hub) Register(sess *Session) {
	select {
	case h.register <- sess:
	default:
		// Buffer full - spawn goroutine to avoid blocking caller
		go func() { h.register <- sess }()
	}
}

// Unregister removes a session from the hub.
// Non-blocking: if buffer is full, spawns goroutine to retry.
// This prevents connection leaks when sessions can't unregister.
func (h *Hub) Unregister(sess *Session) {
	select {
	case h.unregister <- sess:
	default:
		// Buffer full - spawn goroutine to avoid blocking caller
		go func() { h.unregister <- sess }()
	}
}

func (h *Hub) addSession(sess *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sessions[sess.id] = sess

	// If authenticated, add to user sessions
	userID := sess.UserID()
	if userID != uuid.Nil {
		h.userSessions[userID] = append(h.userSessions[userID], sess)
		h.online[userID] = true
	}
}

func (h *Hub) removeSession(sess *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.sessions, sess.id)

	// Remove from user sessions if authenticated
	userID := sess.UserID()
	if userID != uuid.Nil {
		sessions := h.userSessions[userID]
		for i, s := range sessions {
			if s.id == sess.id {
				h.userSessions[userID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
		// If no more sessions for this user, mark offline
		if len(h.userSessions[userID]) == 0 {
			delete(h.userSessions, userID)
			delete(h.online, userID)
			// Broadcast offline presence
			if h.presence != nil {
				go h.presence.UserOffline(userID)
			}
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
// If Redis is enabled and a user isn't on this node, publishes to Redis for cross-node delivery.
func (h *Hub) SendToUsers(userIDs []uuid.UUID, msg *ServerMessage, skipSession string) {
	h.mu.RLock()
	localUsers := make(map[uuid.UUID]bool)
	for _, userID := range userIDs {
		sessions := h.userSessions[userID]
		if len(sessions) > 0 {
			localUsers[userID] = true
			for _, sess := range sessions {
				if sess.id != skipSession {
					sess.Send(msg)
				}
			}
		}
	}
	h.mu.RUnlock()

	// Publish to Redis for users not on this node
	if h.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, userID := range userIDs {
			if !localUsers[userID] {
				// Check if user is online on another node
				online, err := h.redis.IsOnline(ctx, userID.String())
				if err != nil {
					log.Printf("hub: failed to check online status for user %s: %v", userID, err)
					continue
				}
				if online {
					payload := PubSubPayload{
						UserID:  userID.String(),
						Message: msg,
					}
					if err := h.redis.Publish(ctx, "user:"+userID.String(), "data", payload); err != nil {
						log.Printf("hub: failed to publish to user %s: %v", userID, err)
					}
				}
			}
		}
	}
}

// AuthenticateSession associates a session with a user ID.
func (h *Hub) AuthenticateSession(sess *Session, userID uuid.UUID) {
	h.mu.Lock()

	// Check if this is the first session for this user
	wasOnline := h.online[userID]

	// Remove from old user if re-authenticating
	oldUserID := sess.UserID()
	if oldUserID != uuid.Nil && oldUserID != userID {
		sessions := h.userSessions[oldUserID]
		for i, s := range sessions {
			if s.id == sess.id {
				h.userSessions[oldUserID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
		if len(h.userSessions[oldUserID]) == 0 {
			delete(h.userSessions, oldUserID)
			delete(h.online, oldUserID)
		}
	}

	sess.SetUserID(userID)
	h.userSessions[userID] = append(h.userSessions[userID], sess)
	h.online[userID] = true

	h.mu.Unlock()

	// Broadcast online presence if this is the first session
	if !wasOnline && h.presence != nil {
		go h.presence.UserOnline(userID)
	}
}
