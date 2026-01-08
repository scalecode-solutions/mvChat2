# Type Definition Issues

This document catalogs inconsistencies and problems with struct definitions and type usage.

## Critical Issues

### 1. Missing JSON Tags on Store Types

Several store types are missing JSON tags despite being used in API responses:

**File: `store/users.go:25-33`**
```go
type AuthRecord struct {
    ID        uuid.UUID   // No JSON tag
    UserID    uuid.UUID   // No JSON tag
    Scheme    string      // No JSON tag
    Secret    string      // No JSON tag - SENSITIVE!
    Uname     *string     // No JSON tag
    ExpiresAt *time.Time  // No JSON tag
    CreatedAt time.Time   // No JSON tag
}
```

**File: `store/contacts.go:11-18`**
```go
type Contact struct {
    UserID    uuid.UUID  // No JSON tag
    ContactID uuid.UUID  // No JSON tag
    Source    string     // No JSON tag
    Nickname  *string    // No JSON tag
    InviteID  *uuid.UUID // No JSON tag
    CreatedAt time.Time  // No JSON tag
}
```

**File: `store/invites.go:14-25`**
```go
type InviteCode struct {
    // All fields missing JSON tags
}
```

**File: `store/conversations.go:327-331`**
```go
type ReadReceipt struct {
    UserID  uuid.UUID  // No JSON tag
    ReadSeq int        // No JSON tag
    RecvSeq int        // No JSON tag
}
```

**Impact:**
- If serialized directly, fields will use Go field names (PascalCase)
- Sensitive fields like `Secret` could be accidentally exposed

**Recommendation:** Add explicit JSON tags with `omitempty` where appropriate:
```go
type AuthRecord struct {
    ID        uuid.UUID  `json:"id"`
    UserID    uuid.UUID  `json:"userId"`
    Scheme    string     `json:"scheme"`
    Secret    string     `json:"-"` // Never serialize
    Uname     *string    `json:"username,omitempty"`
    ExpiresAt *time.Time `json:"expiresAt,omitempty"`
    CreatedAt time.Time  `json:"createdAt"`
}
```

---

### 2. Inconsistent Time Type Usage

Mix of `time.Time` and `*time.Time` for similar purposes:

**Required fields (non-nullable):**
| Field | Type | File |
|-------|------|------|
| `CreatedAt` | `time.Time` | Multiple files |
| `UpdatedAt` | `time.Time` | Multiple files |
| `Ts` | `time.Time` | `types.go` |

**Optional fields (nullable):**
| Field | Type | File |
|-------|------|------|
| `LastSeen` | `*time.Time` | `store/users.go:20` |
| `LastMsgAt` | `*time.Time` | `store/conversations.go:22` |
| `UsedAt` | `*time.Time` | `store/invites.go` |

This pattern is correct - `*time.Time` for nullable database columns, `time.Time` for non-null.

**Issue:** Some places that should be nullable aren't:
- `store/conversations.go:30-31` - `CreatedAt`, `UpdatedAt` in Member are non-pointer but could be null in LEFT JOINs

---

### 3. Embedded Struct with Field Duplication

**File: `store/conversations.go:44-59`**

```go
type ConversationWithMember struct {
    Conversation
    // Member data (embedded without json tags to avoid conflicts)
    MemberCreatedAt time.Time       `json:"-"`
    MemberUpdatedAt time.Time       `json:"-"`
    Role            string          `json:"role"`      // DUPLICATE from Member
    ReadSeq         int             `json:"readSeq"`   // DUPLICATE from Member
    RecvSeq         int             `json:"recvSeq"`   // DUPLICATE from Member
    ClearSeq        int             `json:"clearSeq"`  // DUPLICATE from Member
    Favorite        bool            `json:"favorite"`  // DUPLICATE from Member
    Muted           bool            `json:"muted"`     // DUPLICATE from Member
    Blocked         bool            `json:"blocked"`   // DUPLICATE from Member
    Private         json.RawMessage `json:"private,omitempty"` // DUPLICATE from Member
}
```

**Issues:**
1. Duplicates all `Member` fields manually
2. Maintenance burden - changes to `Member` require changes here
3. Could embed `Member` instead

**Recommendation:** Either:
- Embed `Member` directly: `Conversation; Member`
- Or use composition: add `Member Member` field

---

## High Priority Issues

### 4. Missing Validation Tags

No structs use validation tags despite accepting user input:

**Examples where validation would help:**

```go
// types.go - Input validation needed
type MsgClientLogin struct {
    Scheme string `json:"scheme"` // Should validate: oneof=basic token
    Secret string `json:"secret"` // Should validate: required
}

type MsgClientSend struct {
    ConversationID string `json:"conv"` // Should validate: required,uuid
    Content        any    `json:"content"` // Should validate: required
}

type MsgClientReact struct {
    Emoji string `json:"emoji"` // Should validate: required,max=10
}
```

**Recommendation:** Add validation using `go-playground/validator`:
```go
type MsgClientLogin struct {
    Scheme string `json:"scheme" validate:"required,oneof=basic token"`
    Secret string `json:"secret" validate:"required"`
}
```

