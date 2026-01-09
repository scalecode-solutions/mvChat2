-- Migration 010: Quick wins - lang preference, no-screenshots, mentions
-- Add user language preference
ALTER TABLE users ADD COLUMN IF NOT EXISTS lang VARCHAR(10);

-- Add no-screenshots preference to conversations (stored in members for per-user setting in DMs)
-- For rooms, this is set by owner/admin and stored in conversation
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS no_screenshots BOOLEAN NOT NULL DEFAULT false;

-- Add index for mentions lookup (messages that mention a specific user)
-- Mentions are stored in message head as {"mentions": [{"userId": "...", ...}]}
-- We use a GIN index on the head JSONB for efficient querying
CREATE INDEX IF NOT EXISTS idx_messages_head_mentions ON messages USING GIN ((head->'mentions'));
