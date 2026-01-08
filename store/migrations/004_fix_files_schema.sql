-- Migration 004: Fix files and file_metadata schema to match Go code
-- From version 3 to version 4
--
-- Changes:
-- - files.user_id renamed to uploader_id
-- - files.status changed from INT to VARCHAR(16)
-- - files.hash made nullable
-- - files.deleted_at added
-- - file_metadata.duration_seconds changed to duration (FLOAT)
-- - file_metadata.thumbnail_path changed to thumbnail (TEXT for base64)
-- - file_metadata.has_thumbnail removed
-- - file_metadata.metadata renamed to extra

-- Check current version
DO $$
BEGIN
    IF (SELECT version FROM schema_version ORDER BY version DESC LIMIT 1) != 3 THEN
        RAISE EXCEPTION 'Migration requires schema version 3, found different version';
    END IF;
END $$;

-- ============================================================================
-- MIGRATE files TABLE
-- ============================================================================

-- Rename user_id to uploader_id
ALTER TABLE files RENAME COLUMN user_id TO uploader_id;

-- Drop old index and create new one
DROP INDEX IF EXISTS idx_files_user;
CREATE INDEX idx_files_uploader ON files(uploader_id);

-- Add deleted_at column
ALTER TABLE files ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Make hash nullable (it already might be, but ensure it)
ALTER TABLE files ALTER COLUMN hash DROP NOT NULL;

-- Update hash index to be partial (only index non-null values)
DROP INDEX IF EXISTS idx_files_hash;
CREATE INDEX idx_files_hash ON files(hash) WHERE hash IS NOT NULL;

-- Change status from INT to VARCHAR
-- First, add a temporary column
ALTER TABLE files ADD COLUMN status_new VARCHAR(16);

-- Convert existing values (0 = 'ready', 1 = 'pending deletion' -> 'failed')
UPDATE files SET status_new = CASE
    WHEN status = 0 THEN 'ready'
    WHEN status = 1 THEN 'failed'
    ELSE 'pending'
END;

-- Drop old column and rename new one
ALTER TABLE files DROP COLUMN status;
ALTER TABLE files RENAME COLUMN status_new TO status;
ALTER TABLE files ALTER COLUMN status SET NOT NULL;
ALTER TABLE files ALTER COLUMN status SET DEFAULT 'pending';

-- Add index for pending status
CREATE INDEX idx_files_status ON files(status) WHERE status = 'pending';

-- ============================================================================
-- MIGRATE file_metadata TABLE
-- ============================================================================

-- Rename duration_seconds to duration and change type
ALTER TABLE file_metadata RENAME COLUMN duration_seconds TO duration_old;
ALTER TABLE file_metadata ADD COLUMN duration FLOAT;
UPDATE file_metadata SET duration = duration_old::FLOAT WHERE duration_old IS NOT NULL;
ALTER TABLE file_metadata DROP COLUMN duration_old;

-- Change thumbnail_path to thumbnail (for base64 data)
ALTER TABLE file_metadata RENAME COLUMN thumbnail_path TO thumbnail;
ALTER TABLE file_metadata ALTER COLUMN thumbnail TYPE TEXT;

-- Remove has_thumbnail (no longer needed - just check if thumbnail is not null)
ALTER TABLE file_metadata DROP COLUMN IF EXISTS has_thumbnail;

-- Rename metadata to extra
ALTER TABLE file_metadata RENAME COLUMN metadata TO extra;

-- ============================================================================
-- UPDATE SCHEMA VERSION
-- ============================================================================

INSERT INTO schema_version (version) VALUES (4);
