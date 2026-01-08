package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InviteCode represents an invite code in the database.
type InviteCode struct {
	ID          uuid.UUID
	InviterID   uuid.UUID
	Code        string
	Email       string
	InviteeName *string
	Status      string // pending, used, expired, revoked
	UsedAt      *time.Time
	UsedBy      *uuid.UUID
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// GenerateInviteCode generates a cryptographically secure 10-digit code.
func GenerateInviteCode() (string, error) {
	// Generate 5 random bytes (40 bits of entropy)
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	// Convert to 10-digit number (use modulo to ensure 10 digits)
	// This gives us a number between 0000000000 and 9999999999
	num := uint64(b[0])<<32 | uint64(b[1])<<24 | uint64(b[2])<<16 | uint64(b[3])<<8 | uint64(b[4])
	code := fmt.Sprintf("%010d", num%10000000000)
	return code, nil
}

// CreateInviteCode creates a new invite code.
func (db *DB) CreateInviteCode(ctx context.Context, inviterID uuid.UUID, email string, inviteeName *string) (*InviteCode, error) {
	// Generate unique code (retry if collision)
	var code string
	var err error
	for i := 0; i < 5; i++ {
		code, err = GenerateInviteCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code: %w", err)
		}

		// Check if code exists
		var exists bool
		err = db.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM invite_codes WHERE code = $1)
		`, code).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if !exists {
			break
		}
		if i == 4 {
			return nil, fmt.Errorf("failed to generate unique code after 5 attempts")
		}
	}

	invite := &InviteCode{}
	err = db.pool.QueryRow(ctx, `
		INSERT INTO invite_codes (inviter_id, code, email, invitee_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, inviter_id, code, email, invitee_name, status, created_at, expires_at
	`, inviterID, code, email, inviteeName).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Email,
		&invite.InviteeName, &invite.Status, &invite.CreatedAt, &invite.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	return invite, nil
}

// GetInviteByCode retrieves an invite code by its code value.
func (db *DB) GetInviteByCode(ctx context.Context, code string) (*InviteCode, error) {
	invite := &InviteCode{}
	var inviteeName, usedBy sql.NullString
	var usedAt sql.NullTime

	err := db.pool.QueryRow(ctx, `
		SELECT id, inviter_id, code, email, invitee_name, status, used_at, used_by, created_at, expires_at
		FROM invite_codes
		WHERE code = $1
	`, code).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Email,
		&inviteeName, &invite.Status, &usedAt, &usedBy,
		&invite.CreatedAt, &invite.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if inviteeName.Valid {
		invite.InviteeName = &inviteeName.String
	}
	if usedAt.Valid {
		invite.UsedAt = &usedAt.Time
	}
	if usedBy.Valid {
		uid, _ := uuid.Parse(usedBy.String)
		invite.UsedBy = &uid
	}

	return invite, nil
}

// UseInviteCode marks an invite code as used and returns the inviter ID.
func (db *DB) UseInviteCode(ctx context.Context, code string, usedByID uuid.UUID) (*InviteCode, error) {
	invite := &InviteCode{}
	var inviteeName sql.NullString

	err := db.pool.QueryRow(ctx, `
		UPDATE invite_codes
		SET status = 'used', used_at = NOW(), used_by = $2
		WHERE code = $1 AND status = 'pending' AND expires_at > NOW()
		RETURNING id, inviter_id, code, email, invitee_name, status, used_at, used_by, created_at, expires_at
	`, code, usedByID).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Email,
		&inviteeName, &invite.Status, &invite.UsedAt, &invite.UsedBy,
		&invite.CreatedAt, &invite.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if inviteeName.Valid {
		invite.InviteeName = &inviteeName.String
	}

	return invite, nil
}

// GetUserInvites retrieves all invite codes created by a user.
func (db *DB) GetUserInvites(ctx context.Context, userID uuid.UUID) ([]*InviteCode, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, inviter_id, code, email, invitee_name, status, used_at, used_by, created_at, expires_at
		FROM invite_codes
		WHERE inviter_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []*InviteCode
	for rows.Next() {
		invite := &InviteCode{}
		var inviteeName, usedBy sql.NullString
		var usedAt sql.NullTime

		err := rows.Scan(
			&invite.ID, &invite.InviterID, &invite.Code, &invite.Email,
			&inviteeName, &invite.Status, &usedAt, &usedBy,
			&invite.CreatedAt, &invite.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}

		if inviteeName.Valid {
			invite.InviteeName = &inviteeName.String
		}
		if usedAt.Valid {
			invite.UsedAt = &usedAt.Time
		}
		if usedBy.Valid {
			uid, _ := uuid.Parse(usedBy.String)
			invite.UsedBy = &uid
		}

		invites = append(invites, invite)
	}

	return invites, nil
}

// RevokeInvite revokes an invite code (only if pending).
func (db *DB) RevokeInvite(ctx context.Context, inviteID uuid.UUID, inviterID uuid.UUID) error {
	result, err := db.pool.Exec(ctx, `
		UPDATE invite_codes
		SET status = 'revoked'
		WHERE id = $1 AND inviter_id = $2 AND status = 'pending'
	`, inviteID, inviterID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("invite not found or already used")
	}
	return nil
}

// ExpireOldInvites marks expired invites (can be run periodically).
func (db *DB) ExpireOldInvites(ctx context.Context) (int64, error) {
	result, err := db.pool.Exec(ctx, `
		UPDATE invite_codes
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at < NOW()
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
