package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Message represents a message in a conversation.
type Message struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID uuid.UUID       `json:"conversationId"`
	Seq            int             `json:"seq"`
	FromUserID     uuid.UUID       `json:"from"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Content        []byte          `json:"content"` // Encrypted Irido content
	Head           json.RawMessage `json:"head,omitempty"`
	DeletedAt      *time.Time      `json:"deletedAt,omitempty"`
	// View-once message support
	ViewOnce    bool `json:"viewOnce,omitempty"`
	ViewOnceTTL *int `json:"viewOnceTTL,omitempty"` // seconds: 10, 30, 60, 300, 3600, 86400, 604800
}

// MessageDeletion represents a soft delete for a specific user.
type MessageDeletion struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	DeletedAt time.Time
}

// CreateMessage creates a new message and returns it with the assigned sequence number.
func (db *DB) CreateMessage(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage) (*Message, error) {
	now := time.Now().UTC()
	msgID := uuid.New()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Get next sequence number and update conversation
	var seq int
	err = tx.QueryRow(ctx, `
		UPDATE conversations 
		SET last_seq = last_seq + 1, last_msg_at = $2, updated_at = $2
		WHERE id = $1
		RETURNING last_seq
	`, convID, now).Scan(&seq)
	if err != nil {
		return nil, err
	}

	// Insert message
	_, err = tx.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, seq, from_user_id, created_at, updated_at, content, head)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, msgID, convID, seq, fromUserID, now, now, content, head)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &Message{
		ID:             msgID,
		ConversationID: convID,
		Seq:            seq,
		FromUserID:     fromUserID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Content:        content,
		Head:           head,
	}, nil
}

// GetMessages retrieves messages from a conversation with pagination.
// Returns messages with seq < before, limited to limit count, ordered by seq DESC.
// Excludes messages that have expired for the requesting user.
func (db *DB) GetMessages(ctx context.Context, convID, userID uuid.UUID, before, limit int, clearSeq int) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var query string
	var args []any

	if before > 0 {
		query = `
			SELECT m.id, m.conversation_id, m.seq, m.from_user_id, m.created_at, m.updated_at, m.content, m.head, m.deleted_at, m.view_once, m.view_once_ttl
			FROM messages m
			LEFT JOIN message_deletions md ON m.id = md.message_id AND md.user_id = $2
			LEFT JOIN message_reads mr ON m.id = mr.message_id AND mr.user_id = $2
			WHERE m.conversation_id = $1
			AND m.seq > $5
			AND m.seq < $4
			AND md.message_id IS NULL
			AND (mr.expired IS NULL OR mr.expired = FALSE)
			ORDER BY m.seq DESC LIMIT $3
		`
		args = []any{convID, userID, limit, before, clearSeq}
	} else {
		query = `
			SELECT m.id, m.conversation_id, m.seq, m.from_user_id, m.created_at, m.updated_at, m.content, m.head, m.deleted_at, m.view_once, m.view_once_ttl
			FROM messages m
			LEFT JOIN message_deletions md ON m.id = md.message_id AND md.user_id = $2
			LEFT JOIN message_reads mr ON m.id = mr.message_id AND mr.user_id = $2
			WHERE m.conversation_id = $1
			AND m.seq > $4
			AND md.message_id IS NULL
			AND (mr.expired IS NULL OR mr.expired = FALSE)
			ORDER BY m.seq DESC LIMIT $3
		`
		args = []any{convID, userID, limit, clearSeq}
	}

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Seq, &msg.FromUserID,
			&msg.CreatedAt, &msg.UpdatedAt, &msg.Content, &msg.Head, &msg.DeletedAt,
			&msg.ViewOnce, &msg.ViewOnceTTL); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// GetMessageBySeq retrieves a single message by conversation and sequence number.
func (db *DB) GetMessageBySeq(ctx context.Context, convID uuid.UUID, seq int) (*Message, error) {
	var msg Message
	err := db.pool.QueryRow(ctx, `
		SELECT id, conversation_id, seq, from_user_id, created_at, updated_at, content, head, deleted_at, view_once, view_once_ttl
		FROM messages WHERE conversation_id = $1 AND seq = $2
	`, convID, seq).Scan(&msg.ID, &msg.ConversationID, &msg.Seq, &msg.FromUserID,
		&msg.CreatedAt, &msg.UpdatedAt, &msg.Content, &msg.Head, &msg.DeletedAt,
		&msg.ViewOnce, &msg.ViewOnceTTL)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// EditMessage updates a message's content and increments edit count.
func (db *DB) EditMessage(ctx context.Context, convID uuid.UUID, seq int, content []byte) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE messages 
		SET content = $3, updated_at = $4,
			head = COALESCE(head, '{}'::jsonb) || jsonb_build_object('edit_count', COALESCE((head->>'edit_count')::int, 0) + 1, 'edited_at', $4::timestamptz)
		WHERE conversation_id = $1 AND seq = $2 AND deleted_at IS NULL
	`, convID, seq, content, now)
	return err
}

