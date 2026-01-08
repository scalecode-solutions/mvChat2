# File Deduplication Implementation

**Priority:** High
**Status:** Not Implemented
**Effort:** Medium (2-3 days)
**Dependencies:** Schema migration

## Summary

The file upload system was supposed to include SHA-256 hash-based deduplication from the FilesAPI backend. The schema defines a `hash` column but the code never calculates or stores it. This means identical files uploaded by different users are stored multiple times, wasting disk space.

## Current State (Broken)

### Schema vs Code Mismatch

```sql
-- Schema expects:
CREATE TABLE files (
    user_id UUID NOT NULL,        -- Code uses "uploader_id"
    hash VARCHAR(64) NOT NULL,    -- Code never sets this
    status INT NOT NULL,          -- Code uses strings
);
```

```go
// Code does:
INSERT INTO files (..., uploader_id, status, ...)
VALUES (..., $4, 'pending', ...)
// No hash parameter - INSERT will fail!
```

### Missing Functionality

1. No hash calculation during upload
2. No `GetFileByHash()` function
3. No deduplication check before saving
4. Access control always returns `true`

## Proposed Implementation

### 1. Schema Migration

```sql
-- Migration: Align schema with code
ALTER TABLE files RENAME COLUMN user_id TO uploader_id;
ALTER TABLE files ALTER COLUMN status TYPE VARCHAR(16);
ALTER TABLE files ALTER COLUMN hash DROP NOT NULL; -- Temporary for migration

-- Add message attachments for reference counting
CREATE TABLE message_attachments (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, file_id)
);
CREATE INDEX idx_attachments_file ON message_attachments(file_id);

-- After backfilling hashes:
-- ALTER TABLE files ALTER COLUMN hash SET NOT NULL;
-- CREATE UNIQUE INDEX idx_files_hash ON files(hash) WHERE deleted_at IS NULL;
```

### 2. Add Streaming Hash Calculation

```go
// media/hash.go
package media

import (
    "crypto/sha256"
    "encoding/base64"
    "io"
)

// CalculateStreamHash computes SHA-256 while writing to a file.
// Uses io.MultiWriter for single-pass efficiency.
func CalculateStreamHash(reader io.Reader, writer io.Writer) (string, int64, error) {
    h := sha256.New()
    mw := io.MultiWriter(writer, h)
    n, err := io.Copy(mw, reader)
    if err != nil {
        return "", 0, err
    }
    return base64.StdEncoding.EncodeToString(h.Sum(nil)), n, nil
}
```

### 3. Add Database Functions

```go
// store/files.go additions

// GetFileByHash retrieves an existing file by content hash.
func (db *DB) GetFileByHash(ctx context.Context, hash string) (*File, error) {
    var f File
    err := db.pool.QueryRow(ctx, `
        SELECT id, created_at, updated_at, uploader_id, status,
               mime_type, size, location, hash, deleted_at
        FROM files
        WHERE hash = $1 AND deleted_at IS NULL
    `, hash).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.UploaderID,
                  &f.Status, &f.MimeType, &f.Size, &f.Location, &f.Hash, &f.DeletedAt)

    if errors.Is(err, pgx.ErrNoRows) {
        return nil, nil
    }
    return &f, err
}

// CreateFileWithHash creates a file record with hash for deduplication.
func (db *DB) CreateFileWithHash(ctx context.Context, uploaderID uuid.UUID,
    mimeType string, size int64, location, hash, originalName string) (*File, error) {

    id := uuid.New()
    now := time.Now().UTC()

    _, err := db.pool.Exec(ctx, `
        INSERT INTO files (id, created_at, updated_at, uploader_id, status,
                          mime_type, size, location, hash, original_name)
        VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9)
    `, id, now, now, uploaderID, mimeType, size, location, hash, originalName)

    if err != nil {
        return nil, err
    }

    return &File{
        ID:           id,
        CreatedAt:    now,
        UpdatedAt:    now,
        UploaderID:   uploaderID,
        Status:       "pending",
        MimeType:     mimeType,
        Size:         size,
        Location:     location,
        Hash:         hash,
        OriginalName: originalName,
    }, nil
}

// CanAccessFile checks if user can access a file.
func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
    var allowed bool
    err := db.pool.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM files
            WHERE id = $1 AND uploader_id = $2 AND deleted_at IS NULL
            UNION
            SELECT 1 FROM message_attachments ma
            JOIN messages m ON ma.message_id = m.id
            JOIN members mem ON m.conversation_id = mem.conversation_id
            WHERE ma.file_id = $1 AND mem.user_id = $2 AND mem.deleted_at IS NULL
        )
    `, fileID, userID).Scan(&allowed)
    return allowed, err
}
```

### 4. Update Upload Handler

```go
// handlers_files.go - Upload with deduplication

