-- Migration 005: Add missing database indexes
-- From version 4 to version 5
--
-- These indexes improve query performance for common operations:
-- 1. Conversation member lookups
-- 2. Finding all deleted messages for a user
-- 3. Ordering messages by time

-- Check current version
DO $$
BEGIN
    IF (SELECT version FROM schema_version ORDER BY version DESC LIMIT 1) != 4 THEN
        RAISE EXCEPTION 'Migration requires schema version 4, found different version';
    END IF;
END $$;

-- Add composite index for conversation member lookups (WHERE user IN conversation)
CREATE INDEX IF NOT EXISTS idx_members_conv_user ON members(conversation_id, user_id)
    WHERE deleted_at IS NULL;

-- Add index for finding all deleted messages for a user
CREATE INDEX IF NOT EXISTS idx_message_deletions_user ON message_deletions(user_id);

-- Add index for ordering messages by creation time
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at DESC);

-- Update schema version
INSERT INTO schema_version (version) VALUES (5);
