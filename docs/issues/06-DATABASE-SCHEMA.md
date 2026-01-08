# Database Schema and Query Issues

This document catalogs schema inconsistencies, missing indexes, and query performance issues.

## Critical Issues

### 1. Schema Mismatch: `uploader_id` vs `user_id`

**Files:** `store/schema.sql`, `store/files.go`

The `files` table schema defines `user_id` but the code uses `uploader_id`:

**Schema (schema.sql:189):**
```sql
user_id UUID NOT NULL REFERENCES users(id),
```

**Index (schema.sql:209):**
```sql
CREATE INDEX idx_files_user ON files(user_id);
```

**Code (files.go:48):**
```go
INSERT INTO files (id, created_at, updated_at, uploader_id, status, ...)
```

**Impact:** Runtime errors - "column 'uploader_id' does not exist"

**Recommendation:** Either:
- Update schema to use `uploader_id` (breaking change)
- Update code to use `user_id`

---

### 2. Missing `hash` Field Handling

**Files:** `store/schema.sql:203`, `store/files.go`

Schema defines a `hash` field with NOT NULL constraint:
```sql
hash VARCHAR(64) NOT NULL,
```

But `CreateFile()` in `files.go` never populates this field.

**Impact:** INSERT will fail due to NOT NULL constraint, or deduplication feature is non-functional.

**Recommendation:** Either:
- Remove NOT NULL constraint if hash is optional
- Implement file hashing and pass to CreateFile()

---

## High Severity Issues

### 3. N+1 Query Problem in GetUserConversations

**File:** `store/conversations.go:246-254`

```go
for i := range results {
    if results[i].Type == "dm" {
        otherUser, err := db.GetDMOtherUser(ctx, results[i].Conversation.ID, userID)
        // GetDMOtherUser() -> GetUserByID() = 2 queries per DM
    }
}
```

**Impact:** For a user with 20 DM conversations:
- Current: 1 + (2 × 20) = 41 queries
- Optimal: 1-2 queries

**Recommendation:** Use JOIN to fetch other user info in initial query:
```sql
SELECT c.*, m.*,
       CASE WHEN c.type = 'dm' THEN
           (SELECT u.* FROM users u
            JOIN dm_participants dp ON u.id = dp.user_id
            WHERE dp.conversation_id = c.id AND dp.user_id != $1)
       END as other_user
FROM conversations c
JOIN members m ON c.id = m.conversation_id
WHERE m.user_id = $1
```

---

### 4. Missing Index on `conversation_id` in members

**File:** `store/schema.sql:137`

Current:
```sql
CREATE INDEX idx_members_user ON members(user_id);
```

Missing:
```sql
CREATE INDEX idx_members_conversation ON members(conversation_id);
```

**Affected queries:**
| Function | File | Line |
|----------|------|------|
| `GetConversationMembers()` | `conversations.go` | 274-277 |
| `GetReadReceipts()` | `conversations.go` | 334-338 |
| `IsMember()` | `conversations.go` | 359 |

**Impact:** Full table scans for membership lookups.

---

### 5. Missing Index on `message_deletions`

**File:** `store/schema.sql:175-181`

No indexes exist on `message_deletions` table.

**Usage (messages.go:91):**
```sql
LEFT JOIN message_deletions md ON m.id = md.message_id AND md.user_id = $2
```

**Recommendation:**
```sql
CREATE INDEX idx_message_deletions_message ON message_deletions(message_id);
CREATE INDEX idx_message_deletions_user ON message_deletions(user_id);
-- Or composite:
CREATE INDEX idx_message_deletions_lookup ON message_deletions(message_id, user_id);
```

---

### 6. Missing Transaction in AddContact

**File:** `store/contacts.go:20-40`

Two separate INSERTs without transaction:

```go
// Line 25-30: Insert first direction
_, err := db.pool.Exec(ctx, `INSERT INTO contacts (user_id, contact_id, ...)`)

// Line 34-39: Insert second direction
_, err = db.pool.Exec(ctx, `INSERT INTO contacts ...`)
```

**Impact:** If second INSERT fails, contact relationship is one-directional.

**Recommendation:**
```go
tx, err := db.pool.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)

// First INSERT
_, err = tx.Exec(ctx, `INSERT INTO contacts ...`)
if err != nil {
    return err
}

// Second INSERT
_, err = tx.Exec(ctx, `INSERT INTO contacts ...`)
if err != nil {
    return err
}

return tx.Commit(ctx)
```

---

## Medium Severity Issues

### 7. Race Condition in Reaction Updates

**File:** `store/messages.go:172-239`

Non-atomic read-modify-write pattern:

```go
// 1. Read current head
err = db.pool.QueryRow(ctx, `SELECT COALESCE(head, '{}'::jsonb) FROM messages`)

// 2. Modify in application
json.Unmarshal(head, &headMap)
// ... modify reactions ...
newHead, _ := json.Marshal(headMap)

// 3. Write back
_, err = db.pool.Exec(ctx, `UPDATE messages SET head = $3`)
```

**Impact:** Concurrent reaction updates can overwrite each other.

**Recommendation:** Use PostgreSQL JSONB operators for atomic updates:
```sql
UPDATE messages
SET head = jsonb_set(
    COALESCE(head, '{}'::jsonb),
    '{reactions}',
    CASE
        WHEN head->'reactions'->$3 ? $4 THEN
            head->'reactions' - $3  -- Remove user from reaction
        ELSE
            jsonb_set(COALESCE(head->'reactions', '{}'::jsonb),
                      ARRAY[$3],
                      COALESCE(head->'reactions'->$3, '[]'::jsonb) || to_jsonb($4::text))
    END
)
WHERE conversation_id = $1 AND seq = $2
```

