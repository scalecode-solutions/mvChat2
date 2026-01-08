# File Deduplication Analysis & Design

## Executive Summary

The mvChat2 codebase has a **broken file storage implementation** that was supposed to include SHA-256 hash-based deduplication from the FilesAPI backend. The schema defines a `hash` column but the code never calculates or stores it, meaning deduplication does not work.

This document compares three implementations:
1. **FilesAPI** (C# .NET) - The original reference implementation
2. **Original mvChat** (Go/Tinode-based) - Working adaptation with deduplication
3. **Current mvChat2** (Go) - Broken implementation missing deduplication

---

## Implementation Comparison

### 1. FilesAPI (C# .NET) - Reference Implementation

**Architecture:**
```
IStorageService -> StorageService
    -> IStorageRepository (raw file storage)
    -> IFileDetailsRepository (metadata + hash tracking)
    -> IPreviewService (thumbnails/previews)
```

**Key Design Patterns:**

1. **Separation of Concerns:**
   - `StorageId` = physical file on disk (GUID)
   - `Id` = logical file reference (GUID)
   - `HashId` = SHA-256 content hash (base64)
   - Multiple logical files can share one physical file

2. **Deduplication Flow:**
   ```csharp
   // StorageService.cs:112-153
   public async Task<FileDetails> UploadFileAsync(Stream stream, FileDetails fileDetails)
   {
       // 1. Write to temp file first
       using var fileHelper = new FileHelper(stream, fileDetails.Name);

       // 2. Calculate hash from temp file
       var hashId = SHA256CheckSum(fileHelper.GetFilePath());

       // 3. Check for existing file with same hash
       var existingFile = await _fileDetailsRepository.GetFileDetailsByHashIdAsync(hashId);

       if (existingFile != default)
       {
           // 4a. DEDUP HIT: Return existing file, optionally update metadata
           existingFile.LastModified = DateTime.UtcNow;
           await _fileDetailsRepository.UpdateFileDetailsAsync(existingFile.Id, existingFile);
           return existingFile;
       }

       // 4b. NEW FILE: Save to permanent storage
       var id = await _storageRepository.UploadFileAsync(fileStream, fileDetails.Name);
       fileDetails.StorageId = id.ToString();
       fileDetails.HashId = hashId;
       await _fileDetailsRepository.AddFileDetailsAsync(fileDetails);

       // 5. Generate previews (async is OK here)
       await ProcessFileAsync(fileHelper.GetFilePath(), fileDetails);

       return fileDetails;
   }
   ```

3. **Hash Calculation:**
   ```csharp
   public string SHA256CheckSum(string filePath)
   {
       using var SHA256 = System.Security.Cryptography.SHA256.Create();
       using var fileStream = File.OpenRead(filePath);
       return Convert.ToBase64String(SHA256.ComputeHash(fileStream));
   }
   ```

4. **Deletion with Reference Counting:**
   ```csharp
   // Only delete physical file if no other records reference it
   var otherFiles = await _fileDetailsRepository.GetAllFileDetailsAsync();
   var hasOtherReferences = otherFiles.Any(f => f.StorageId == storageId);
   if (!hasOtherReferences)
   {
       await _storageRepository.DeleteFileAsync(storageId);
   }
   ```

5. **Database Schema:**
   ```sql
   CREATE TABLE FileDetails (
       Id VARCHAR(50) PRIMARY KEY,
       StorageId VARCHAR(50) NOT NULL,
       HashId VARCHAR(100) NOT NULL UNIQUE,  -- Index for dedup lookup
       Name VARCHAR(255) NOT NULL,
       Size BIGINT,
       AddedDate TIMESTAMP,
       AddedBy VARCHAR(100),
       ContentType VARCHAR(100),
       -- Preview metadata
       Width INT,
       Height INT,
       HasThumbnail BOOLEAN,
       HasPreview BOOLEAN,
       -- Extended metadata as JSONB
       MetadataJson JSONB
   );
   CREATE UNIQUE INDEX IX_FileDetails_HashId ON FileDetails(HashId);
   ```

**Strengths:**
- Clean separation between physical storage and logical references
- Hash uniqueness enforced at database level
- Reference counting for safe deletion
- Flexible metadata storage with JSONB

**Weaknesses:**
- Hash calculated after writing to temp file (two disk writes)
- `GetAllFileDetailsAsync()` for reference counting is O(n) - inefficient

---

### 2. Original mvChat (Go/Tinode-based) - Working Implementation

**Architecture:**
```
media.Handler interface -> fs.fshandler
    -> preview.Service (thumbnails)
    -> store.Files (database operations)
```

**Key Design Patterns:**

1. **Streaming Hash Calculation (Efficient!):**
   ```go
   // hash.go:27-42
   func CalculateStreamHash(reader io.Reader, writer io.Writer) (string, int64, error) {
       h := sha256.New()
       // MultiWriter writes to BOTH file and hasher simultaneously
       mw := io.MultiWriter(writer, h)
       n, err := io.Copy(mw, reader)
       if err != nil {
           return "", 0, fmt.Errorf("failed to copy and hash: %w", err)
       }
       return base64.StdEncoding.EncodeToString(h.Sum(nil)), n, nil
   }
   ```
   This is more efficient than FilesAPI - hash is calculated during the initial write, not after.

2. **Deduplication Flow:**
   ```go
   // filesys.go:162-264
   func (fh *fshandler) Upload(fdef *types.FileDef, file io.Reader) (string, int64, error) {
       // 1. Create temp file
       tempPath := filepath.Join(os.TempDir(), "mvchat_upload_"+fileName)
       tempFile, err := os.Create(tempPath)

       // 2. Stream to temp file WHILE calculating hash (single pass!)
       hashId, size, err := preview.CalculateStreamHash(file, tempFile)
       tempFile.Close()

       // 3. Check for existing file with same hash
       existingMeta, existingLocation, err := store.Files.MetadataGetByHash(hashId)

       if existingMeta != nil && existingLocation != "" {
           // 4a. DEDUP HIT: Delete temp, use existing location
           os.Remove(tempPath)
           location = existingLocation
           logs.Info.Printf("Dedup hit: hash %s already exists at %s", hashId, location)
       } else {
           // 4b. NEW FILE: Move temp to permanent location
           os.Rename(tempPath, location)
       }

       // 5. Create metadata SYNCHRONOUSLY (critical for dedup to work!)
       fh.createFileMetadata(fdef, hashId, existingMeta)

       // 6. Generate thumbnails asynchronously (only for new files)
       if existingMeta == nil && fh.previewService.ShouldGenerateThumbnail(fdef.MimeType) {
           go fh.generateThumbnailsAsync(location, fdef.MimeType, fdef.Id, hashId)
       }

       return fh.ServeURL + fname, size, nil
   }
   ```

3. **Directory Structure (Date-based):**
   ```go
   // usr/{userId}/{year}/{month}/{day}/{type}/filename.ext
   subDir := filepath.Join(
       "usr",
       userId,
       fmt.Sprintf("%d", now.Year()),
       fmt.Sprintf("%02d", int(now.Month())),
       fmt.Sprintf("%02d", now.Day()),
       typeDir,  // "images", "videos", "documents", "other"
   )
   ```

4. **Database Interface:**
   ```go
   type FilePersistenceInterface interface {
       StartUpload(fd *types.FileDef) error
       FinishUpload(fd *types.FileDef, success bool, size int64) (*types.FileDef, error)
       Get(fid string) (*types.FileDef, error)
       DeleteUnused(olderThan time.Time, limit int) error
       LinkAttachments(topic string, msgId types.UserId, attachments []string) error
       // Metadata operations with hash lookup
       MetadataCreate(meta *types.FileMetadata) error
       MetadataGet(fileId int64) (*types.FileMetadata, error)
       MetadataGetByHash(hashId string) (*types.FileMetadata, string, error)  // KEY!
       MetadataUpdate(fileId int64, meta *types.FileMetadata) error
   }
   ```

**Strengths:**
- Streaming hash calculation (single disk pass)
- Synchronous metadata creation ensures dedup works for rapid uploads
- Date-based directory structure scales better
- Copies thumbnail metadata from existing file on dedup hit

**Weaknesses:**
- Uses `os.Rename` then falls back to copy (could fail on cross-device)
- No explicit reference counting for deletion

---

### 3. Current mvChat2 (Go) - BROKEN Implementation

**What's Wrong:**

1. **Schema vs Code Mismatch:**
   ```sql
   -- Schema (schema.sql:187-207)
   CREATE TABLE files (
       id UUID PRIMARY KEY,
       user_id UUID NOT NULL,      -- Code uses "uploader_id"!
       hash VARCHAR(64) NOT NULL,  -- Code NEVER sets this!
       status INT NOT NULL,        -- Code uses strings like 'pending'!
       original_name VARCHAR(512), -- Code doesn't use this
       ...
   );
   ```

   ```go
   // Code (files.go:43-50)
   _, err := db.pool.Exec(ctx, `
       INSERT INTO files (id, created_at, updated_at, uploader_id, status, ...)
       VALUES ($1, $2, $3, $4, 'pending', ...)
   `, id, now, now, uploaderID, mimeType, size, location)
   // NOTE: No hash parameter! INSERT will fail due to NOT NULL constraint!
   ```

2. **No Hash Calculation:**
   ```go
   // handlers_files.go:82-95
   fileRecord, err := fh.db.CreateFile(r.Context(), userID, mimeType, header.Size, "")
   path, err := fh.processor.SaveUpload(r.Context(), fileID, file, mimeType)
   // No hash calculated, no dedup check, file always saved
   ```

3. **No Dedup Check:**
   - Missing `GetFileByHash()` function
   - No check before saving file
   - Every upload creates a new file

4. **File Metadata Schema Mismatch:**
   ```sql
   -- Schema
   CREATE TABLE file_metadata (
       file_id UUID PRIMARY KEY,
       width INT,
       height INT,
       duration_seconds INT,    -- Code uses "duration" (float)
       has_thumbnail BOOLEAN,
       thumbnail_path VARCHAR,  -- Code uses "thumbnail" (base64 data)
       metadata JSONB,          -- Code uses "extra"
       ...
   );
   ```

   ```go
   // Code
   type FileMetadata struct {
       FileID    uuid.UUID
       Width     *int
       Height    *int
       Duration  *float64        // Schema: INT duration_seconds
       Thumbnail *string         // Schema: thumbnail_path (path, not data)
       Extra     json.RawMessage // Schema: metadata
   }
   ```

5. **Access Control Bypass:**
   ```go
   func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
       // ...
       // TODO: Check if file is referenced in a message in a conversation the user is in
       // For now, allow access (files are typically shared in messages)
       return true, nil  // SECURITY BUG: Always allows access!
   }
   ```

---

## Proposed Design for mvChat2

### Goals

1. **Fix schema/code mismatches** - Align database schema with Go code
2. **Implement proper deduplication** - SHA-256 hash-based, like FilesAPI/mvChat
3. **Use streaming hash calculation** - Single-pass efficiency from mvChat
4. **Support reference counting** - Safe deletion when file no longer referenced
5. **Improve directory structure** - Date-based organization like mvChat
6. **Proper access control** - Check message attachments, not just uploader

### Database Schema Changes

```sql
-- files table (aligned with code)
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Uploader tracking
    uploader_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- File info
    mime_type VARCHAR(128) NOT NULL,
    size BIGINT NOT NULL,
    original_name VARCHAR(512),  -- Optional: preserve original filename

    -- Storage
    location VARCHAR(512) NOT NULL,
    hash VARCHAR(64) NOT NULL,    -- SHA-256 base64 encoded

    -- Status as string for clarity
    status VARCHAR(16) NOT NULL DEFAULT 'pending',  -- 'pending', 'ready', 'failed'

    -- Soft delete
    deleted_at TIMESTAMPTZ
);

-- Index for deduplication lookup (critical!)
CREATE UNIQUE INDEX idx_files_hash ON files(hash) WHERE deleted_at IS NULL;
CREATE INDEX idx_files_uploader ON files(uploader_id);
CREATE INDEX idx_files_status ON files(status) WHERE status != 'ready';

-- file_metadata table (aligned with code)
CREATE TABLE file_metadata (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,

    -- Dimensions
    width INT,
    height INT,
    duration FLOAT,  -- seconds, for video/audio

    -- Thumbnail (stored as base64 in DB for simplicity)
    thumbnail TEXT,  -- base64 encoded JPEG

    -- Extended metadata
    extra JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Message attachments tracking (for access control + reference counting)
CREATE TABLE message_attachments (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, file_id)
);

CREATE INDEX idx_attachments_file ON message_attachments(file_id);
```

### Go Code Changes

#### 1. Add Hash Calculation (port from mvChat)

```go
// media/hash.go
package media

import (
    "crypto/sha256"
    "encoding/base64"
    "io"
)

// CalculateStreamHash computes SHA-256 hash while writing to a file.
// This is efficient as we hash during the initial write (single pass).
// Returns: hash (base64), bytes written, error
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

#### 2. Add Database Functions

```go
// store/files.go additions

// GetFileByHash retrieves an existing file by content hash.
// Returns nil, nil if no file with that hash exists.
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
    if err != nil {
        return nil, err
    }
    return &f, nil
}

