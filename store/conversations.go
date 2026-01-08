package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Conversation represents a conversation (DM or group).
type Conversation struct {
	ID        uuid.UUID       `json:"id"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Type      string          `json:"type"` // "dm" or "group"
	OwnerID   *uuid.UUID      `json:"ownerId,omitempty"`
	Public    json.RawMessage `json:"public,omitempty"`
	LastSeq   int             `json:"lastSeq"`
	LastMsgAt *time.Time      `json:"lastMsgAt,omitempty"`
	DelID     int             `json:"delId"`
}

// Member represents a user's membership in a conversation.
type Member struct {
	ConversationID uuid.UUID       `json:"conversationId"`
	UserID         uuid.UUID       `json:"userId"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Role           string          `json:"role"`
	ReadSeq        int             `json:"readSeq"`
	RecvSeq        int             `json:"recvSeq"`
	ClearSeq       int             `json:"clearSeq"`
	Favorite       bool            `json:"favorite"`
	Muted          bool            `json:"muted"`
	Blocked        bool            `json:"blocked"`
	Private        json.RawMessage `json:"private,omitempty"`
	DeletedAt      *time.Time      `json:"deletedAt,omitempty"`
}

// ConversationWithMember combines conversation data with member-specific data.
type ConversationWithMember struct {
	Conversation
	// Member data (embedded without json tags to avoid conflicts)
	MemberCreatedAt time.Time       `json:"-"`
	MemberUpdatedAt time.Time       `json:"-"`
	Role            string          `json:"role"`
	ReadSeq         int             `json:"readSeq"`
	RecvSeq         int             `json:"recvSeq"`
	ClearSeq        int             `json:"clearSeq"`
	Favorite        bool            `json:"favorite"`
	Muted           bool            `json:"muted"`
	Blocked         bool            `json:"blocked"`
	Private         json.RawMessage `json:"private,omitempty"`
	// For DMs: the other user's info
	OtherUser *User `json:"otherUser,omitempty"`
}