---

### 8. TOCTOU Race in Invite Code Generation

**File:** `store/invites.go:43-67`

```go
for i := 0; i < 5; i++ {
    code, err = GenerateInviteCode()

    // Check if exists
    var exists bool
    err = db.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invite_codes WHERE code = $1)`, code)

    // Gap: another goroutine could insert same code here

    // Insert
    err = db.pool.QueryRow(ctx, `INSERT INTO invite_codes... VALUES ($1, ...)`)
}
```

**Impact:** Under concurrent load, duplicate codes could be generated.

**Recommendation:** Use ON CONFLICT or rely solely on UNIQUE constraint:
```go
for i := 0; i < 5; i++ {
    code, _ = GenerateInviteCode()
    err = db.pool.QueryRow(ctx, `
        INSERT INTO invite_codes (code, ...) VALUES ($1, ...)
        ON CONFLICT (code) DO NOTHING
        RETURNING id
    `, code).Scan(&id)
    if err == nil {
        return code, nil  // Success
    }
    // Retry with new code
}
```

---

### 9. Dynamic SQL Column Names Without Validation

**File:** `store/conversations.go:294-315`

```go
func (db *DB) UpdateMemberSettings(ctx context.Context, convID, userID uuid.UUID, updates map[string]any) error {
    for key, val := range updates {
        setClauses += key + " = $" + string(rune('0'+i))  // No validation!
    }
    query := "UPDATE members SET " + setClauses + " WHERE ..."
}
```

**Impact:** If `updates` keys come from user input, SQL injection possible.

**Recommendation:** Use whitelist validation:
```go
var allowedColumns = map[string]bool{
    "role": true, "favorite": true, "muted": true,
    "blocked": true, "private": true,
}

for key := range updates {
    if !allowedColumns[key] {
        return fmt.Errorf("invalid column: %s", key)
    }
}
```

---

### 10. Missing `original_name` Field Usage

**File:** `store/schema.sql:197`, `store/files.go`

Schema defines:
```sql
original_name VARCHAR(512),
```

But `CreateFile()` never uses this field. Original filenames are lost.

**Recommendation:** Add `originalName` parameter to `CreateFile()`.

---

## Low Severity Issues

### 11. String Status Values Instead of Enum

**Files:** `store/invites.go`, `store/files.go`

Status values stored as strings:

```sql
-- invite_codes
status VARCHAR(16) NOT NULL DEFAULT 'pending'
-- Values: 'pending', 'used', 'expired', 'revoked'

-- files
status VARCHAR(16) NOT NULL DEFAULT 'uploading'
-- Values: 'uploading', 'ready', 'failed'
```

**Issues:**
- No compile-time type checking
- Typos possible in code
- No database-level validation

**Recommendation:** Use PostgreSQL ENUM:
```sql
CREATE TYPE invite_status AS ENUM ('pending', 'used', 'expired', 'revoked');
CREATE TYPE file_status AS ENUM ('uploading', 'ready', 'failed');
```

---

### 12. Missing Composite Index for Conversation Queries

**File:** `store/conversations.go:221-224`

Query filters by multiple fields:
```sql
WHERE m.user_id = $1
  AND c.deleted_at IS NULL
ORDER BY COALESCE(c.last_msg_at, c.created_at) DESC
```

**Recommendation:**
```sql
CREATE INDEX idx_conversations_active_recent
ON conversations(COALESCE(last_msg_at, created_at) DESC)
WHERE deleted_at IS NULL;
```

---

### 13. No Index for Time-Based Ordering

Many tables use `created_at` ordering but lack indexes:

| Table | Order By | Index Exists |
|-------|----------|--------------|
| `messages` | `seq` | Yes (primary key) |
| `files` | `created_at` | No |
| `invite_codes` | `created_at` | No |
| `contacts` | `created_at` | No |

---

## Recommendations Summary

| Issue | Severity | Action |
|-------|----------|--------|
| Schema mismatch (uploader_id) | Critical | Fix immediately |
| Missing hash field handling | Critical | Remove NOT NULL or implement |
| N+1 queries | High | Refactor with JOIN |
| Missing conversation_id index | High | Add index |
| Missing message_deletions index | High | Add index |
| Missing transaction in AddContact | High | Wrap in transaction |
| Reaction race condition | Medium | Use JSONB operators |
| Invite code race | Medium | Use ON CONFLICT |
| Dynamic SQL validation | Medium | Add whitelist |
| Missing original_name | Low | Add to CreateFile |
| String status values | Low | Consider ENUM |

## Suggested Migration

```sql
-- Migration: Fix schema issues

-- 1. Fix files table column name
ALTER TABLE files RENAME COLUMN user_id TO uploader_id;

-- 2. Make hash nullable (or add default)
ALTER TABLE files ALTER COLUMN hash DROP NOT NULL;

-- 3. Add missing indexes
CREATE INDEX IF NOT EXISTS idx_members_conversation ON members(conversation_id);
CREATE INDEX IF NOT EXISTS idx_message_deletions_lookup ON message_deletions(message_id, user_id);
CREATE INDEX IF NOT EXISTS idx_files_created ON files(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invites_created ON invite_codes(created_at DESC);

-- 4. Add composite index for active conversations
CREATE INDEX IF NOT EXISTS idx_conversations_active
ON conversations(COALESCE(last_msg_at, created_at) DESC)
WHERE deleted_at IS NULL;
```