// CreateFileWithHash creates a new file record with hash for deduplication.
func (db *DB) CreateFileWithHash(ctx context.Context, uploaderID uuid.UUID,
    mimeType string, size int64, location, hash string) (*File, error) {

    id := uuid.New()
    now := time.Now().UTC()

    _, err := db.pool.Exec(ctx, `
        INSERT INTO files (id, created_at, updated_at, uploader_id, status,
                          mime_type, size, location, hash)
        VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8)
    `, id, now, now, uploaderID, mimeType, size, location, hash)

    if err != nil {
        return nil, err
    }

    return &File{
        ID:         id,
        CreatedAt:  now,
        UpdatedAt:  now,
        UploaderID: uploaderID,
        Status:     "pending",
        MimeType:   mimeType,
        Size:       size,
        Location:   location,
        Hash:       hash,
    }, nil
}

// GetFileReferenceCount returns how many message attachments reference a file.
func (db *DB) GetFileReferenceCount(ctx context.Context, fileID uuid.UUID) (int, error) {
    var count int
    err := db.pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM message_attachments WHERE file_id = $1
    `, fileID).Scan(&count)
    return count, err
}

// CanAccessFile checks if a user can access a file.
// Access is allowed if:
// 1. User is the uploader, OR
// 2. File is attached to a message in a conversation the user is a member of
func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
    var allowed bool
    err := db.pool.QueryRow(ctx, `
        SELECT EXISTS(
            -- User is uploader
            SELECT 1 FROM files WHERE id = $1 AND uploader_id = $2 AND deleted_at IS NULL
            UNION
            -- File is in a conversation user is member of
            SELECT 1 FROM message_attachments ma
            JOIN messages m ON ma.message_id = m.id
            JOIN members mem ON m.conversation_id = mem.conversation_id
            WHERE ma.file_id = $1 AND mem.user_id = $2 AND mem.deleted_at IS NULL
        )
    `, fileID, userID).Scan(&allowed)
    return allowed, err
}
```

#### 3. Update Upload Handler

```go
// handlers_files.go - Updated upload with deduplication

func (fh *FileHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
    // ... auth and form parsing (unchanged) ...

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

    // Check for existing file with same hash (DEDUPLICATION!)
    existingFile, err := fh.db.GetFileByHash(r.Context(), hash)
    if err != nil {
        os.Remove(tempPath)
        http.Error(w, "database error", http.StatusInternalServerError)
        return
    }

    var fileRecord *store.File

    if existingFile != nil {
        // DEDUP HIT: Reuse existing file
        os.Remove(tempPath) // Don't need the temp file

        // Create new file record pointing to same location
        fileRecord, err = fh.db.CreateFileWithHash(r.Context(), userID,
            existingFile.MimeType, existingFile.Size, existingFile.Location, hash)
        if err != nil {
            http.Error(w, "failed to create file record", http.StatusInternalServerError)
            return
        }

        // Copy metadata from existing file
        existingMeta, _ := fh.db.GetFileMetadata(r.Context(), existingFile.ID)
        if existingMeta != nil {
            fh.db.CreateFileMetadata(r.Context(), fileRecord.ID,
                existingMeta.Width, existingMeta.Height,
                existingMeta.Duration, existingMeta.Thumbnail, existingMeta.Extra)
        }

        // Mark as ready immediately (no processing needed)
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
            mimeType, size, path, hash)
        if err != nil {
            os.Remove(path)
            http.Error(w, "failed to create file record", http.StatusInternalServerError)
            return
        }

        // Process media async (thumbnail generation, etc.)
        go fh.processMedia(fileRecord.ID, path, mimeType)
    }

    // Return file info
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    fmt.Fprintf(w, `{"id":"%s","mime":"%s","size":%d,"dedup":%t}`,
        fileRecord.ID, fileRecord.MimeType, fileRecord.Size, existingFile != nil)
}
```

---

## Migration Strategy

### Phase 1: Schema Migration
1. Add migration to rename `user_id` -> `uploader_id` or update code
2. Change `status` from INT to VARCHAR
3. Make `hash` nullable temporarily for existing data
4. Add `message_attachments` table

### Phase 2: Code Updates
1. Add `media/hash.go` with streaming hash calculation
2. Update `store/files.go` with new functions
3. Update `handlers_files.go` with dedup flow
4. Add `SaveUploadFromPath` to media processor

### Phase 3: Data Backfill
1. Calculate hashes for existing files
2. Update records with hash values
3. Make `hash` NOT NULL again
4. Identify and remove duplicate physical files

### Phase 4: Testing
1. Unit tests for hash calculation
2. Integration tests for dedup flow
3. Load testing for concurrent uploads of same file

---

## Summary: Best Practices Combined

| Feature | FilesAPI | mvChat | mvChat2 (Proposed) |
|---------|----------|--------|-------------------|
| Hash calculation | After write | Streaming | Streaming |
| Dedup check | By hash lookup | By hash lookup | By hash lookup |
| Hash storage | Unique index | In metadata | Unique index |
| Physical storage | GUID-named | Date-based dirs | Date-based dirs |
| Reference counting | Query all files | None | message_attachments |
| Metadata on dedup | Update existing | Copy from existing | Copy from existing |
| Thumbnail storage | Separate files | Separate files | Base64 in DB |
| Access control | Not shown | Not shown | Via message attachments |

The proposed mvChat2 design combines:
- **Streaming hash** from mvChat (efficiency)
- **Unique hash index** from FilesAPI (database-enforced dedup)
- **Reference counting** via attachments table (safe deletion)
- **Proper access control** (security)
