package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Contact represents a user's contact.
type Contact struct {
	UserID    uuid.UUID
	ContactID uuid.UUID
	Source    string
	Nickname  *string
	InviteID  *uuid.UUID
	CreatedAt time.Time
}

// AddContact adds a contact relationship (bidirectional).
func (db *DB) AddContact(ctx context.Context, userID, contactID uuid.UUID, source string, inviteID *uuid.UUID) error {
	now := time.Now().UTC()

	// Add both directions
	_, err := db.pool.Exec(ctx, `
		INSERT INTO contacts (user_id, contact_id, source, invite_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, contact_id) DO NOTHING
	`, userID, contactID, source, inviteID, now)
	if err != nil {
		return err
	}

	_, err = db.pool.Exec(ctx, `
		INSERT INTO contacts (user_id, contact_id, source, invite_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, contact_id) DO NOTHING
	`, contactID, userID, source, inviteID, now)
	return err
}

// GetContacts returns all contacts for a user.
func (db *DB) GetContacts(ctx context.Context, userID uuid.UUID) ([]Contact, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT user_id, contact_id, source, nickname, invite_id, created_at
		FROM contacts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.UserID, &c.ContactID, &c.Source, &c.Nickname, &c.InviteID, &c.CreatedAt); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// IsContact checks if two users are contacts.
func (db *DB) IsContact(ctx context.Context, userID, contactID uuid.UUID) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM contacts
			WHERE user_id = $1 AND contact_id = $2
		)
	`, userID, contactID).Scan(&exists)
	return exists, err
}

// UpdateContactNickname updates a contact's nickname.
func (db *DB) UpdateContactNickname(ctx context.Context, userID, contactID uuid.UUID, nickname *string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE contacts SET nickname = $3
		WHERE user_id = $1 AND contact_id = $2
	`, userID, contactID, nickname)
	return err
}

// RemoveContact removes a contact relationship (bidirectional).
func (db *DB) RemoveContact(ctx context.Context, userID, contactID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM contacts
		WHERE (user_id = $1 AND contact_id = $2) OR (user_id = $2 AND contact_id = $1)
	`, userID, contactID)
	return err
}

// GetPendingInvitesByEmail returns all pending invites for an email address.
func (db *DB) GetPendingInvitesByEmail(ctx context.Context, email string) ([]InviteCode, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, inviter_id, code, email, invitee_name, status, used_at, used_by, created_at, expires_at
		FROM invite_codes
		WHERE email = $1 AND status = 'pending' AND expires_at > NOW()
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []InviteCode
	for rows.Next() {
		var inv InviteCode
		if err := rows.Scan(&inv.ID, &inv.InviterID, &inv.Code, &inv.Email, &inv.InviteeName,
			&inv.Status, &inv.UsedAt, &inv.UsedBy, &inv.CreatedAt, &inv.ExpiresAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}
