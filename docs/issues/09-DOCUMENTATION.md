# Documentation and Comment Issues

This document catalogs missing documentation, outdated comments, and areas needing explanation.

## High Priority Issues

### 1. Missing Documentation on Exported Types

Many exported types lack godoc comments:

**Root Package:**
| Type | File | Line |
|------|------|------|
| `Handlers` | `handlers.go` | 15-22 |
| `Hub` | `hub.go` | 13-33 |
| `Session` | `session.go` | 27-46 |
| `Server` | `server.go` | 20-25 |
| `PresenceManager` | `presence.go` | 12-27 |
| `FileHandlers` | `handlers_files.go` | 16-21 |
| `AuthValidator` | `handlers_files.go` | 23-26 |

**Auth Package:**
| Type/Function | File | Line |
|---------------|------|------|
| `Auth` | `auth/auth.go` | 39-42 |
| `Claims` | `auth/auth.go` | 58-62 |

**Config Package:**
| Item | File | Line |
|------|------|------|
| `DSN()` | `config/config.go` | 50-56 |

**Store Package:**
| Type | File | Line |
|------|------|------|
| `Conversation` | `conversations.go` | 13 |
| `Member` | `conversations.go` | 26-41 |
| `Message` | `messages.go` | 13-24 |
| `MessageDeletion` | `messages.go` | 26-31 |
| `File` | `files.go` | 14-24 |
| `FileMetadata` | `files.go` | 26-34 |
| `InviteCode` | `invites.go` | 13-25 |

**Recommendation:** Add godoc comments to all exported types:
```go
// Hub manages WebSocket session routing and cross-node messaging.
// It maintains a registry of connected sessions and coordinates
// presence updates across multiple server instances via Redis pub/sub.
type Hub struct {
    // ...
}
```

---

### 2. TODO Comments Requiring Attention

**Critical TODOs:**

| File | Line | TODO | Impact |
|------|------|------|--------|
| `store/files.go` | 142 | "Check if file is in a message in a conversation the user is a member of" | Security: File access control bypassed |
| `handlers_conv.go` | 152 | "join, leave, invite, kick, update" | Feature incomplete: Room operations not implemented |

**Stale TODO:**
| File | Line | TODO | Status |
|------|------|------|--------|
| `server.go` | 40 | "Add file upload/download routes" | Routes exist in `main.go:170`, TODO is outdated |

**Recommendation:** Either implement TODOs or document as known limitations.

---

### 3. Missing Protocol/API Documentation

**File:** `types.go`

All WebSocket message types lack documentation:

```go
// Example: Currently undocumented
type MsgClientHi struct {
    Version   string `json:"ver"`
    UserAgent string `json:"ua,omitempty"`
    DeviceID  string `json:"dev,omitempty"`
    Lang      string `json:"lang,omitempty"`
}
```

**Should be:**
```go
// MsgClientHi is sent by the client to establish a session.
// It must be the first message sent after WebSocket connection.
//
// Fields:
//   - Version: Client protocol version (e.g., "0.1.0")
//   - UserAgent: Client application identifier
//   - DeviceID: Unique device identifier for multi-device support
//   - Lang: Preferred language for server messages (e.g., "en-US")
type MsgClientHi struct {
    // ...
}
```

---

### 4. Complex Code Without Explanation

**File:** `auth/auth.go:84-118` - `VerifyPassword()`
```go
// Complex manual hash parsing with Sscanf workaround
// Why? What's the expected format? What's the fallback?
```

**File:** `store/messages.go:173-239` - `AddReaction()`
```go
// Complex nested JSON manipulation for reaction toggling
// Data structure not documented
// Toggle behavior not explained
```

**File:** `store/conversations.go:294-315` - `UpdateMemberSettings()`
```go
// Dynamic SQL building with string concatenation
// Comment says "simplified version" but no explanation of risks
```

**File:** `irido/irido.go:159-211` - `parseMap()`
```go
// Recursive parsing with type assertions
// No explanation of why this approach vs reflection
```

---

## Medium Priority Issues

### 5. Outdated Comments

| File | Line | Comment | Issue |
|------|------|---------|-------|
| `auth/auth.go` | 96 | "Handle the case where Sscanf doesn't split on $" | Doesn't explain why Sscanf is unreliable |
| `store/conversations.go` | 311 | "This is a simplified version - in production use a query builder" | Code is production code |
| `redis/redis.go` | 78 | "Store node ID with 2 minute TTL" | Doesn't explain timing rationale |
| `email/email.go` | 76 | "Use TLS for port 465, STARTTLS for 587" | Code only handles 465, not 587 |

---

### 6. Inconsistent Comment Styles

**Good style (with section headers):**
```go
// redis/redis.go - Lines 71-113
// ============================================================================
// Presence Cache
// ============================================================================
```

