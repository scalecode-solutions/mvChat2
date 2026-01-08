# Error Handling Issues

This document catalogs inconsistencies and problems with error handling throughout the codebase.

## Critical Issues

### 1. Silently Ignored Errors in Database Operations

Multiple database calls use the blank identifier `_` to ignore errors, which can lead to silent failures and data inconsistencies.

**Affected Files:**

| File | Line | Code | Impact |
|------|------|------|--------|
| `handlers_invite.go` | 66 | `inviter, _ := h.db.GetUserByID(ctx, s.userID)` | Inviter info missing silently |
| `handlers_invite.go` | 178 | `inviter, _ := h.db.GetUserByID(ctx, invite.InviterID)` | Same in redemption flow |
| `handlers_conv.go` | 216 | `user, _ := h.db.GetUserByID(ctx, c.ContactID)` | Contact list iteration |
| `handlers_conv.go` | 369 | `user, _ := h.db.GetUserByID(ctx, uid)` | Member list iteration |
| `handlers_conv.go` | 507 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Message broadcast |
| `handlers_conv.go` | 596 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Edit broadcast |
| `handlers_conv.go` | 663 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Unsend broadcast |
| `handlers_conv.go` | 723 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Delete broadcast |
| `handlers_conv.go` | 785 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Reaction broadcast |
| `handlers_conv.go` | 827 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Typing indicator |
| `handlers_conv.go` | 869 | `memberIDs, _ := h.db.GetConversationMembers(ctx, convID)` | Read receipt |

**Recommendation:** Handle errors properly or at minimum log them. For broadcast operations, consider logging errors but continuing to ensure messages reach available users.

---

### 2. Missing Error Handling for Critical Operations

Some operations that should always check errors don't:

| File | Line | Operation | Risk |
|------|------|-----------|------|
| `handlers_invite.go` | 175 | `h.db.AddContact(ctx, invite.InviterID, s.userID, "invite", &invite.ID)` | Contact not added after invite |
| `handlers_invite.go` | 209 | `h.db.AddContact(ctx, invite.InviterID, newUserID, "invite", &invite.ID)` | Same in RedeemInviteCode |
| `handlers_invite.go` | 234 | `h.db.AddContact(ctx, other.InviterID, newUserID, "invite", &other.ID)` | Secondary invites |
| `handlers_files.go` | 92 | `fh.db.UpdateFileStatus(r.Context(), fileID, "failed")` | Failed uploads not marked |
| `handlers_files.go` | 146 | `fh.db.CreateFileMetadata(ctx, fileID, width, height, duration, thumbnail, nil)` | Metadata silently lost |
| `handlers_files.go` | 149 | `fh.db.UpdateFileStatus(ctx, fileID, "ready")` | Status not updated |

---

## High Severity Issues

### 3. Inconsistent Error Codes for Same Conditions

The same logical error returns different HTTP/protocol status codes in different places:

| Condition | Location | Code Used |
|-----------|----------|-----------|
| "user not found" | `handlers.go:90` | `CodeInternalError (500)` |
| "user not found" | `handlers.go:132` | `CodeUnauthorized (401)` |
| "user not found" | `handlers_conv.go:60` | `CodeNotFound (404)` |

**Recommendation:** Standardize error codes:
- `404` for resource not found (conversations, files, messages)
- `401` for authentication failures (invalid credentials)
- `403` for authorization failures (not a member)
- `500` for unexpected internal errors

---

### 4. Incorrect HTTP Status Code

**File:** `handlers_files.go:198`

```go
if fileWithMeta.Status != "ready" {
    http.Error(w, "file not ready", http.StatusAccepted)  // 202 is wrong!
    return
}
```

`StatusAccepted (202)` means "request accepted for processing" - semantically incorrect here. Should be:
- `404 Not Found` if file doesn't exist in usable form
- `409 Conflict` if file is still processing
- `503 Service Unavailable` with `Retry-After` header

---

### 5. Encryption Error Masked by Fallback

**File:** `handlers_conv.go:320-325`

```go
plaintext, err := h.encryptor.Decrypt(m.Content)
if err == nil {
    item["content"] = plaintext
} else {
    item["content"] = m.Content // Fallback for unencrypted messages
}
```

**Issues:**
1. Could expose encrypted ciphertext to clients if decryption fails
2. No logging of decryption failures
3. Silent fallback masks real encryption problems

---

## Medium Severity Issues

### 6. Generic "database error" Messages

The message "database error" appears 46+ times with no detail about which query failed:

**Files with generic errors:**
- `handlers.go` - Lines 73, 212
- `handlers_conv.go` - Lines 56, 295 (repeated 10+ times)
- All handler files

**Recommendation:** Add error context using `fmt.Errorf("failed to get user %s: %w", userID, err)` pattern.

---

### 7. Missing Error Wrapping in Store Package

Store package functions return raw errors without context:

**Example from `store/users.go:46`:**
```go
if err != nil {
    return uuid.Nil, err  // No context about what operation failed
}
```

**Recommendation:** Wrap all errors:
```go
if err != nil {
    return uuid.Nil, fmt.Errorf("failed to create user: %w", err)
}
```

---

### 8. Silent JSON Operations

**File:** `handlers_conv.go`

```go
// Line 482 - Marshal error ignored
head, _ = json.Marshal(map[string]any{"reply_to": send.ReplyTo})

// Line 510 - Unmarshal error ignored
json.Unmarshal(head, &headMap)
```

While unlikely to fail, these should at minimum be logged.

---

### 9. Goroutine Errors Logged to stdout

**File:** `handlers_invite.go:83-89`

```go
go func() {
    if err := h.email.SendInvite(...); err != nil {
        fmt.Printf("Failed to send invite email to %s: %v\n", ...)  // Goes to stdout
    }
}()
```

**Issues:**
- Uses `fmt.Printf` instead of proper logging
- No retry mechanism
- Email failure doesn't affect user experience (silent)

---

### 10. Inconsistent Error Response Patterns

**HTTP handlers** use `http.Error()`:
- `handlers_files.go:59` - `http.StatusBadRequest`
- `handlers_files.go:84` - `http.StatusInternalServerError`

**WebSocket handlers** use custom control messages:
- `CodeBadRequest (400)`, `CodeInternalError (500)` etc.

This creates inconsistency between HTTP and WebSocket error handling.

---

## Low Severity Issues

### 11. WebSocket Upgrade Error Logging

**File:** `server.go:45-48`

```go
conn, err := upgrader.Upgrade(w, r, nil)
if err != nil {
    fmt.Printf("WebSocket upgrade failed: %v\n", err)  // stdout, not structured log
    return
}
```

---

## Summary by Severity

| Severity | Count | Description |
|----------|-------|-------------|
| Critical | 2 | Silently ignored errors affecting data integrity |
| High | 3 | Inconsistent codes, encryption fallback, HTTP status |
| Medium | 5 | Generic messages, missing wrapping, JSON errors |
| Low | 1 | Logging to stdout |

## Recommended Actions

1. **Immediate:** Fix all `_` error ignoring in broadcast operations
2. **Immediate:** Add error handling to `AddContact` calls
3. **High:** Standardize error codes across all handlers
4. **High:** Fix encryption fallback to log and potentially fail
5. **Medium:** Add error wrapping throughout store package
6. **Medium:** Replace `fmt.Printf` with structured logging
