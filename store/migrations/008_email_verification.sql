-- Migration 008: Add email verification fields to users table
-- From version 7 to version 8
--
-- Adds support for optional email verification.
-- NOTE: email_verified defaults to TRUE for user safety in DV contexts.
-- When verification is disabled in config, emails are auto-verified.

-- Check current version
DO $$
BEGIN
    IF (SELECT version FROM schema_version ORDER BY version DESC LIMIT 1) != 7 THEN
        RAISE EXCEPTION 'Migration requires schema version 7, found different version';
    END IF;
END $$;

-- Add email_verified column (defaults to TRUE for safety)
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT TRUE;

-- Add verification token fields
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_token VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_expires TIMESTAMPTZ;

-- Index for token lookup
CREATE INDEX IF NOT EXISTS idx_users_email_verification_token
ON users(email_verification_token)
WHERE email_verification_token IS NOT NULL;

-- Update schema version
INSERT INTO schema_version (version) VALUES (8);
