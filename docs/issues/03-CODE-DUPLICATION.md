# Code Duplication Issues

This document catalogs repeated code patterns that could be refactored for better maintainability.

## High Priority Refactoring

### 1. Authentication Checks (14 occurrences)

Every handler repeats the same authentication check:

```go
if !s.IsAuthenticated() {
    s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
    return
}
```

**Locations:**
| File | Line | Handler |
|------|------|---------|
| `handlers_conv.go` | 13-17 | HandleDM |
| `handlers_conv.go` | 135-139 | HandleRoom |
| `handlers_conv.go` | ~200 | HandleGet |
| `handlers_conv.go` | ~440 | HandleSend |
| `handlers_conv.go` | ~540 | HandleEdit |
| `handlers_conv.go` | ~610 | HandleUnsend |
| `handlers_conv.go` | ~680 | HandleDelete |
| `handlers_conv.go` | ~750 | HandleReact |
| `handlers_conv.go` | ~810 | HandleTyping |
| `handlers_conv.go` | ~850 | HandleRead |
| `handlers_contact.go` | 11-14 | HandleContact |
| `handlers_invite.go` | 15-18 | HandleInvite |
| Plus 2+ more in handlers.go |

**Recommendation:** Create a wrapper or middleware pattern:

```go
func (h *Handlers) requireAuth(s *Session, msg *ClientMessage, handler func()) {
    if !s.IsAuthenticated() {
        s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
        return
    }
    handler()
}
```

---

### 2. UUID Parsing with Error Handling (11+ occurrences)

Identical UUID parsing pattern repeated throughout:

```go
convID, err := uuid.Parse(dm.ConversationID)
if err != nil {
    s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
    return
}
```

**Locations:**
| File | Line | Context |
|------|------|---------|
| `handlers_conv.go` | 88-92 | handleStartDM |
| `handlers_conv.go` | 286-290 | handleGetMessages |
| `handlers_conv.go` | 344-348 | handleGetMembers |
| `handlers_conv.go` | 391-395 | handleGetReceipts |
| `handlers_conv.go` | 443-447 | HandleSend |
| `handlers_conv.go` | 540-544 | HandleEdit |
| `handlers_conv.go` | ~610 | HandleUnsend |
| `handlers_conv.go` | ~680 | HandleDelete |
| `handlers_conv.go` | ~750 | HandleReact |
| `handlers_conv.go` | ~810 | HandleTyping |
| `handlers_conv.go` | ~850 | HandleRead |

**Recommendation:** Create helper function:

```go
func parseUUID(s *Session, msg *ClientMessage, uuidStr, field string) (uuid.UUID, bool) {
    id, err := uuid.Parse(uuidStr)
    if err != nil {
        s.Send(CtrlError(msg.ID, CodeBadRequest, fmt.Sprintf("invalid %s", field)))
        return uuid.Nil, false
    }
    return id, true
}
```

---

### 3. Membership Verification (4+ occurrences)

Same membership check pattern:

```go
isMember, err := h.db.IsMember(ctx, convID, s.userID)
if err != nil {
    s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
    return
}
if !isMember {
    s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
    return
}
```

**Locations:**
| File | Line |
|------|------|
| `handlers_conv.go` | 94-103 |
| `handlers_conv.go` | 350-359 |
| `handlers_conv.go` | 397-406 |
| `handlers_conv.go` | 765-774 |

**Recommendation:** Create helper:

```go
func (h *Handlers) checkMembership(ctx context.Context, s *Session, msg *ClientMessage, convID uuid.UUID) bool {
    isMember, err := h.db.IsMember(ctx, convID, s.userID)
    if err != nil {
        s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
        return false
    }
    if !isMember {
        s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
        return false
    }
    return true
}
```

---

### 4. Broadcast Messages to Conversation Members (7 occurrences)

Nearly identical broadcast pattern:

```go
memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
infoMsg := &ServerMessage{
    Info: &MsgServerInfo{
        ConversationID: convID.String(),
        From:           s.userID.String(),
        What:           "<action>",
        // ... additional fields
    },
}
h.hub.SendToUsers(memberIDs, infoMsg, s.id)
```

**Locations:**
| File | Line | Action |
|------|------|--------|
| `handlers_conv.go` | 507-522 | data message |
| `handlers_conv.go` | 596-607 | edit |
| `handlers_conv.go` | 663-673 | unsend |
| `handlers_conv.go` | 723-733 | delete |
| `handlers_conv.go` | 785-796 | react |
| `handlers_conv.go` | 827-836 | typing |
| `handlers_conv.go` | 869-879 | read |

**Recommendation:** Create broadcast helper:

```go
func (h *Handlers) broadcastInfo(ctx context.Context, convID uuid.UUID, info *MsgServerInfo, skipSession string) error {
    memberIDs, err := h.db.GetConversationMembers(ctx, convID)
    if err != nil {
        return fmt.Errorf("failed to get members: %w", err)
    }
    h.hub.SendToUsers(memberIDs, &ServerMessage{Info: info}, skipSession)
    return nil
}
```

---

### 5. Secret Decoding for Authentication (3 occurrences)

Base64 decoding and parsing of "username:password" format:

```go
decoded, err := base64.StdEncoding.DecodeString(secret)
if err != nil {
    s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret encoding"))
    return
}
parts := strings.SplitN(string(decoded), ":", 2)
if len(parts) != 2 {
    s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret format"))
    return
}
username, password := parts[0], parts[1]
```