---

### 5. Protocol Message Validation Gaps

**File: `types.go:10-31`**

```go
type ClientMessage struct {
    ID      string           `json:"id,omitempty"`  // Optional - clients can't correlate responses
    Hi      *MsgClientHi     `json:"hi,omitempty"`  // All fields optional
    Login   *MsgClientLogin  `json:"login,omitempty"`
    Acc     *MsgClientAcc    `json:"acc,omitempty"`
    // ... all omitempty
}
```

**Issues:**
1. `ID` is optional but needed for response correlation
2. Multiple message types can be set simultaneously (only first processed)
3. No way to enforce exactly one message type per request

---

## Medium Priority Issues

### 6. Inconsistent JSON Tag Naming

Mixed styles across different structs:

**Abbreviated style:**
```go
// types.go
Version   string `json:"ver"`
UserAgent string `json:"ua"`
DeviceID  string `json:"dev"`
```

**Full camelCase style:**
```go
// store/files.go
UploaderID uuid.UUID `json:"uploaderId"`
```

**Full lowercase:**
```go
// types.go
Scheme string `json:"scheme"`
```

**Recommendation:** Standardize on camelCase for all JSON fields.

---

### 7. Pointer vs Value Receivers Consistency

All types correctly use pointer receivers. This is good.

```go
// All correct examples:
func (db *DB) GetUserByID(...)
func (e *Encryptor) Encrypt(...)
func (s *Session) Send(...)
func (h *Hub) SendToUsers(...)
```

No issues found here.

---

### 8. Undocumented Field Semantics

Some fields have unclear purposes:

| Field | Type | File | Issue |
|-------|------|------|-------|
| `DelID` | `int` | `store/conversations.go:23` | What does this represent? |
| `RecvSeq` | `int` | `store/conversations.go:35` | Difference from `ReadSeq`? |
| `ClearSeq` | `int` | `store/conversations.go:36` | Purpose unclear |

**Recommendation:** Add field documentation:
```go
type Member struct {
    // ReadSeq is the sequence number of the last message the user has read
    ReadSeq int `json:"readSeq"`
    // RecvSeq is the sequence number of the last message delivered to the user
    RecvSeq int `json:"recvSeq"`
    // ClearSeq is the sequence number below which messages are hidden for this user
    ClearSeq int `json:"clearSeq"`
}
```

---

### 9. Type Definitions Without Documentation

Many exported types lack documentation comments:

**Missing documentation:**
- `types.go` - All message types
- `handlers.go:15-22` - `Handlers` struct
- `hub.go:13-33` - `Hub` struct
- `session.go:27-46` - `Session` struct
- `presence.go:12-27` - `PresenceManager` struct

**Example fix:**
```go
// Hub manages WebSocket sessions and message routing between clients.
// It maintains a registry of active sessions and handles broadcasting
// messages to users across potentially multiple nodes via Redis pub/sub.
type Hub struct {
    // ...
}
```

---

## Low Priority Issues

### 10. Unused Type Fields

Some fields appear to be defined but unused:

| Field | Type | File | Notes |
|-------|------|------|-------|
| `hash` | `VARCHAR(64)` | `schema.sql` | Defined in schema but never populated |
| `original_name` | `VARCHAR(512)` | `schema.sql` | Defined but not used in file operations |

---

### 11. Magic String Status Values

Status fields use string literals instead of constants:

```go
// store/invites.go - Various locations
status = 'pending'
status = 'used'
status = 'expired'
status = 'revoked'

// store/files.go
status = 'uploading'
status = 'ready'
status = 'failed'
```

**Recommendation:** Define constants:
```go
const (
    InviteStatusPending = "pending"
    InviteStatusUsed    = "used"
    InviteStatusExpired = "expired"
    InviteStatusRevoked = "revoked"
)

const (
    FileStatusUploading = "uploading"
    FileStatusReady     = "ready"
    FileStatusFailed    = "failed"
)
```

---

## Summary

| Issue | Severity | Count | Files Affected |
|-------|----------|-------|----------------|
| Missing JSON tags | Critical | 4 structs | users.go, contacts.go, invites.go, conversations.go |
| Embedded struct duplication | High | 1 struct | conversations.go |
| Missing validation tags | High | All input types | types.go |
| Time type inconsistency | Medium | Minor | Various |
| JSON naming inconsistency | Medium | 10+ fields | types.go, store/*.go |
| Missing documentation | Medium | 10+ types | Various |
| Magic string values | Low | 2 enums | invites.go, files.go |

## Recommended Actions

### Immediate
1. Add `json:"-"` to sensitive fields (e.g., `Secret` in `AuthRecord`)
2. Add JSON tags to all store types that may be serialized
3. Document `DelID`, `RecvSeq`, `ClearSeq` field meanings

### High Priority
1. Add validation tags to all input message types
2. Refactor `ConversationWithMember` to use composition
3. Define constants for status values

### Medium Priority
1. Standardize JSON tag naming convention
2. Add documentation comments to all exported types
3. Review and document nullable vs non-nullable time fields
