package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// InviteCode represents an invite code in the database.
type InviteCode struct {
	ID          uuid.UUID
	InviterID   uuid.UUID
	Code        string  // Short 10-char alphanumeric code for user sharing
	Token       string  // Full cryptographic token for verification
	Email       string  // Invitee's email (used to verify token)
	InviteeName *string
	Status      string // pending, used, expired, revoked
	UsedAt      *time.Time
	UsedBy      *uuid.UUID
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CreateInviteCode creates a new invite code record.
// code: short 10-char alphanumeric code for user sharing
// token: full cryptographic token for verification
func (db *DB) CreateInviteCode(ctx context.Context, inviterID uuid.UUID, code, token, email string, inviteeName *string) (*InviteCode, error) {
	invite := &InviteCode{}
	err := db.pool.QueryRow(ctx, `
		INSERT INTO invite_codes (inviter_id, code, token, email, invitee_name)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, inviter_id, code, token, email, invitee_name, status, created_at, expires_at
	`, inviterID, code, token, email, inviteeName).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Token, &invite.Email,
		&invite.InviteeName, &invite.Status, &invite.CreatedAt, &invite.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	return invite, nil
}

// GetInviteByID retrieves an invite by its ID.
func (db *DB) GetInviteByID(ctx context.Context, id uuid.UUID) (*InviteCode, error) {
	invite := &InviteCode{}
	var inviteeName, usedBy sql.NullString
	var usedAt sql.NullTime

	err := db.pool.QueryRow(ctx, `
		SELECT id, inviter_id, code, token, email, invitee_name, status, used_at, used_by, created_at, expires_at
		FROM invite_codes
		WHERE id = $1
	`, id).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Token, &invite.Email,
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

// GetInviteByCode retrieves a pending invite by its short code.
// This is used when users redeem invites using the short 10-char code.
func (db *DB) GetInviteByCode(ctx context.Context, code string) (*InviteCode, error) {
	invite := &InviteCode{}
	var inviteeName, usedBy sql.NullString
	var usedAt sql.NullTime

	err := db.pool.QueryRow(ctx, `
		SELECT id, inviter_id, code, token, email, invitee_name, status, used_at, used_by, created_at, expires_at
		FROM invite_codes
		WHERE code = $1 AND status = 'pending' AND expires_at > NOW()
	`, code).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Token, &invite.Email,
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

// GetPendingInviteByUsernames finds a pending invite by inviter username and invitee email.
// Used to verify cryptographic invite tokens.
func (db *DB) GetPendingInviteByUsernames(ctx context.Context, inviterUsername, inviteeEmail string) (*InviteCode, error) {
	invite := &InviteCode{}
	var inviteeName sql.NullString

	// Join with auth to match inviter by username
	err := db.pool.QueryRow(ctx, `
		SELECT ic.id, ic.inviter_id, ic.code, ic.token, ic.email, ic.invitee_name, ic.status, ic.created_at, ic.expires_at
		FROM invite_codes ic
		JOIN auth a ON ic.inviter_id = a.user_id AND a.scheme = 'basic'
		WHERE a.uname = $1
		AND ic.email = $2
		AND ic.status = 'pending'
		AND ic.expires_at > NOW()
		ORDER BY ic.created_at DESC
		LIMIT 1
	`, inviterUsername, inviteeEmail).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Token, &invite.Email,
		&inviteeName, &invite.Status, &invite.CreatedAt, &invite.ExpiresAt,
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

// UseInvite marks an invite as used by its ID.
func (db *DB) UseInvite(ctx context.Context, inviteID, usedByID uuid.UUID) (*InviteCode, error) {
	invite := &InviteCode{}
	var inviteeName sql.NullString

	err := db.pool.QueryRow(ctx, `
		UPDATE invite_codes
		SET status = 'used', used_at = NOW(), used_by = $2
		WHERE id = $1 AND status = 'pending' AND expires_at > NOW()
		RETURNING id, inviter_id, code, token, email, invitee_name, status, used_at, used_by, created_at, expires_at
	`, inviteID, usedByID).Scan(
		&invite.ID, &invite.InviterID, &invite.Code, &invite.Token, &invite.Email,
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
		SELECT id, inviter_id, code, token, email, invitee_name, status, used_at, used_by, created_at, expires_at
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
			&invite.ID, &invite.InviterID, &invite.Code, &invite.Token, &invite.Email,
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

// ErrInviteNotFound is returned when an invite cannot be found or modified.
var ErrInviteNotFound = errors.New("invite not found or already used")

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
		return ErrInviteNotFound
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
