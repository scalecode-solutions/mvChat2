-- Migration 003: Add contacts table
-- Run this after invite_codes migration

CREATE TABLE IF NOT EXISTS contacts (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- How the contact was established
    source VARCHAR(16) NOT NULL DEFAULT 'invite', -- 'invite', 'manual'
    
    -- Optional nickname for this contact
    nickname VARCHAR(64),
    
    -- Reference to the invite that created this contact (if source='invite')
    invite_id UUID REFERENCES invite_codes(id) ON DELETE SET NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (user_id, contact_id)
);

CREATE INDEX IF NOT EXISTS idx_contacts_user ON contacts(user_id);
CREATE INDEX IF NOT EXISTS idx_contacts_contact ON contacts(contact_id);

-- Update schema version
UPDATE schema_version SET version = 3, applied_at = NOW() WHERE version = 2;
INSERT INTO schema_version (version) SELECT 3 WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 3);
