-- mvChat2 Database Schema
-- PostgreSQL 15+

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- USERS
-- ============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Account state: 'ok', 'suspended', 'deleted'
    state VARCHAR(16) NOT NULL DEFAULT 'ok',
    state_at TIMESTAMPTZ,
    
    -- Public profile data (display name, avatar URL, etc.)
    public JSONB,
    
    -- Last activity tracking
    last_seen TIMESTAMPTZ,
    user_agent VARCHAR(255),

    -- Temporary password flag (user must change password on next login)
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE,

    -- User's email address (for password reset, verification, etc.)
    email VARCHAR(255)
);

CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;

CREATE INDEX idx_users_state ON users(state) WHERE state != 'deleted';

-- ============================================================================
-- AUTHENTICATION
-- ============================================================================

CREATE TABLE auth (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 'basic' for login/password, 'token' for session tokens
    scheme VARCHAR(16) NOT NULL,
    
    -- For basic: hashed password. For token: token hash
    secret VARCHAR(255) NOT NULL,
    
    -- For basic: login name (unique). For token: not used
    uname VARCHAR(64),
    
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(scheme, uname)
);

CREATE INDEX idx_auth_user ON auth(user_id);
CREATE INDEX idx_auth_lookup ON auth(scheme, uname) WHERE uname IS NOT NULL;

-- ============================================================================
-- CONVERSATIONS (DMs and Rooms)
-- ============================================================================

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- 'dm' or 'room'
    type VARCHAR(8) NOT NULL,
    
    -- For rooms: owner user ID
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- For rooms: public info (name, avatar, description)
    public JSONB,
    
    -- Message sequence tracking
    last_seq INT NOT NULL DEFAULT 0,
    last_msg_at TIMESTAMPTZ,
    
    -- Deletion sequence tracking
    del_id INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_conversations_type ON conversations(type);
CREATE INDEX idx_conversations_last_msg ON conversations(last_msg_at DESC);

-- For DMs: track the two participants for fast lookup
-- This allows finding existing DM between two users
CREATE TABLE dm_participants (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    user1_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user2_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Ensure user1_id < user2_id for consistent ordering
    CONSTRAINT dm_user_order CHECK (user1_id < user2_id),
    UNIQUE(user1_id, user2_id)
);

CREATE INDEX idx_dm_user1 ON dm_participants(user1_id);
CREATE INDEX idx_dm_user2 ON dm_participants(user2_id);

-- ============================================================================
-- MEMBERS (Conversation membership)
-- ============================================================================

CREATE TABLE members (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- For rooms: 'owner', 'admin', 'member'
    role VARCHAR(16) NOT NULL DEFAULT 'member',
    
    -- Read/receive tracking
    read_seq INT NOT NULL DEFAULT 0,
    recv_seq INT NOT NULL DEFAULT 0,
    
    -- Bulk delete: messages with seq <= clear_seq are hidden for this user
    clear_seq INT NOT NULL DEFAULT 0,
    
    -- User preferences for this conversation
    favorite BOOLEAN NOT NULL DEFAULT FALSE,
    muted BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- For DMs: block the other user
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- User's private data for this conversation (nickname, notes, etc.)
    private JSONB,
    
    -- Soft delete: user left/was removed but record kept for history
    deleted_at TIMESTAMPTZ,
    
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX idx_members_user ON members(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_members_favorite ON members(user_id, favorite) WHERE favorite = TRUE AND deleted_at IS NULL;
-- Composite index for conversation member lookups
CREATE INDEX idx_members_conv_user ON members(conversation_id, user_id) WHERE deleted_at IS NULL;

-- ============================================================================
-- MESSAGES
-- ============================================================================

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    
    -- Per-conversation sequence number (for ordering, pagination)
    seq INT NOT NULL,
    
    from_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Message content in Irido format (encrypted at rest)
    content BYTEA,
    
    -- Message metadata: edit_count, reactions, reply_to, etc.
    head JSONB,
    
    -- Hard delete (delete for everyone): set by sender
    deleted_at TIMESTAMPTZ,
    
    UNIQUE(conversation_id, seq)
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id, seq DESC);
CREATE INDEX idx_messages_from ON messages(from_user_id);
CREATE INDEX idx_messages_created ON messages(created_at DESC);

-- ============================================================================
-- MESSAGE DELETIONS (Soft delete for individual messages)
-- ============================================================================

CREATE TABLE message_deletions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (message_id, user_id)
);

-- Index for finding all deleted messages for a user
CREATE INDEX idx_message_deletions_user ON message_deletions(user_id);

-- ============================================================================
-- FILES
-- ============================================================================

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    uploader_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- File info
    mime_type VARCHAR(128) NOT NULL,
    size BIGINT NOT NULL,
    original_name VARCHAR(512),

    -- Storage location (path on disk)
    location VARCHAR(512) NOT NULL,

    -- SHA-256 hash for deduplication (nullable until hash feature implemented)
    hash VARCHAR(64),

    -- Status: 'pending', 'ready', 'failed'
    status VARCHAR(16) NOT NULL DEFAULT 'pending',

    -- Soft delete
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_files_uploader ON files(uploader_id);
CREATE INDEX idx_files_hash ON files(hash) WHERE hash IS NOT NULL;
CREATE INDEX idx_files_status ON files(status) WHERE status = 'pending';

CREATE TABLE file_metadata (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,

    -- Media dimensions
    width INT,
    height INT,

    -- Duration in seconds for audio/video (float for precision)
    duration FLOAT,

    -- Thumbnail data (base64 encoded JPEG stored in DB for simplicity)
    thumbnail TEXT,

    -- Extended metadata (EXIF, codec info, etc.)
    extra JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- SCHEMA VERSION
-- ============================================================================

CREATE TABLE schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schema_version (version) VALUES (6);

-- ============================================================================
-- INVITE CODES
-- ============================================================================

CREATE TABLE invite_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Who created the invite
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Short 10-character alphanumeric code for user sharing
    code VARCHAR(10) NOT NULL UNIQUE,

    -- Full cryptographic token (for verification)
    token TEXT NOT NULL,

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

CREATE INDEX idx_invite_codes_inviter ON invite_codes(inviter_id);
CREATE INDEX idx_invite_codes_code ON invite_codes(code) WHERE status = 'pending';
CREATE INDEX idx_invite_codes_email ON invite_codes(email);

-- ============================================================================
-- CONTACTS (User relationships from invites)
-- ============================================================================

CREATE TABLE contacts (
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

CREATE INDEX idx_contacts_user ON contacts(user_id);
CREATE INDEX idx_contacts_contact ON contacts(contact_id);
