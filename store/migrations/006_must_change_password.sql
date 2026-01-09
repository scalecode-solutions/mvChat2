-- Migration 006: Add must_change_password flag to users table
-- From version 5 to version 6
--
-- This flag indicates that the user signed up with a temporary password
-- (e.g., an invite code) and must change it before continuing to use the app.

-- Check current version
DO $$
BEGIN
    IF (SELECT version FROM schema_version ORDER BY version DESC LIMIT 1) != 5 THEN
        RAISE EXCEPTION 'Migration requires schema version 5, found different version';
    END IF;
END $$;

-- Add must_change_password column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

-- Update schema version
INSERT INTO schema_version (version) VALUES (6);
