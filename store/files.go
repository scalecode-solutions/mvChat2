package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// File represents an uploaded file.
type File struct {
	ID           uuid.UUID  `json:"id"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	UploaderID   uuid.UUID  `json:"uploaderId"`
	Status       string     `json:"status"` // "pending", "ready", "failed"
	MimeType     string     `json:"mimeType"`
	Size         int64      `json:"size"`
	Location     string     `json:"-"`                      // Internal path, not exposed
	Hash         *string    `json:"-"`                      // SHA-256 hash for deduplication
	OriginalName *string    `json:"originalName,omitempty"` // Original filename
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
}

// FileMetadata represents additional metadata for a file.
type FileMetadata struct {
	FileID    uuid.UUID       `json:"fileId"`
	Width     *int            `json:"width,omitempty"`
	Height    *int            `json:"height,omitempty"`
	Duration  *float64        `json:"duration,omitempty"`
	Thumbnail *string         `json:"thumbnail,omitempty"`
	Extra     json.RawMessage `json:"extra,omitempty"`
}

// FileWithMetadata combines file and metadata.
type FileWithMetadata struct {
	File
	Metadata *FileMetadata `json:"metadata,omitempty"`
}

// CreateFile creates a new file record (without hash - for backwards compatibility).
func (db *DB) CreateFile(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location string) (*File, error) {
	return db.CreateFileWithHash(ctx, uploaderID, mimeType, size, location, "", "")
}

// CreateFileWithHash creates a new file record with hash and original name for deduplication.
func (db *DB) CreateFileWithHash(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error) {
	return db.CreateFileWithID(ctx, uuid.New(), uploaderID, mimeType, size, location, hash, originalName)
}

// CreateFileWithID creates a new file record with a specific ID (used when file is already saved to disk with that ID).
func (db *DB) CreateFileWithID(ctx context.Context, fileID, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error) {
	now := time.Now().UTC()

	var hashPtr, namePtr *string
	if hash != "" {
		hashPtr = &hash
	}
	if originalName != "" {
		namePtr = &originalName
	}

	_, err := db.pool.Exec(ctx, `
		INSERT INTO files (id, created_at, updated_at, uploader_id, status, mime_type, size, location, hash, original_name)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9)
	`, fileID, now, now, uploaderID, mimeType, size, location, hashPtr, namePtr)

	if err != nil {
		return nil, err
	}

	return &File{
		ID:           fileID,
		CreatedAt:    now,
		UpdatedAt:    now,
		UploaderID:   uploaderID,
		Status:       "pending",
		MimeType:     mimeType,
		Size:         size,
		Location:     location,
		Hash:         hashPtr,
		OriginalName: namePtr,
	}, nil
}

// UpdateFileStatus updates a file's status.
func (db *DB) UpdateFileStatus(ctx context.Context, fileID uuid.UUID, status string) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE files SET status = $2, updated_at = $3 WHERE id = $1
	`, fileID, status, now)
	return err
}

// GetFileByID retrieves a file by ID.
func (db *DB) GetFileByID(ctx context.Context, id uuid.UUID) (*File, error) {
	var f File
	err := db.pool.QueryRow(ctx, `
		SELECT id, created_at, updated_at, uploader_id, status, mime_type, size, location, hash, original_name, deleted_at
		FROM files WHERE id = $1
	`, id).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.UploaderID, &f.Status, &f.MimeType, &f.Size, &f.Location, &f.Hash, &f.OriginalName, &f.DeletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetFileByHash retrieves a ready file by its content hash.
// Returns nil if no matching file exists.
func (db *DB) GetFileByHash(ctx context.Context, hash string) (*File, error) {
	var f File
	err := db.pool.QueryRow(ctx, `
		SELECT id, created_at, updated_at, uploader_id, status, mime_type, size, location, hash, original_name, deleted_at
		FROM files
		WHERE hash = $1 AND status = 'ready' AND deleted_at IS NULL
		LIMIT 1
	`, hash).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.UploaderID, &f.Status, &f.MimeType, &f.Size, &f.Location, &f.Hash, &f.OriginalName, &f.DeletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// UpdateFileLocation updates the file's storage location.
func (db *DB) UpdateFileLocation(ctx context.Context, fileID uuid.UUID, location string) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE files SET location = $2, updated_at = $3 WHERE id = $1
	`, fileID, location, now)
	return err
}

// GetFileWithMetadata retrieves a file with its metadata.
func (db *DB) GetFileWithMetadata(ctx context.Context, id uuid.UUID) (*FileWithMetadata, error) {
	f, err := db.GetFileByID(ctx, id)
	if err != nil || f == nil {
		return nil, err
	}

	fwm := &FileWithMetadata{File: *f}

	var meta FileMetadata
	err = db.pool.QueryRow(ctx, `
		SELECT file_id, width, height, duration, thumbnail, extra
		FROM file_metadata WHERE file_id = $1
	`, id).Scan(&meta.FileID, &meta.Width, &meta.Height, &meta.Duration, &meta.Thumbnail, &meta.Extra)

	if err == nil {
		fwm.Metadata = &meta
	}

	return fwm, nil
}

// CreateFileMetadata creates metadata for a file.
func (db *DB) CreateFileMetadata(ctx context.Context, fileID uuid.UUID, width, height *int, duration *float64, thumbnail *string, extra json.RawMessage) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO file_metadata (file_id, width, height, duration, thumbnail, extra)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (file_id) DO UPDATE SET
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			duration = EXCLUDED.duration,
			thumbnail = EXCLUDED.thumbnail,
			extra = EXCLUDED.extra
	`, fileID, width, height, duration, thumbnail, extra)
	return err
}

// GetFileMetadata retrieves metadata for a file.
func (db *DB) GetFileMetadata(ctx context.Context, fileID uuid.UUID) (*FileMetadata, error) {
	var meta FileMetadata
	err := db.pool.QueryRow(ctx, `
		SELECT file_id, width, height, duration, thumbnail, extra
		FROM file_metadata WHERE file_id = $1
	`, fileID).Scan(&meta.FileID, &meta.Width, &meta.Height, &meta.Duration, &meta.Thumbnail, &meta.Extra)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// DeleteFile soft-deletes a file.
func (db *DB) DeleteFile(ctx context.Context, fileID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, `
		UPDATE files SET deleted_at = $2, updated_at = $2 WHERE id = $1
	`, fileID, now)
	return err
}

// CanAccessFile checks if a user can access a file.
// Access is granted if:
// 1. User is the uploader
// 2. File is referenced in a message in a conversation the user is a member of
func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
	// Check if user is the uploader OR if file is in a message in a conversation they're a member of
	// File references are stored in message content (Irido format) as media[].ref = file UUID
	var hasAccess bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(
			-- User is the uploader
			SELECT 1 FROM files
			WHERE id = $1 AND uploader_id = $2 AND deleted_at IS NULL

			UNION ALL

			-- File is referenced in a message in a conversation user is member of
			SELECT 1 FROM messages m
			JOIN members mem ON m.conversation_id = mem.conversation_id
			WHERE mem.user_id = $2
			AND mem.deleted_at IS NULL
			AND m.deleted_at IS NULL
			AND (
				-- Check if file ID appears in the message content (BYTEA) or head (JSONB)
				-- Content is Irido format with media[].ref containing file UUIDs
				m.content::text LIKE '%' || $1::text || '%'
				OR m.head::text LIKE '%' || $1::text || '%'
			)
		)
	`, fileID, userID).Scan(&hasAccess)

	if err != nil {
		return false, err
	}

	return hasAccess, nil
}
