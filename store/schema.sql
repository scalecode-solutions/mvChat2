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
    user_agent VARCHAR(255)
);

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
-- CONVERSATIONS (DMs and Groups)
-- ============================================================================

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- 'dm' or 'group'
    type VARCHAR(8) NOT NULL,
    
    -- For groups: owner user ID
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- For groups: public info (name, avatar, description)
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
    
    -- For groups: 'owner', 'admin', 'member'
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

-- ============================================================================
-- MESSAGE DELETIONS (Soft delete for individual messages)
-- ============================================================================

CREATE TABLE message_deletions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (message_id, user_id)
);

-- ============================================================================
-- FILES
-- ============================================================================

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- File info
    mime_type VARCHAR(128) NOT NULL,
    size BIGINT NOT NULL,
    original_name VARCHAR(512),
    
    -- Storage location (path on disk)
    location VARCHAR(512) NOT NULL,
    
    -- SHA-256 hash for deduplication
    hash VARCHAR(64) NOT NULL,
    
    -- Status: 0 = active, 1 = pending deletion
    status INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_files_user ON files(user_id);
CREATE INDEX idx_files_hash ON files(hash);

CREATE TABLE file_metadata (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    
    -- Media dimensions
    width INT,
    height INT,
    duration_seconds INT,
    
    -- Thumbnail/preview paths
    has_thumbnail BOOLEAN NOT NULL DEFAULT FALSE,
    thumbnail_path VARCHAR(512),
    
    -- Extended metadata (EXIF, etc.)
    metadata JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- SCHEMA VERSION
-- ============================================================================

CREATE TABLE schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schema_version (version) VALUES (1);
