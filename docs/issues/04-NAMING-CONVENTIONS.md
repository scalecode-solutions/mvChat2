# Naming Convention Issues

This document catalogs inconsistencies in naming patterns throughout the codebase.

## High Priority Issues

### 1. Inconsistent Terminology: "Conversation" vs "Conv"

Mixed terminology for the same concept across the codebase:

| Location | Usage |
|----------|-------|
| Type names | `Conversation`, `ConversationID`, `ConversationWithMember` |
| JSON keys | `"conv"` (abbreviated) |
| Error messages | `"missing user or conv"` |
| Local variables | `conv`, `convID` |
| Function names | `GetConversationByID`, `GetConversationMembers` |

**Examples:**

```go
// types.go - JSON uses abbreviated form
ConversationID string `json:"conv,omitempty"`  // Line 92

// handlers_conv.go - Error message uses abbreviation
"missing user or conv"  // Line 38

// store/conversations.go - Variable uses abbreviation
var conv Conversation  // Line 78
```

**Recommendation:** Standardize on one term. Suggested:
- Types: `Conversation` (full name)
- JSON: `"conversationId"` (full camelCase)
- Variables: `conv` is acceptable for local scope, `conversation` for broader scope
- Errors: Use full name "missing conversation"

---

### 2. Field Abbreviations in Structs

Some struct fields use inconsistent abbreviations:

| Field | File | Line | Issue |
|-------|------|------|-------|
| `Uname` | `store/users.go` | 30 | Should be `Username` |
| `DelID` | `store/conversations.go` | 23 | Should be `DeleteID` or `DeletionID` |

```go
// store/users.go
type AuthRecord struct {
    Uname *string  // Line 30 - Should be Username
}

// store/conversations.go
type Conversation struct {
    DelID int `json:"delId"`  // Line 23 - Unclear what this represents
}
```

**Recommendation:** Use full descriptive names for all struct fields.

---

### 3. JSON Tag Naming Inconsistency

Mixed conventions in JSON serialization:

| Pattern | Example | Location |
|---------|---------|----------|
| Abbreviated | `"ver"`, `"ua"`, `"dev"` | `types.go:51-53` |
| camelCase | `"conversationId"`, `"fromUserId"` | Various |
| lowercase | `"scheme"`, `"secret"` | `types.go` |

```go
// types.go - Mixed abbreviations
type MsgClientHi struct {
    Version   string `json:"ver"`       // Abbreviated
    UserAgent string `json:"ua"`        // Abbreviated
    DeviceID  string `json:"dev"`       // Abbreviated
    Lang      string `json:"lang"`      // Full name
}
```

**Recommendation:** Standardize on camelCase for all JSON fields:
- `"version"` instead of `"ver"`
- `"userAgent"` instead of `"ua"`
- `"deviceId"` instead of `"dev"`

---

## Medium Priority Issues

### 4. Parameter Naming Inconsistency

Constructor and function parameters use inconsistent abbreviations:

```go
// handlers.go:25
func NewHandlers(db *store.DB, a *auth.Auth, hub *Hub, enc *crypto.Encryptor, emailSvc *email.Service)
```

| Parameter | Style |
|-----------|-------|
| `db` | Abbreviation |
| `a` | Single letter |
| `hub` | Full name |
| `enc` | Abbreviation |
| `emailSvc` | Abbreviated + suffix |

**Recommendation:** Use consistent naming:
```go
func NewHandlers(store *store.DB, auth *auth.Auth, hub *Hub, encryptor *crypto.Encryptor, email *email.Service)
```

---

### 5. Database Column vs JSON Tag Mismatch

Inconsistent translation between database columns and JSON serialization:

| Struct Field | JSON Tag | Issue |
|--------------|----------|-------|
| `FromUserID` | `"from"` | Over-abbreviated |
| `UploaderID` | `"uploaderId"` | Correct |
| `FileID` | `"fileId"` | Correct |