func (fh *FileHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
    // ... auth and form parsing unchanged ...

    // Write to temp file while calculating hash
    tempPath := filepath.Join(os.TempDir(), "mvchat_upload_"+uuid.New().String())
    tempFile, err := os.Create(tempPath)
    if err != nil {
        http.Error(w, "failed to create temp file", http.StatusInternalServerError)
        return
    }

    hash, size, err := media.CalculateStreamHash(file, tempFile)
    tempFile.Close()
    if err != nil {
        os.Remove(tempPath)
        http.Error(w, "failed to process upload", http.StatusInternalServerError)
        return
    }

    // DEDUPLICATION CHECK
    existingFile, err := fh.db.GetFileByHash(r.Context(), hash)
    if err != nil {
        os.Remove(tempPath)
        http.Error(w, "database error", http.StatusInternalServerError)
        return
    }

    var fileRecord *store.File

    if existingFile != nil {
        // DEDUP HIT: Reuse existing file
        os.Remove(tempPath)

        fileRecord, err = fh.db.CreateFileWithHash(r.Context(), userID,
            existingFile.MimeType, existingFile.Size,
            existingFile.Location, hash, header.Filename)
        if err != nil {
            http.Error(w, "failed to create file record", http.StatusInternalServerError)
            return
        }

        // Copy metadata from existing file
        if meta, _ := fh.db.GetFileMetadata(r.Context(), existingFile.ID); meta != nil {
            fh.db.CreateFileMetadata(r.Context(), fileRecord.ID,
                meta.Width, meta.Height, meta.Duration, meta.Thumbnail, meta.Extra)
        }

        fh.db.UpdateFileStatus(r.Context(), fileRecord.ID, "ready")
    } else {
        // NEW FILE: Move temp to permanent storage
        fileID := uuid.New()
        path, err := fh.processor.SaveUploadFromPath(r.Context(), fileID, tempPath, mimeType)
        if err != nil {
            os.Remove(tempPath)
            http.Error(w, "failed to save file", http.StatusInternalServerError)
            return
        }

        fileRecord, err = fh.db.CreateFileWithHash(r.Context(), userID,
            mimeType, size, path, hash, header.Filename)
        if err != nil {
            os.Remove(path)
            http.Error(w, "failed to create file record", http.StatusInternalServerError)
            return
        }

        go fh.processMedia(fileRecord.ID, path, mimeType)
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    fmt.Fprintf(w, `{"id":"%s","mime":"%s","size":%d}`,
        fileRecord.ID, fileRecord.MimeType, fileRecord.Size)
}
```

### 5. Update File Struct

```go
// store/files.go

type File struct {
    ID           uuid.UUID  `json:"id"`
    CreatedAt    time.Time  `json:"createdAt"`
    UpdatedAt    time.Time  `json:"updatedAt"`
    UploaderID   uuid.UUID  `json:"uploaderId"`
    Status       string     `json:"status"`
    MimeType     string     `json:"mimeType"`
    Size         int64      `json:"size"`
    Location     string     `json:"-"`
    Hash         string     `json:"-"`          // SHA-256 base64
    OriginalName string     `json:"originalName,omitempty"`
    DeletedAt    *time.Time `json:"deletedAt,omitempty"`
}
```

## Files to Modify

| File | Changes |
|------|---------|
| `store/schema.sql` | Rename column, add attachments table |
| `store/files.go` | Add Hash field, new functions |
| `media/hash.go` | New file for streaming hash |
| `media/media.go` | Add `SaveUploadFromPath` |
| `handlers_files.go` | Dedup flow in upload handler |

## Testing

```go
// store/files_test.go
func TestGetFileByHash(t *testing.T) {
    // Test finding existing file by hash
    // Test returning nil for non-existent hash
}

func TestDeduplication(t *testing.T) {
    // Upload file A, get hash
    // Upload identical file B
    // Verify B points to same location as A
    // Verify only one physical file exists
}

func TestCanAccessFile(t *testing.T) {
    // Test uploader can access
    // Test conversation member can access via attachment
    // Test non-member cannot access
}
```

## Migration Steps

1. Create schema migration file
2. Apply migration (makes hash nullable)
3. Deploy new code
4. Run backfill script to calculate hashes for existing files
5. Apply second migration to make hash NOT NULL + unique index
6. Optionally: deduplicate existing physical files

## Benefits

- **Storage savings**: Identical files stored once
- **Faster uploads**: Skip processing for duplicates
- **Bandwidth savings**: No redundant writes
- **Proper access control**: Security fix included
