package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/store"
)

// PresenceManager handles online/offline status and notifications.
type PresenceManager struct {
	hub *Hub
	db  *store.DB
}

// NewPresenceManager creates a new presence manager.
func NewPresenceManager(hub *Hub, db *store.DB) *PresenceManager {
	return &PresenceManager{
		hub: hub,
		db:  db,
	}
}

// UserOnline is called when a user comes online (first session connects).
func (p *PresenceManager) UserOnline(userID uuid.UUID) {
	ctx := context.Background()

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
	ctx := context.Background()

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
	ctx := context.Background()

	for _, uid := range userIDs {
		user, err := p.db.GetUserByID(ctx, uid)
		if err != nil || user == nil {
			continue
		}

		var presMsg *ServerMessage
		if p.hub.IsOnline(uid) {
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
