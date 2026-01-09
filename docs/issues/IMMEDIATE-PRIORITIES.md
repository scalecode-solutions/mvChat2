# Immediate Priorities - Critical Fixes Status

**Created:** January 8, 2026
**Last Updated:** January 8, 2026
**Status:** MOSTLY COMPLETE

This document tracks the critical issues that were identified for production readiness. Most have been resolved.

---

## Verification Checklist

### Security - ALL COMPLETE

- [x] `JWT_SECRET` and `ENCRYPTION_KEY` required in production
  - **Resolution:** `config/config.go:287-358` validates against `knownInsecureKeys` map
  - Server refuses to start with default or empty keys

- [x] CORS only allows configured origins
  - **Resolution:** Handled by Caddy reverse proxy

- [x] Server has read/write/idle timeouts
  - **Resolution:** Handler timeouts added with `context.WithTimeout()`

- [x] File access restricted to uploader or conversation members
  - **Resolution:** `store/files.go:203-239` implements proper SQL check

- [x] Rate limiting active on auth and upload endpoints
  - **Resolution:** Caddy-based (100 req/min API, 10 req/min auth)
  - WebSocket message rate limiting via `ratelimit` package

- [x] Invite codes cryptographically secure
  - **Resolution:** Two-layer system with HMAC signatures + AES-256-GCM encryption

- [x] Email injection prevented
  - **Resolution:** `email/email.go` sanitizes headers and HTML-escapes content

### Database - ALL COMPLETE

- [x] Schema matches code (uploader_id, string status)
  - **Resolution:** Migration 004 (`store/migrations/004_fix_files_schema.sql`)

- [x] All required indexes created
  - **Resolution:** Migration 005 (`store/migrations/005_add_indexes.sql`)
  - Indexes: idx_members_conv_user, idx_message_deletions_user, idx_messages_created, idx_files_hash, idx_files_status

### WebSocket - CRITICAL ITEMS COMPLETE

- [x] No data races detected by `go run -race`
  - **Resolution:** `session.go:40-54` adds `sync.RWMutex` with proper getters/setters

- [x] Send() race condition fixed
  - **Resolution:** `defer recover()` pattern + atomic closing flag + `sync.Once` for Close()

- [x] WebSocket message rate limiting
  - **Resolution:** Token bucket rate limiter in `session.go:206`

- [ ] Broadcast errors logged
  - **Status:** Not yet implemented - errors still silently ignored in some places

---

## Remaining High Priority Items

### Protocol Improvements (Not Critical)

| Task | File | Notes |
|------|------|-------|
| Validate message ID present | session.go | Should require ID for stateful operations |
| Validate single message type | session.go | Should reject if multiple types set |
| Non-blocking unregister | hub.go | Prevent goroutine hangs |

### Performance (Not Critical)

| Task | File | Notes |
|------|------|-------|
| Fix N+1 query | store/conversations.go | GetUserConversations does 2 queries per DM |
| Wrap AddContact in transaction | store/contacts.go | Currently two separate INSERTs |

### Observability (Not Critical)

| Task | File | Notes |
|------|------|-------|
| Log broadcast errors | hub.go | Currently silently ignored |
| Log Redis errors | hub.go, presence.go | Currently silently ignored |
| Structured logging with PII redaction | various | Currently uses fmt.Printf |

---

## Implementation Summary

### Completed Security Fixes

1. **Cryptographic Key Validation** (`config/config.go`)
   - `knownInsecureKeys` map blocks all default/placeholder values
   - Minimum length requirements enforced
   - Server fails to start with insecure configuration

2. **File Access Control** (`store/files.go:203-239`)
   ```go
   // Access granted if:
   // 1. User is the uploader
   // 2. File is referenced in a message in a conversation the user is a member of
   ```

3. **Invite Code Security** (two-layer system)
   - Short 10-digit code for user sharing
   - Cryptographic token with HMAC signature
   - AES-256-GCM encryption before database storage
   - Proper expiration handling

4. **Session Concurrency** (`session.go`)
   - `sync.RWMutex` protects user fields
   - Token bucket rate limiting for messages
   - Safe close with `sync.Once`

### Completed Database Fixes

1. **Migration 004** - Fixed files table schema
   - Renamed `user_id` to `uploader_id`
   - Changed `status` from INT to VARCHAR
   - Made `hash` nullable

2. **Migration 005** - Added missing indexes
   - Composite index on members(conversation_id, user_id)
   - Index on message_deletions(user_id)
   - Index on messages(created_at DESC)
   - Partial indexes on files(hash) and files(status)

3. **Migration 006** - Added `must_change_password` to users

4. **Migration 007** - Added `email` to users

---

## Next Steps After Immediate Fixes

1. **Testing Infrastructure** - Create store interface for mocking, add unit tests
2. **File Deduplication** - Implement hash calculation on upload (schema ready)
3. **Code Quality** - Refactor duplicated authentication checks
4. **Documentation** - Add godoc to exported types
