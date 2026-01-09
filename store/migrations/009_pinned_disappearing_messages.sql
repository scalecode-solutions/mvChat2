-- Migration 009: Pinned messages and disappearing messages
--
-- Pinned messages: One pinned message per conversation
-- Disappearing messages: Per-conversation TTL setting + view-once per message

-- Conversation-level settings
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS disappearing_ttl INT;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS pinned_message_id UUID REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS pinned_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Message-level view-once support
ALTER TABLE messages ADD COLUMN IF NOT EXISTS view_once BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS view_once_ttl INT; -- seconds: 10, 30, 60, 300, 3600, 86400, 604800

-- Track message reads for expiration calculation
-- expires_at is calculated as read_at + TTL (from conversation or view_once_ttl)
CREATE TABLE IF NOT EXISTS message_reads (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ, -- NULL if no disappearing/view-once
    expired BOOLEAN NOT NULL DEFAULT FALSE, -- soft delete flag for this user
    PRIMARY KEY (message_id, user_id)
);

-- Index for efficient expiration queries
CREATE INDEX IF NOT EXISTS idx_message_reads_expires ON message_reads(expires_at) WHERE expires_at IS NOT NULL AND NOT expired;
CREATE INDEX IF NOT EXISTS idx_message_reads_user ON message_reads(user_id, expired);

-- Update schema version
UPDATE schema_version SET version = 9 WHERE version = 8;
INSERT INTO schema_version (version) SELECT 9 WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 9);