// CreateDM creates a new DM conversation between two users.
// Returns existing conversation if one already exists.
func (db *DB) CreateDM(ctx context.Context, user1ID, user2ID uuid.UUID) (*Conversation, bool, error) {
	// Ensure consistent ordering (user1 < user2)
	if user1ID.String() > user2ID.String() {
		user1ID, user2ID = user2ID, user1ID
	}

	// Check if DM already exists
	var existingID uuid.UUID
	err := db.pool.QueryRow(ctx, `
		SELECT conversation_id FROM dm_participants
		WHERE user1_id = $1 AND user2_id = $2
	`, user1ID, user2ID).Scan(&existingID)

	if err == nil {
		// DM exists, return it
		conv, err := db.GetConversationByID(ctx, existingID)
		return conv, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	// Create new DM
	now := time.Now().UTC()
	convID := uuid.New()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	// Create conversation
	_, err = tx.Exec(ctx, `
		INSERT INTO conversations (id, created_at, updated_at, type)
		VALUES ($1, $2, $3, 'dm')
	`, convID, now, now)
	if err != nil {
		return nil, false, err
	}

	// Create dm_participants entry
	_, err = tx.Exec(ctx, `
		INSERT INTO dm_participants (conversation_id, user1_id, user2_id)
		VALUES ($1, $2, $3)
	`, convID, user1ID, user2ID)
	if err != nil {
		return nil, false, err
	}

	// Create member entries for both users
	_, err = tx.Exec(ctx, `
		INSERT INTO members (conversation_id, user_id, created_at, updated_at, role)
		VALUES ($1, $2, $3, $4, 'member'), ($1, $5, $3, $4, 'member')
	`, convID, user1ID, now, now, user2ID)
	if err != nil {
		return nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}

	return &Conversation{
		ID:        convID,
		CreatedAt: now,
		UpdatedAt: now,
		Type:      "dm",
		LastSeq:   0,
	}, true, nil
}

// CreateGroup creates a new group conversation.
func (db *DB) CreateGroup(ctx context.Context, ownerID uuid.UUID, public json.RawMessage) (*Conversation, error) {
	now := time.Now().UTC()
	convID := uuid.New()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create conversation
	_, err = tx.Exec(ctx, `
		INSERT INTO conversations (id, created_at, updated_at, type, owner_id, public)
		VALUES ($1, $2, $3, 'group', $4, $5)
	`, convID, now, now, ownerID, public)
	if err != nil {
		return nil, err
	}

	// Add owner as member
	_, err = tx.Exec(ctx, `
		INSERT INTO members (conversation_id, user_id, created_at, updated_at, role)
		VALUES ($1, $2, $3, $4, 'owner')
	`, convID, ownerID, now, now)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &Conversation{
		ID:        convID,
		CreatedAt: now,
		UpdatedAt: now,
		Type:      "group",
		OwnerID:   &ownerID,
		Public:    public,
		LastSeq:   0,
	}, nil
}

// GetConversationByID retrieves a conversation by ID.
func (db *DB) GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	var conv Conversation
	err := db.pool.QueryRow(ctx, `
		SELECT id, created_at, updated_at, type, owner_id, public, last_seq, last_msg_at, del_id
		FROM conversations WHERE id = $1
	`, id).Scan(&conv.ID, &conv.CreatedAt, &conv.UpdatedAt, &conv.Type, &conv.OwnerID, &conv.Public, &conv.LastSeq, &conv.LastMsgAt, &conv.DelID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// GetMember retrieves a user's membership in a conversation.
func (db *DB) GetMember(ctx context.Context, convID, userID uuid.UUID) (*Member, error) {
	var m Member
	err := db.pool.QueryRow(ctx, `
		SELECT conversation_id, user_id, created_at, updated_at, role,
			read_seq, recv_seq, clear_seq, favorite, muted, blocked, private, deleted_at
		FROM members WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID).Scan(&m.ConversationID, &m.UserID, &m.CreatedAt, &m.UpdatedAt, &m.Role,
		&m.ReadSeq, &m.RecvSeq, &m.ClearSeq, &m.Favorite, &m.Muted, &m.Blocked, &m.Private, &m.DeletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetUserConversations retrieves all conversations for a user.
func (db *DB) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]ConversationWithMember, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT c.id, c.created_at, c.updated_at, c.type, c.owner_id, c.public, c.last_seq, c.last_msg_at, c.del_id,
			m.created_at, m.updated_at, m.role, m.read_seq, m.recv_seq, m.clear_seq, m.favorite, m.muted, m.blocked, m.private
		FROM conversations c
		JOIN members m ON c.id = m.conversation_id
		WHERE m.user_id = $1 AND m.deleted_at IS NULL
		ORDER BY COALESCE(c.last_msg_at, c.created_at) DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ConversationWithMember
	for rows.Next() {
		var cwm ConversationWithMember
		if err := rows.Scan(
			&cwm.Conversation.ID, &cwm.Conversation.CreatedAt, &cwm.Conversation.UpdatedAt,
			&cwm.Conversation.Type, &cwm.Conversation.OwnerID, &cwm.Conversation.Public,
			&cwm.Conversation.LastSeq, &cwm.Conversation.LastMsgAt, &cwm.Conversation.DelID,
			&cwm.MemberCreatedAt, &cwm.MemberUpdatedAt, &cwm.Role,
			&cwm.ReadSeq, &cwm.RecvSeq, &cwm.ClearSeq,
			&cwm.Favorite, &cwm.Muted, &cwm.Blocked, &cwm.Private,
		); err != nil {
			return nil, err
		}
		results = append(results, cwm)
	}

	// For DMs, fetch the other user's info
	for i := range results {
		if results[i].Type == "dm" {
			otherUser, err := db.GetDMOtherUser(ctx, results[i].Conversation.ID, userID)
			if err == nil && otherUser != nil {
				results[i].OtherUser = otherUser
			}
		}
	}

	return results, rows.Err()
}

// GetDMOtherUser gets the other user in a DM conversation.
func (db *DB) GetDMOtherUser(ctx context.Context, convID, userID uuid.UUID) (*User, error) {
	var otherUserID uuid.UUID
	err := db.pool.QueryRow(ctx, `
		SELECT CASE WHEN user1_id = $2 THEN user2_id ELSE user1_id END
		FROM dm_participants WHERE conversation_id = $1
	`, convID, userID).Scan(&otherUserID)
	if err != nil {
		return nil, err
	}
	return db.GetUserByID(ctx, otherUserID)
}

// GetConversationMembers retrieves all members of a conversation.
func (db *DB) GetConversationMembers(ctx context.Context, convID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT user_id FROM members
		WHERE conversation_id = $1 AND deleted_at IS NULL
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		members = append(members, userID)
	}
	return members, rows.Err()
}

// UpdateMemberSettings updates a member's settings (favorite, muted, blocked, private).
func (db *DB) UpdateMemberSettings(ctx context.Context, convID, userID uuid.UUID, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()

	// Build dynamic UPDATE query
	setClauses := ""
	args := []any{convID, userID}
	i := 3
	for key, val := range updates {
		if setClauses != "" {
			setClauses += ", "
		}
		setClauses += key + " = $" + string(rune('0'+i))
		args = append(args, val)
		i++
	}

	// This is a simplified version - in production use a query builder
	query := "UPDATE members SET " + setClauses + " WHERE conversation_id = $1 AND user_id = $2"
	_, err := db.pool.Exec(ctx, query, args...)
	return err
}

// UpdateReadSeq updates a member's read sequence.
func (db *DB) UpdateReadSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE members SET read_seq = $3, recv_seq = GREATEST(recv_seq, $3), updated_at = $4
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID, seq, time.Now().UTC())
	return err
}

// IsMember checks if a user is a member of a conversation.
func (db *DB) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM members
			WHERE conversation_id = $1 AND user_id = $2 AND deleted_at IS NULL
		)
	`, convID, userID).Scan(&exists)
	return exists, err
}

// IsBlocked checks if a user is blocked in a DM.
func (db *DB) IsBlocked(ctx context.Context, convID, blockerID, blockedID uuid.UUID) (bool, error) {
	var blocked bool
	err := db.pool.QueryRow(ctx, `
		SELECT blocked FROM members
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, blockerID).Scan(&blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return blocked, err
}