**Locations:**
| File | Line | Context |
|------|------|---------|
| `handlers.go` | 55-68 | handleBasicLogin |
| `handlers.go` | 179-197 | handleCreateAccount |
| `handlers.go` | 310-322 | handleUpdateAccount password change |

**Recommendation:** Create helper:

```go
func decodeCredentials(s *Session, msg *ClientMessage, secret string) (username, password string, ok bool) {
    decoded, err := base64.StdEncoding.DecodeString(secret)
    if err != nil {
        s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret encoding"))
        return "", "", false
    }
    parts := strings.SplitN(string(decoded), ":", 2)
    if len(parts) != 2 {
        s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret format"))
        return "", "", false
    }
    return parts[0], parts[1], true
}
```

---

## Medium Priority Refactoring

### 6. User Response Object Building (3+ occurrences)

Building user info maps with identical fields:

```go
"id":       user.ID.String(),
"public":   user.Public,
"online":   h.hub.IsOnline(user.ID),
"lastSeen": user.LastSeen,
```

**Locations:**
| File | Line |
|------|------|
| `handlers_conv.go` | 79-83 |
| `handlers_conv.go` | 263-268 |
| `handlers_conv.go` | 371-376 |

**Recommendation:** Create helper:

```go
func (h *Handlers) userToMap(user *store.User) map[string]any {
    return map[string]any{
        "id":       user.ID.String(),
        "public":   user.Public,
        "online":   h.hub.IsOnline(user.ID),
        "lastSeen": user.LastSeen,
    }
}
```

---

### 7. Store Package: SQL Row Scanning (5+ occurrences)

Similar row scanning pattern in list-fetching functions:

```go
var results []<Type>
for rows.Next() {
    var item <Type>
    if err := rows.Scan(&item.Field1, &item.Field2, ...); err != nil {
        return nil, err
    }
    results = append(results, item)
}
return results, rows.Err()
```

**Locations:**
| File | Line | Function |
|------|------|----------|
| `store/conversations.go` | 231-244 | GetUserConversations |
| `store/messages.go` | 110-117 | GetMessages |
| `store/contacts.go` | 56-62 | GetContacts |
| `store/invites.go` | 164-190 | GetUserInvites |
| `store/users.go` | 173-179 | SearchUsers |

**Recommendation:** Could use generics (Go 1.18+) for common scan patterns or use sqlx/scany libraries.

---

### 8. Store Package: NULL Handling Pattern (2+ occurrences)

Repeated nullable type conversion:

```go
var inviteeName, usedBy sql.NullString
var usedAt sql.NullTime
// ... scan ...
if inviteeName.Valid {
    invite.InviteeName = &inviteeName.String
}
if usedAt.Valid {
    invite.UsedAt = &usedAt.Time
}
if usedBy.Valid {
    uid, _ := uuid.Parse(usedBy.String)
    invite.UsedBy = &uid
}
```

**Locations:**
| File | Line |
|------|------|
| `store/invites.go` | 86-116 |
| `store/invites.go` | 164-190 |

**Recommendation:** Create helper functions:

```go
func nullStringToPtr(ns sql.NullString) *string {
    if ns.Valid {
        return &ns.String
    }
    return nil
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
    if nt.Valid {
        return &nt.Time
    }
    return nil
}
```

---

### 9. Store Package: pgx.ErrNoRows Handling (8+ occurrences)

Same pattern for handling "no rows" errors:

```go
if errors.Is(err, pgx.ErrNoRows) {
    return nil, nil
}
if err != nil {
    return nil, err
}
```

**Locations:**
| File | Line |
|------|------|
| `store/conversations.go` | 187-189 |
| `store/users.go` | 59-61 |
| `store/messages.go` | 130-132 |
| `store/files.go` | Multiple |
| `store/invites.go` | Multiple |
| `store/contacts.go` | Multiple |

**Recommendation:** Create standardized error handler:

```go
func handleQueryRow(err error) error {
    if errors.Is(err, pgx.ErrNoRows) {
        return nil  // Not found is not an error
    }
    return err
}
```

---

### 10. Optional Field Handling in Response Maps (5+ occurrences)

Checking nullable pointer fields before adding to response:

```go
if m.Head != nil {
    item["head"] = m.Head
}
```

**Locations:**
| File | Line |
|------|------|
| `handlers_conv.go` | 259-261 |
| `handlers_conv.go` | 327-330 |
| `handlers_invite.go` | 117-129 |

**Recommendation:** Create response builder helper with fluent API or use struct with `omitempty` tags.

---

## Summary

| Pattern | Occurrences | Priority | Estimated LOC Savings |
|---------|-------------|----------|----------------------|
| Auth checks | 14 | High | 56 lines |
| UUID parsing | 11 | High | 55 lines |
| Membership verification | 4 | High | 32 lines |
| Broadcast messages | 7 | High | 70+ lines |
| Secret decoding | 3 | Medium | 24 lines |
| User response building | 3 | Medium | 15 lines |
| SQL row scanning | 5 | Medium | Variable |
| NULL handling | 2+ | Medium | Variable |
| pgx.ErrNoRows | 8 | Medium | 24 lines |

**Total Estimated Savings:** 250+ lines of duplicated code

## Recommended Implementation Order

1. **Week 1:** Authentication wrapper and UUID parsing helper
2. **Week 1:** Membership verification helper
3. **Week 2:** Broadcast helper function
4. **Week 2:** Secret decoding helper
5. **Week 3:** Store package helpers (NULL handling, error handling)
6. **Week 3:** Response building patterns