// UnsendMessage marks a message as unsent (soft delete for everyone).
// Only usable within the unsend time window. Content is preserved for audit.
func (db *DB) UnsendMessage(ctx context.Context, convID uuid.UUID, seq int) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE messages SET deleted_at = $3, updated_at = $3,
			head = COALESCE(head, '{}'::jsonb) || '{"unsent": true}'::jsonb
		WHERE conversation_id = $1 AND seq = $2
	`, convID, seq, now)
	return err
}

// DeleteMessageForEveryone marks a message as deleted for all participants (soft delete).
// No time limit. Content is preserved for audit.
func (db *DB) DeleteMessageForEveryone(ctx context.Context, convID uuid.UUID, seq int) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE messages SET deleted_at = $3, updated_at = $3
		WHERE conversation_id = $1 AND seq = $2
	`, convID, seq, now)
	return err
}

// DeleteMessageForUser creates a soft delete entry for a specific user.
func (db *DB) DeleteMessageForUser(ctx context.Context, msgID, userID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		INSERT INTO message_deletions (message_id, user_id, deleted_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id) DO NOTHING
	`, msgID, userID, now)
	return err
}

// AddReaction adds or toggles a reaction on a message.
// Uses SELECT FOR UPDATE to prevent race conditions when multiple users react simultaneously.
func (db *DB) AddReaction(ctx context.Context, convID uuid.UUID, seq int, userID uuid.UUID, emoji string) error {
	now := time.Now().UTC()
	userIDStr := userID.String()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock the row while we modify it to prevent race conditions
	var head json.RawMessage
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(head, '{}'::jsonb) FROM messages
		WHERE conversation_id = $1 AND seq = $2
		FOR UPDATE
	`, convID, seq).Scan(&head)
	if err != nil {
		return err
	}

	// Parse and update reactions
	var headMap map[string]any
	if err := json.Unmarshal(head, &headMap); err != nil {
		headMap = make(map[string]any)
	}

	reactions, ok := headMap["reactions"].(map[string]any)
	if !ok {
		reactions = make(map[string]any)
	}

	emojiUsers, ok := reactions[emoji].([]any)
	if !ok {
		emojiUsers = []any{}
	}

	// Toggle: remove if exists, add if not
	found := false
	newUsers := []any{}
	for _, u := range emojiUsers {
		if u == userIDStr {
			found = true
		} else {
			newUsers = append(newUsers, u)
		}
	}
	if !found {
		newUsers = append(newUsers, userIDStr)
	}

	if len(newUsers) == 0 {
		delete(reactions, emoji)
	} else {
		reactions[emoji] = newUsers
	}

	if len(reactions) == 0 {
		delete(headMap, "reactions")
	} else {
		headMap["reactions"] = reactions
	}

	newHead, err := json.Marshal(headMap)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE messages SET head = $3, updated_at = $4
		WHERE conversation_id = $1 AND seq = $2
	`, convID, seq, newHead, now)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetEditCount returns the edit count for a message.
func (db *DB) GetEditCount(ctx context.Context, convID uuid.UUID, seq int) (int, error) {
	var head json.RawMessage
	err := db.pool.QueryRow(ctx, `
		SELECT COALESCE(head, '{}'::jsonb) FROM messages
		WHERE conversation_id = $1 AND seq = $2
	`, convID, seq).Scan(&head)
	if err != nil {
		return 0, err
	}

	var headMap map[string]any
	if err := json.Unmarshal(head, &headMap); err != nil {
		return 0, nil
	}

	if count, ok := headMap["edit_count"].(float64); ok {
		return int(count), nil
	}
	return 0, nil
}

// CreateMessageWithViewOnce creates a new message with optional view-once settings.
func (db *DB) CreateMessageWithViewOnce(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage, viewOnce bool, viewOnceTTL *int) (*Message, error) {
	now := time.Now().UTC()
	msgID := uuid.New()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Get next sequence number and update conversation
	var seq int
	err = tx.QueryRow(ctx, `
		UPDATE conversations
		SET last_seq = last_seq + 1, last_msg_at = $2, updated_at = $2
		WHERE id = $1
		RETURNING last_seq
	`, convID, now).Scan(&seq)
	if err != nil {
		return nil, err
	}

	// Insert message with view_once fields
	_, err = tx.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, seq, from_user_id, created_at, updated_at, content, head, view_once, view_once_ttl)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, msgID, convID, seq, fromUserID, now, now, content, head, viewOnce, viewOnceTTL)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &Message{
		ID:             msgID,
		ConversationID: convID,
		Seq:            seq,
		FromUserID:     fromUserID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Content:        content,
		Head:           head,
		ViewOnce:       viewOnce,
		ViewOnceTTL:    viewOnceTTL,
	}, nil
}

// MessageRead represents a user's read of a specific message.
type MessageRead struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	ReadAt    time.Time
	ExpiresAt *time.Time
	Expired   bool
}

// RecordMessageRead records that a user has read a message and calculates expiration.
// For view-once messages, uses the message's view_once_ttl.
// For conversation disappearing messages, uses the conversation's disappearing_ttl.
func (db *DB) RecordMessageRead(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error) {
	now := time.Now().UTC()

	// Get message info and conversation TTL in one query
	var viewOnce bool
	var viewOnceTTL *int
	var convDisappearingTTL *int
	var fromUserID uuid.UUID

	err := db.pool.QueryRow(ctx, `
		SELECT m.view_once, m.view_once_ttl, c.disappearing_ttl, m.from_user_id
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.id = $1
	`, messageID).Scan(&viewOnce, &viewOnceTTL, &convDisappearingTTL, &fromUserID)
	if err != nil {
		return nil, err
	}

	// Don't track reads for sender's own messages
	if fromUserID == userID {
		return nil, nil
	}

	// Calculate expires_at based on TTL
	var expiresAt *time.Time
	if viewOnce && viewOnceTTL != nil {
		t := now.Add(time.Duration(*viewOnceTTL) * time.Second)
		expiresAt = &t
	} else if convDisappearingTTL != nil {
		t := now.Add(time.Duration(*convDisappearingTTL) * time.Second)
		expiresAt = &t
	}

	// Upsert the read record (don't update if already exists - first read wins)
	_, err = db.pool.Exec(ctx, `
		INSERT INTO message_reads (message_id, user_id, read_at, expires_at, expired)
		VALUES ($1, $2, $3, $4, FALSE)
		ON CONFLICT (message_id, user_id) DO NOTHING
	`, messageID, userID, now, expiresAt)
	if err != nil {
		return nil, err
	}

	return &MessageRead{
		MessageID: messageID,
		UserID:    userID,
		ReadAt:    now,
		ExpiresAt: expiresAt,
		Expired:   false,
	}, nil
}

// GetMessageRead returns the read record for a user and message.
func (db *DB) GetMessageRead(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error) {
	var mr MessageRead
	err := db.pool.QueryRow(ctx, `
		SELECT message_id, user_id, read_at, expires_at, expired
		FROM message_reads
		WHERE message_id = $1 AND user_id = $2
	`, messageID, userID).Scan(&mr.MessageID, &mr.UserID, &mr.ReadAt, &mr.ExpiresAt, &mr.Expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &mr, nil
}

// ExpireReadMessages marks messages as expired for users whose TTL has passed.
// Returns the number of messages expired.
func (db *DB) ExpireReadMessages(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	result, err := db.pool.Exec(ctx, `
		UPDATE message_reads
		SET expired = TRUE
		WHERE expires_at IS NOT NULL
		AND expires_at <= $1
		AND NOT expired
	`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// IsMessageExpiredForUser checks if a message has expired for a specific user.
func (db *DB) IsMessageExpiredForUser(ctx context.Context, messageID, userID uuid.UUID) (bool, error) {
	var expired bool
	err := db.pool.QueryRow(ctx, `
		SELECT expired FROM message_reads
		WHERE message_id = $1 AND user_id = $2
	`, messageID, userID).Scan(&expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // Not read yet, so not expired
	}
	if err != nil {
		return false, err
	}
	return expired, nil
}

// GetMessageByID retrieves a message by its ID.
func (db *DB) GetMessageByID(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	var msg Message
	err := db.pool.QueryRow(ctx, `
		SELECT id, conversation_id, seq, from_user_id, created_at, updated_at, content, head, deleted_at, view_once, view_once_ttl
		FROM messages WHERE id = $1
	`, messageID).Scan(&msg.ID, &msg.ConversationID, &msg.Seq, &msg.FromUserID,
		&msg.CreatedAt, &msg.UpdatedAt, &msg.Content, &msg.Head, &msg.DeletedAt, &msg.ViewOnce, &msg.ViewOnceTTL)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetMessagesMentioningUser retrieves messages that mention a specific user.
// Uses the GIN index on head->'mentions' for efficient querying.
func (db *DB) GetMessagesMentioningUser(ctx context.Context, userID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := db.pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.seq, m.from_user_id, m.created_at, m.updated_at, m.content, m.head, m.deleted_at, m.view_once, m.view_once_ttl
		FROM messages m
		JOIN members mem ON m.conversation_id = mem.conversation_id AND mem.user_id = $1 AND mem.deleted_at IS NULL
		WHERE m.head->'mentions' @> $2::jsonb
			AND m.deleted_at IS NULL
		ORDER BY m.created_at DESC
		LIMIT $3
	`, userID, `[{"userId":"`+userID.String()+`"}]`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Seq, &msg.FromUserID,
			&msg.CreatedAt, &msg.UpdatedAt, &msg.Content, &msg.Head, &msg.DeletedAt, &msg.ViewOnce, &msg.ViewOnceTTL); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}
