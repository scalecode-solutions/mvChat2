-- Migration 002: Add invite codes table
-- Run this after the initial schema

CREATE TABLE IF NOT EXISTS invite_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Who created the invite
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 10-digit numeric code
    code VARCHAR(10) NOT NULL UNIQUE,
    
    -- Recipient email (for sending the invite)
    email VARCHAR(255) NOT NULL,
    
    -- Optional: display name for the invitee
    invitee_name VARCHAR(128),
    
    -- Status: 'pending', 'used', 'expired', 'revoked'
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    
    -- When the code was used (if used)
    used_at TIMESTAMPTZ,
    used_by UUID REFERENCES users(id) ON DELETE SET NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days')
);

CREATE INDEX IF NOT EXISTS idx_invite_codes_inviter ON invite_codes(inviter_id);
CREATE INDEX IF NOT EXISTS idx_invite_codes_code ON invite_codes(code) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_invite_codes_email ON invite_codes(email);

-- Update schema version
UPDATE schema_version SET version = 2, applied_at = NOW() WHERE version = 1;
INSERT INTO schema_version (version) SELECT 2 WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 2);
