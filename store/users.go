package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User represents a user in the database.
type User struct {
	ID                 uuid.UUID       `json:"id"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	State              string          `json:"state"`
	Public             json.RawMessage `json:"public,omitempty"`
	LastSeen           *time.Time      `json:"lastSeen,omitempty"`
	UserAgent          string          `json:"userAgent,omitempty"`
	MustChangePassword bool            `json:"mustChangePassword,omitempty"`
}

// AuthRecord represents an authentication record.
type AuthRecord struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Scheme    string
	Secret    string
	Uname     *string
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// CreateUser creates a new user and returns the user ID.
func (db *DB) CreateUser(ctx context.Context, public json.RawMessage) (uuid.UUID, error) {
	return db.CreateUserWithOptions(ctx, public, false)
}

// CreateUserWithOptions creates a new user with additional options.
func (db *DB) CreateUserWithOptions(ctx context.Context, public json.RawMessage, mustChangePassword bool) (uuid.UUID, error) {
	id := uuid.New()
	now := time.Now().UTC()

	_, err := db.pool.Exec(ctx, `
		INSERT INTO users (id, created_at, updated_at, state, public, must_change_password)
		VALUES ($1, $2, $3, 'ok', $4, $5)
	`, id, now, now, public, mustChangePassword)

	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := db.pool.QueryRow(ctx, `
		SELECT id, created_at, updated_at, state, public, last_seen, user_agent, must_change_password
		FROM users WHERE id = $1 AND state != 'deleted'
	`, id).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.State, &user.Public, &user.LastSeen, &user.UserAgent, &user.MustChangePassword)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUserLastSeen updates the user's last seen timestamp and user agent.
func (db *DB) UpdateUserLastSeen(ctx context.Context, userID uuid.UUID, userAgent string) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE users SET last_seen = $1, user_agent = $2, updated_at = $1
		WHERE id = $3
	`, now, userAgent, userID)
	return err
}

// UpdateUserPublic updates the user's public data.
func (db *DB) UpdateUserPublic(ctx context.Context, userID uuid.UUID, public json.RawMessage) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE users SET public = $1, updated_at = $2
		WHERE id = $3
	`, public, now, userID)
	return err
}

// CreateAuthRecord creates a new authentication record.
func (db *DB) CreateAuthRecord(ctx context.Context, userID uuid.UUID, scheme, secret string, uname *string) error {
	id := uuid.New()
	now := time.Now().UTC()

	_, err := db.pool.Exec(ctx, `
		INSERT INTO auth (id, user_id, scheme, secret, uname, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, userID, scheme, secret, uname, now)

	return err
}

// GetAuthByUsername retrieves an auth record by username (for basic auth).
func (db *DB) GetAuthByUsername(ctx context.Context, username string) (*AuthRecord, error) {
	var auth AuthRecord
	err := db.pool.QueryRow(ctx, `
		SELECT id, user_id, scheme, secret, uname, expires_at, created_at
		FROM auth WHERE scheme = 'basic' AND uname = $1
	`, username).Scan(&auth.ID, &auth.UserID, &auth.Scheme, &auth.Secret, &auth.Uname, &auth.ExpiresAt, &auth.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

// UsernameExists checks if a username is already taken.
func (db *DB) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM auth WHERE scheme = 'basic' AND uname = $1)
	`, username).Scan(&exists)
	return exists, err
}

// UpdatePassword updates the user's password (basic auth secret).
func (db *DB) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE auth SET secret = $1
		WHERE user_id = $2 AND scheme = 'basic'
	`, hashedPassword, userID)
	return err
}

// ClearMustChangePassword clears the must_change_password flag for a user.
// This should be called after a successful password change.
func (db *DB) ClearMustChangePassword(ctx context.Context, userID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE users SET must_change_password = FALSE, updated_at = $2
		WHERE id = $1
	`, userID, time.Now().UTC())
	return err
}

// GetAuthByUserID retrieves the basic auth record for a user.
func (db *DB) GetAuthByUserID(ctx context.Context, userID uuid.UUID) (*AuthRecord, error) {
	var auth AuthRecord
	err := db.pool.QueryRow(ctx, `
		SELECT id, user_id, scheme, secret, uname, expires_at, created_at
		FROM auth WHERE user_id = $1 AND scheme = 'basic'
	`, userID).Scan(&auth.ID, &auth.UserID, &auth.Scheme, &auth.Secret, &auth.Uname, &auth.ExpiresAt, &auth.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

// GetUserUsername retrieves the username for a user (from basic auth).
func (db *DB) GetUserUsername(ctx context.Context, userID uuid.UUID) (string, error) {
	var uname string
	err := db.pool.QueryRow(ctx, `
		SELECT uname FROM auth WHERE user_id = $1 AND scheme = 'basic'
	`, userID).Scan(&uname)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return uname, err
}

// SearchUsers searches for users by display name (from public.fn field).
func (db *DB) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	rows, err := db.pool.Query(ctx, `
		SELECT id, created_at, updated_at, state, public, last_seen, user_agent, must_change_password
		FROM users
		WHERE state = 'ok'
		AND public->>'fn' ILIKE '%' || $1 || '%'
		ORDER BY public->>'fn'
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.State, &user.Public, &user.LastSeen, &user.UserAgent, &user.MustChangePassword); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