```go
// store/messages.go
type Message struct {
    FromUserID uuid.UUID `json:"from"`  // Line 18 - "from" is too abbreviated
}

// store/files.go
type File struct {
    UploaderID uuid.UUID `json:"uploaderId"`  // Line 18 - Correct style
}
```

**Recommendation:** Use consistent camelCase: `"fromUserId"` instead of `"from"`.

---

### 6. Receiver Variable Naming

Receivers are consistent per type but vary across types:

| Type | Receiver |
|------|----------|
| `*DB` | `db` |
| `*Encryptor` | `e` |
| `*Processor` | `p` |
| `*Irido` | `i` |
| `*Config` | `c` |
| `*Session` | `s` |
| `*Hub` | `h` |
| `*Handlers` | `h` |

This is acceptable in Go, but note that `*Hub` and `*Handlers` both use `h`.

---

### 7. Handler Naming Pattern

Public and private handlers follow good convention:

```go
// Public handlers - exported
func (h *Handlers) HandleLogin(...)   // Public entry point
func (h *Handlers) HandleDM(...)      // Public entry point

// Private handlers - unexported
func (h *Handlers) handleBasicLogin(...)  // Implementation detail
func (h *Handlers) handleTokenLogin(...)  // Implementation detail
func (h *Handlers) handleStartDM(...)     // Implementation detail
```

This pattern is correct and should be maintained.

---

## Low Priority Issues

### 8. Unexported Function Naming in irido Package

Multiple unexported parsing functions:

```go
// irido/irido.go
func parseMap(c map[string]any) *Irido      // Line 159
func parseMedia(m map[string]any) *Media    // Line 213
func parseEmbed(e map[string]any) *Embed    // Line 248
func parseReply(r map[string]any) *Reply    // Line 273
func parseMention(m map[string]any) *Mention // Line 289
func mediaDescription(m *Media) string       // Line 346
```

These are appropriately unexported internal helpers. No change needed.

---

### 9. Error Variable Naming

Errors follow Go conventions:

```go
// auth/auth.go
var (
    ErrInvalidToken  = errors.New("invalid token")
    ErrTokenExpired  = errors.New("token expired")
    ErrWeakPassword  = errors.New("password too weak")
)
```

This follows Go `Err` prefix convention correctly.

---

### 10. Context Variable Naming

Context is consistently named `ctx` throughout, which follows Go conventions.

---

## Summary Table

| Issue | Severity | Count | Impact |
|-------|----------|-------|--------|
| Conversation/Conv terminology | High | 8+ locations | Cognitive friction |
| Field abbreviations (Uname, DelID) | High | 2 | Unclear meaning |
| JSON tag inconsistency | High | 10+ fields | API inconsistency |
| Parameter naming | Medium | 5+ constructors | Readability |
| JSON vs struct mismatch | Medium | 3+ fields | Confusion |
| Receiver naming | Low | N/A | Acceptable |

## Recommendations

### Immediate Actions
1. **Rename `Uname` to `Username`** in `store/users.go`
2. **Rename `DelID` to `DeletionID`** or add documentation explaining what it represents
3. **Document JSON naming convention** - prefer full camelCase names

### Medium-Term Actions
1. Create a style guide documenting naming conventions
2. Consider renaming abbreviated JSON fields in next major version (breaking change)
3. Standardize parameter naming in constructors

### Style Guide Suggestions

```markdown
## Naming Conventions

### Struct Fields
- Use full descriptive names: `Username`, not `Uname`
- Use camelCase for multi-word: `FromUserID`, `ConversationID`

### JSON Tags
- Use camelCase: `"conversationId"`, `"userId"`
- Avoid abbreviations: `"version"` not `"ver"`

### Local Variables
- Short scope: abbreviations OK (`conv`, `msg`, `ctx`)
- Longer scope: full names (`conversation`, `message`)

### Function Parameters
- Use consistent style within a function signature
- Prefer full names for exported functions
- OK to use abbreviations for internal helpers

### Error Variables
- Prefix with `Err`: `ErrNotFound`, `ErrInvalidToken`
- Use `errors.New()` for static errors
- Use `fmt.Errorf()` with `%w` for wrapped errors
```
