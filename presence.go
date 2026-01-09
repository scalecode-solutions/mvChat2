package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/redis"
	"github.com/scalecode-solutions/mvchat2/store"
)

// PresenceManager handles online/offline status and notifications.
type PresenceManager struct {
	hub   *Hub
	db    *store.DB
	redis *redis.Client
}

// NewPresenceManager creates a new presence manager.
func NewPresenceManager(hub *Hub, db *store.DB) *PresenceManager {
	return &PresenceManager{
		hub:   hub,
		db:    db,
		redis: hub.redis,
	}
}

// UserOnline is called when a user comes online (first session connects).
func (p *PresenceManager) UserOnline(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update Redis presence cache if enabled
	if p.redis != nil {
		if err := p.redis.SetOnline(ctx, userID.String()); err != nil {
			log.Printf("presence: failed to set online for user %s: %v", shortID(userID), err)
		}
	}

	// Get all users who should be notified (DM partners and group members)
	notifyUsers := p.getPresenceSubscribers(ctx, userID)

	// Send online notification
	presMsg := &ServerMessage{
		Pres: &MsgServerPres{
			UserID: userID.String(),
			What:   "on",
		},
	}

	for _, uid := range notifyUsers {
		p.hub.SendToUser(uid, presMsg)
	}
}

// UserOffline is called when a user goes offline (last session disconnects).
func (p *PresenceManager) UserOffline(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Remove from Redis presence cache if enabled
	if p.redis != nil {
		if err := p.redis.SetOffline(ctx, userID.String()); err != nil {
			log.Printf("presence: failed to set offline for user %s: %v", shortID(userID), err)
		}
	}

	// Update last_seen in database
	p.db.UpdateUserLastSeen(ctx, userID, "")

	// Get user's last seen time
	user, _ := p.db.GetUserByID(ctx, userID)
	var lastSeen *time.Time
	if user != nil {
		lastSeen = user.LastSeen
	}

	// Get all users who should be notified
	notifyUsers := p.getPresenceSubscribers(ctx, userID)

	// Send offline notification
	presMsg := &ServerMessage{
		Pres: &MsgServerPres{
			UserID:   userID.String(),
			What:     "off",
			LastSeen: lastSeen,
		},
	}

	for _, uid := range notifyUsers {
		p.hub.SendToUser(uid, presMsg)
	}
}

// getPresenceSubscribers returns all users who should receive presence updates for a user.
// This includes DM partners and group members.
func (p *PresenceManager) getPresenceSubscribers(ctx context.Context, userID uuid.UUID) []uuid.UUID {
	subscribers := make(map[uuid.UUID]bool)

	// Get all conversations the user is in
	convs, err := p.db.GetUserConversations(ctx, userID)
	if err != nil {
		return nil
	}

	for _, conv := range convs {
		// Get members of each conversation
		members, err := p.db.GetConversationMembers(ctx, conv.Conversation.ID)
		if err != nil {
			continue
		}
		for _, memberID := range members {
			if memberID != userID {
				subscribers[memberID] = true
			}
		}
	}

	// Convert map to slice
	result := make([]uuid.UUID, 0, len(subscribers))
	for uid := range subscribers {
		result = append(result, uid)
	}
	return result
}

// SendPresenceProbe sends current online status of requested users.
// Called when a client wants to know who's online.
func (p *PresenceManager) SendPresenceProbe(s *Session, userIDs []uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, uid := range userIDs {
		user, err := p.db.GetUserByID(ctx, uid)
		if err != nil || user == nil {
			continue
		}

		var presMsg *ServerMessage
		if p.IsOnline(ctx, uid) {
			presMsg = &ServerMessage{
				Pres: &MsgServerPres{
					UserID: uid.String(),
					What:   "on",
				},
			}
		} else {
			presMsg = &ServerMessage{
				Pres: &MsgServerPres{
					UserID:   uid.String(),
					What:     "off",
					LastSeen: user.LastSeen,
				},
			}
		}
		s.Send(presMsg)
	}
}

// IsOnline checks if a user is online (locally or via Redis).
func (p *PresenceManager) IsOnline(ctx context.Context, userID uuid.UUID) bool {
	// Check local hub first
	if p.hub.IsOnline(userID) {
		return true
	}

	// Check Redis if enabled (user might be on another node)
	if p.redis != nil {
		online, err := p.redis.IsOnline(ctx, userID.String())
		if err != nil {
			log.Printf("presence: failed to check online status for user %s: %v", shortID(userID), err)
			return false
		}
		return online
	}

	return false
}

// StartHeartbeat starts a goroutine that refreshes online TTLs in Redis.
func (p *PresenceManager) StartHeartbeat(ctx context.Context) {
	if p.redis == nil {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				p.refreshOnlineUsers(ctx)
			}
		}
	}()
}

// refreshOnlineUsers refreshes the TTL for all locally online users.
func (p *PresenceManager) refreshOnlineUsers(ctx context.Context) {
	p.hub.mu.RLock()
	userIDs := make([]uuid.UUID, 0, len(p.hub.online))
	for uid := range p.hub.online {
		userIDs = append(userIDs, uid)
	}
	p.hub.mu.RUnlock()

	for _, uid := range userIDs {
		if err := p.redis.RefreshOnline(ctx, uid.String()); err != nil {
			log.Printf("presence: failed to refresh online for user %s: %v", shortID(uid), err)
		}
	}
}
