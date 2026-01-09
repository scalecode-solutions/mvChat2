-- Migration 007: Add email column to users table
-- From version 6 to version 7
--
-- Stores email on user account to enable:
-- - Password reset flow
-- - Email verification
-- - User lookup by email

-- Check current version
DO $$
BEGIN
    IF (SELECT version FROM schema_version ORDER BY version DESC LIMIT 1) != 6 THEN
        RAISE EXCEPTION 'Migration requires schema version 6, found different version';
    END IF;
END $$;

-- Add email column to users table (nullable for existing users)
ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255);

-- Add unique index on email (partial - only for non-null values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;

-- Update schema version
INSERT INTO schema_version (version) VALUES (7);
