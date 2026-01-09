package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Conversation represents a conversation (DM or room).
type Conversation struct {
	ID        uuid.UUID       `json:"id"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Type      string          `json:"type"` // "dm" or "room"
	OwnerID   *uuid.UUID      `json:"ownerId,omitempty"`
	Public    json.RawMessage `json:"public,omitempty"`
	LastSeq   int             `json:"lastSeq"`
	LastMsgAt *time.Time      `json:"lastMsgAt,omitempty"`
	DelID     int             `json:"delId"`
	// Disappearing messages TTL in seconds (nil = disabled)
	DisappearingTTL *int `json:"disappearingTTL,omitempty"`
	// Pinned message info
	PinnedMessageID *uuid.UUID `json:"pinnedMessageId,omitempty"`
	PinnedAt        *time.Time `json:"pinnedAt,omitempty"`
	PinnedBy        *uuid.UUID `json:"pinnedBy,omitempty"`
	// No-screenshots flag (set by owner/admin for rooms)
	NoScreenshots bool `json:"noScreenshots,omitempty"`
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
	// Pinned message seq (resolved from pinned_message_id)
	PinnedSeq *int `json:"pinnedSeq,omitempty"`
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

// CreateRoom creates a new room conversation.
func (db *DB) CreateRoom(ctx context.Context, ownerID uuid.UUID, public json.RawMessage) (*Conversation, error) {
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
		VALUES ($1, $2, $3, 'room', $4, $5)
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
		Type:      "room",
		OwnerID:   &ownerID,
		Public:    public,
		LastSeq:   0,
	}, nil
}

// GetConversationByID retrieves a conversation by ID.
func (db *DB) GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	var conv Conversation
	err := db.pool.QueryRow(ctx, `
		SELECT id, created_at, updated_at, type, owner_id, public, last_seq, last_msg_at, del_id,
			disappearing_ttl, pinned_message_id, pinned_at, pinned_by, no_screenshots
		FROM conversations WHERE id = $1
	`, id).Scan(&conv.ID, &conv.CreatedAt, &conv.UpdatedAt, &conv.Type, &conv.OwnerID, &conv.Public,
		&conv.LastSeq, &conv.LastMsgAt, &conv.DelID,
		&conv.DisappearingTTL, &conv.PinnedMessageID, &conv.PinnedAt, &conv.PinnedBy, &conv.NoScreenshots)

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
// Uses a single query with JOINs to avoid N+1 queries for DM other user info.
func (db *DB) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]ConversationWithMember, error) {
	// Single query that LEFT JOINs dm_participants and users to get DM other user info
	rows, err := db.pool.Query(ctx, `
		SELECT
			c.id, c.created_at, c.updated_at, c.type, c.owner_id, c.public, c.last_seq, c.last_msg_at, c.del_id,
			c.disappearing_ttl, c.pinned_message_id, c.pinned_at, c.pinned_by, c.no_screenshots,
			m.created_at, m.updated_at, m.role, m.read_seq, m.recv_seq, m.clear_seq, m.favorite, m.muted, m.blocked, m.private,
			-- DM other user fields (NULL for rooms)
			ou.id, ou.created_at, ou.updated_at, ou.state, ou.public, ou.last_seen, ou.user_agent,
			ou.must_change_password, ou.email, ou.email_verified,
			-- Pinned message seq (NULL if no pinned message)
			pm.seq
		FROM conversations c
		JOIN members m ON c.id = m.conversation_id
		-- LEFT JOIN to get other user for DMs
		LEFT JOIN dm_participants dp ON c.id = dp.conversation_id AND c.type = 'dm'
		LEFT JOIN users ou ON ou.id = CASE
			WHEN dp.user1_id = $1 THEN dp.user2_id
			ELSE dp.user1_id
		END AND ou.state != 'deleted'
		-- LEFT JOIN to get pinned message seq
		LEFT JOIN messages pm ON pm.id = c.pinned_message_id
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
		// For the other user, use pointer types to handle NULLs from LEFT JOIN
		var (
			ouID                 *uuid.UUID
			ouCreatedAt          *time.Time
			ouUpdatedAt          *time.Time
			ouState              *string
			ouPublic             []byte
			ouLastSeen           *time.Time
			ouUserAgent          *string
			ouMustChangePassword *bool
			ouEmail              *string
			ouEmailVerified      *bool
		)

		if err := rows.Scan(
			&cwm.Conversation.ID, &cwm.Conversation.CreatedAt, &cwm.Conversation.UpdatedAt,
			&cwm.Conversation.Type, &cwm.Conversation.OwnerID, &cwm.Conversation.Public,
			&cwm.Conversation.LastSeq, &cwm.Conversation.LastMsgAt, &cwm.Conversation.DelID,
			&cwm.Conversation.DisappearingTTL, &cwm.Conversation.PinnedMessageID,
			&cwm.Conversation.PinnedAt, &cwm.Conversation.PinnedBy, &cwm.Conversation.NoScreenshots,
			&cwm.MemberCreatedAt, &cwm.MemberUpdatedAt, &cwm.Role,
			&cwm.ReadSeq, &cwm.RecvSeq, &cwm.ClearSeq,
			&cwm.Favorite, &cwm.Muted, &cwm.Blocked, &cwm.Private,
			// Other user fields (nullable)
			&ouID, &ouCreatedAt, &ouUpdatedAt, &ouState, &ouPublic,
			&ouLastSeen, &ouUserAgent, &ouMustChangePassword, &ouEmail, &ouEmailVerified,
			// Pinned message seq
			&cwm.PinnedSeq,
		); err != nil {
			return nil, err
		}

		// If this is a DM and we have other user data, populate OtherUser
		if cwm.Type == "dm" && ouID != nil {
			cwm.OtherUser = &User{
				ID:        *ouID,
				CreatedAt: *ouCreatedAt,
				UpdatedAt: *ouUpdatedAt,
				State:     *ouState,
				Public:    ouPublic,
				LastSeen:  ouLastSeen,
			}
			if ouUserAgent != nil {
				cwm.OtherUser.UserAgent = *ouUserAgent
			}
			if ouMustChangePassword != nil {
				cwm.OtherUser.MustChangePassword = *ouMustChangePassword
			}
			if ouEmail != nil {
				cwm.OtherUser.Email = ouEmail
			}
			if ouEmailVerified != nil {
				cwm.OtherUser.EmailVerified = *ouEmailVerified
			}
		}

		results = append(results, cwm)
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

// MemberSettings holds updateable member settings.
type MemberSettings struct {
	Favorite *bool
	Muted    *bool
	Blocked  *bool
	Private  json.RawMessage
}

// UpdateMemberSettings updates a member's settings (favorite, muted, blocked, private).
// Only non-nil fields are updated.
func (db *DB) UpdateMemberSettings(ctx context.Context, convID, userID uuid.UUID, settings MemberSettings) error {
	now := time.Now().UTC()

	_, err := db.pool.Exec(ctx, `
		UPDATE members SET
			favorite = COALESCE($3, favorite),
			muted = COALESCE($4, muted),
			blocked = COALESCE($5, blocked),
			private = COALESCE($6, private),
			updated_at = $7
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID, settings.Favorite, settings.Muted, settings.Blocked, settings.Private, now)
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

// UpdateRecvSeq updates a user's received (delivered) sequence number.
// This indicates the user's device has received messages up to this seq.
func (db *DB) UpdateRecvSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE members SET recv_seq = GREATEST(recv_seq, $3), updated_at = $4
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID, seq, time.Now().UTC())
	return err
}

// UpdateClearSeq clears conversation history up to seq for a user.
// Messages with seq <= clearSeq will be hidden for this user.
func (db *DB) UpdateClearSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE members SET clear_seq = $3, updated_at = $4
		WHERE conversation_id = $1 AND user_id = $2
	`, convID, userID, seq, time.Now().UTC())
	return err
}

// ReadReceipt represents a user's read status in a conversation.
type ReadReceipt struct {
	UserID  uuid.UUID
	ReadSeq int
	RecvSeq int
}

// GetReadReceipts returns read receipts for all members of a conversation.
func (db *DB) GetReadReceipts(ctx context.Context, convID uuid.UUID) ([]ReadReceipt, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT user_id, read_seq, recv_seq
		FROM members
		WHERE conversation_id = $1 AND deleted_at IS NULL
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []ReadReceipt
	for rows.Next() {
		var r ReadReceipt
		if err := rows.Scan(&r.UserID, &r.ReadSeq, &r.RecvSeq); err != nil {
			return nil, err
		}
		receipts = append(receipts, r)
	}
	return receipts, rows.Err()
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

// AddRoomMember adds a user to a room with the specified role.
// If the user is already a member (including soft-deleted), this is a no-op.
func (db *DB) AddRoomMember(ctx context.Context, convID, userID uuid.UUID, role string) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		INSERT INTO members (conversation_id, user_id, created_at, updated_at, role)
		VALUES ($1, $2, $3, $3, $4)
		ON CONFLICT (conversation_id, user_id) DO UPDATE SET
			deleted_at = NULL,
			role = EXCLUDED.role,
			updated_at = EXCLUDED.updated_at
	`, convID, userID, now, role)
	return err
}

// RemoveMember soft-deletes a member from a conversation.
func (db *DB) RemoveMember(ctx context.Context, convID, userID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE members SET deleted_at = $3, updated_at = $3
		WHERE conversation_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, convID, userID, now)
	return err
}

// GetMemberRole returns the role of a member in a conversation.
// Returns empty string if not a member.
func (db *DB) GetMemberRole(ctx context.Context, convID, userID uuid.UUID) (string, error) {
	var role string
	err := db.pool.QueryRow(ctx, `
		SELECT role FROM members
		WHERE conversation_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, convID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// UpdateRoomPublic updates a room's public metadata.
func (db *DB) UpdateRoomPublic(ctx context.Context, convID uuid.UUID, public json.RawMessage) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE conversations SET public = $2, updated_at = $3
		WHERE id = $1 AND type = 'room'
	`, convID, public, now)
	return err
}

// SetPinnedMessage sets or clears the pinned message for a conversation.
// Pass messageID as nil to unpin.
func (db *DB) SetPinnedMessage(ctx context.Context, convID uuid.UUID, messageID *uuid.UUID, pinnedBy uuid.UUID) error {
	now := time.Now().UTC()
	if messageID == nil {
		// Unpin
		_, err := db.pool.Exec(ctx, `
			UPDATE conversations
			SET pinned_message_id = NULL, pinned_at = NULL, pinned_by = NULL, updated_at = $2
			WHERE id = $1
		`, convID, now)
		return err
	}
	// Pin
	_, err := db.pool.Exec(ctx, `
		UPDATE conversations
		SET pinned_message_id = $2, pinned_at = $3, pinned_by = $4, updated_at = $3
		WHERE id = $1
	`, convID, messageID, now, pinnedBy)
	return err
}

// GetPinnedMessageSeq returns the seq of the pinned message, or nil if no message is pinned.
func (db *DB) GetPinnedMessageSeq(ctx context.Context, convID uuid.UUID) (*int, error) {
	var seq *int
	err := db.pool.QueryRow(ctx, `
		SELECT m.seq FROM conversations c
		JOIN messages m ON m.id = c.pinned_message_id
		WHERE c.id = $1
	`, convID).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return seq, nil
}

// UpdateConversationDisappearingTTL sets or clears the disappearing messages TTL.
// Pass ttl as nil to disable disappearing messages.
func (db *DB) UpdateConversationDisappearingTTL(ctx context.Context, convID uuid.UUID, ttl *int) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE conversations SET disappearing_ttl = $2, updated_at = $3
		WHERE id = $1
	`, convID, ttl, now)
	return err
}

// GetConversationDisappearingTTL returns the disappearing TTL for a conversation.
func (db *DB) GetConversationDisappearingTTL(ctx context.Context, convID uuid.UUID) (*int, error) {
	var ttl *int
	err := db.pool.QueryRow(ctx, `
		SELECT disappearing_ttl FROM conversations WHERE id = $1
	`, convID).Scan(&ttl)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return ttl, err
}

// UpdateConversationNoScreenshots sets the no-screenshots flag for a conversation.
func (db *DB) UpdateConversationNoScreenshots(ctx context.Context, convID uuid.UUID, noScreenshots bool) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE conversations SET no_screenshots = $2, updated_at = $3
		WHERE id = $1
	`, convID, noScreenshots, now)
	return err
}