**Inconsistent style:**
```go
// auth/auth.go - Mix of inline and block comments
hash := argon2.IDKey(...)  // Parameters: time=1, memory=64MB...
```

**Recommendation:** Standardize on block comments above code for explanations.

---

### 7. Missing Transaction Documentation

**File:** `store/conversations.go:63-133`

`CreateDM()` and `CreateRoom()` use transactions but don't document this:
```go
func (db *DB) CreateDM(ctx context.Context, ...) (*Conversation, error) {
    tx, err := db.pool.Begin(ctx)  // Transaction not documented
    // ...
}
```

**Recommendation:** Document atomicity guarantees:
```go
// CreateDM creates a new direct message conversation between two users.
// The operation is atomic - either all database records are created or none.
// Returns the created conversation with the caller's membership data.
```

---

### 8. Missing nil/Error Semantics Documentation

Multiple store functions return `nil, nil` on "not found":

```go
// store/users.go:59-60
if errors.Is(err, pgx.ErrNoRows) {
    return nil, nil
}

// store/conversations.go:187-188
if errors.Is(err, pgx.ErrNoRows) {
    return nil, nil
}
```

**Issue:** Callers must check both error AND nil value. Not documented.

**Recommendation:** Document in package:
```go
// Package store provides database access for mvChat2.
//
// Error handling: Functions that retrieve a single record return (nil, nil)
// when the record is not found. Callers should check the result for nil
// before use. Other database errors are returned as (nil, err).
package store
```

---

### 9. Missing Handler Documentation

Handler functions lack documentation of:
- What the handler does
- Required authentication state
- Side effects
- Response format

**Example from `handlers_conv.go`:**
```go
// Currently undocumented
func (h *Handlers) HandleDM(s *Session, msg *ClientMessage) {
```

**Should be:**
```go
// HandleDM processes direct message operations.
// Requires authentication.
//
// Actions:
//   - "get": Returns existing DM with user or nil
//   - "start": Creates new DM conversation or returns existing
//   - "manage": Updates DM settings (favorite, muted, blocked)
//
// Request: MsgClientDM with action and user/conv fields
// Response: CtrlSuccess with conversation data or CtrlError
func (h *Handlers) HandleDM(s *Session, msg *ClientMessage) {
```

---

## Low Priority Issues

### 10. Missing Package Documentation

**Root package (main):** No package comment

**auth package:** Has package comment (good)

**irido package:** Has excellent package comment (good example):
```go
// Package irido implements the Irido message format.
// Irido (named after the Japanese word for "entry", 入り戸) is a
// structured format for rich messages...
```

---

### 11. Validation Rules Not Documented

**File:** `irido/irido.go:506-532`

Validation rules are inline comments but should be documented:
```go
// Must have text or media
// Max 10 media attachments
// mentions require text
```

**Recommendation:** Document constraints in godoc:
```go
// Validate checks if an Irido message is valid.
// Constraints:
//   - Message must have text or at least one media attachment
//   - Maximum 10 media attachments per message
//   - Mentions require accompanying text
//   - Media items must have non-empty src fields
// Returns an error describing the first validation failure.
```

---

### 12. Context Usage Not Documented

Store methods take `context.Context` but don't document:
- Timeout behavior
- Cancellation behavior
- Whether context is propagated to database

---

## Summary

| Category | Count | Priority |
|----------|-------|----------|
| Missing type documentation | 20+ | High |
| Critical TODOs | 2 | High |
| Missing API/protocol docs | 30+ | High |
| Complex code unexplained | 4 | High |
| Outdated comments | 4 | Medium |
| Inconsistent style | Multiple | Medium |
| Missing transaction docs | 2 | Medium |
| Missing nil semantics | Package-wide | Medium |
| Missing handler docs | 10+ | Medium |
| Missing package docs | 1 | Low |
| Validation not documented | 1 | Low |

## Recommended Documentation Structure

### 1. Package-Level Documentation

Each package should have a doc.go or header comment explaining:
- Package purpose
- Key types and their relationships
- Error handling conventions
- Thread safety guarantees

### 2. Type Documentation

All exported types should document:
- Purpose of the type
- When to use it
- Thread safety (if applicable)
- Field meanings for structs

### 3. Function Documentation

All exported functions should document:
- What the function does
- Parameters and their constraints
- Return values and error conditions
- Side effects
- Concurrency guarantees

### 4. Protocol Documentation

Create `docs/PROTOCOL.md` documenting:
- Message types and their purposes
- Request/response flow
- Authentication requirements
- Error codes and meanings
- Example message sequences

### 5. API Documentation

Create `docs/API.md` documenting:
- REST endpoints (file upload/download)
- WebSocket endpoint
- Health check endpoint
- Required headers and authentication
