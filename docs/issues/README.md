# Codebase Issues & Improvement Opportunities

This directory contains documentation of issues, inconsistencies, and improvement opportunities identified in the mvChat2 codebase.

**Analysis Date:** January 8, 2026
**Last Updated:** January 8, 2026
**Scope:** Primary backend codebase (excluding SDK)

## Status Summary

Most critical and high-priority security issues have been resolved. The codebase is now production-ready from a security standpoint.

| Category | Original | Resolved | Remaining |
|----------|----------|----------|-----------|
| Security (Critical) | 3 | 3 | 0 |
| Security (High) | 5 | 5 | 0 |
| Security (Medium) | 6 | 2 | 4 |
| Database (Critical) | 2 | 2 | 0 |
| Database (High) | 4 | 2 | 2 |
| WebSocket (Critical) | 3 | 3 | 0 |
| WebSocket (High) | 4 | 1 | 3 |
| Testing | 1 | 0 | 1 |

## Resolved Issues

### Security - All Critical/High Fixed

| Issue | Status | Resolution |
|-------|--------|------------|
| Hardcoded cryptographic keys | FIXED | `config/config.go` validates against `knownInsecureKeys`, refuses to start with defaults |
| CORS origin validation | FIXED | Handled by Caddy reverse proxy |
| Request timeouts | FIXED | Handler timeouts added to all database operations |
| File access control bypass | FIXED | `CanAccessFile()` now checks uploader OR message reference in user's conversations |
| Rate limiting | FIXED | Caddy-based (100 req/min API, 10 req/min auth) + WebSocket message rate limiting |
| Weak invite codes | FIXED | Two-layer system with HMAC signatures, cryptographic tokens |
| Invite token exposure | FIXED | AES-256-GCM encryption before database storage |
| Email header/HTML injection | FIXED | Proper sanitization in email package |
| SQL injection (UpdateMemberSettings) | FIXED | Now uses typed `MemberSettings` struct, not dynamic map keys |

### Database - Schema Fixed

| Issue | Status | Resolution |
|-------|--------|------------|
| Schema mismatch (uploader_id) | FIXED | Migration 004 renamed column |
| Status type mismatch | FIXED | Migration 004 changed to VARCHAR |
| Hash NOT NULL constraint | FIXED | Migration 004 made nullable |
| Missing indexes | FIXED | Migration 005 added idx_members_conv_user, idx_message_deletions_user, idx_messages_created, idx_files_hash, idx_files_status |

### WebSocket - Concurrency Fixed

| Issue | Status | Resolution |
|-------|--------|------------|
| Session field data races | FIXED | `sync.RWMutex` added to Session struct with proper getters/setters |
| Send() race condition | FIXED | `defer recover()` pattern + atomic closing flag + sync.Once for Close() |
| WebSocket message rate limiting | FIXED | Token bucket rate limiter in session.go |

### Features Added

| Feature | Status | Details |
|---------|--------|---------|
| must_change_password flag | ADDED | Users signing up with invite code as password are flagged |
| Email on user account | ADDED | Email stored from invite, updateable via acc message |
| Password change endpoint | ADDED | Via acc message with secret field |
| Unsend time limit | ADDED | 5 minute window |
| Edit limits | ADDED | 10 edits per message within 15 minutes |

## Remaining Issues

### High Priority

| Issue | File | Notes |
|-------|------|-------|
| N+1 query in GetUserConversations | store/conversations.go | Performance - refactor with JOIN |
| Missing transaction in AddContact | store/contacts.go | Data integrity risk |
| Message ID validation | session.go | Protocol - should require ID for stateful ops |
| Multiple message types validation | session.go | Protocol - should validate exactly one type |
| Connection leak on channel full | hub.go | Resource management |

### Medium Priority

| Issue | File | Notes |
|-------|------|-------|
| Database SSL disabled by default | config/config.go | Security hardening |
| Missing X-Forwarded-For validation | server.go | IP spoofing possible |
| Sensitive data in logs | various | PII in error messages |
| Missing security headers | - | Handled by Caddy, but app could add them |
| Reaction race condition | store/messages.go | Non-atomic JSONB update |
| Hub lock released early | hub.go | Potential race with session pointers |

### Low Priority

| Issue | File | Notes |
|-------|------|-------|
| lastAction field unused | session.go | Idle timeout not implemented |
| Silent WriteJSON errors | session.go | Should log for debugging |
| Redis errors silently ignored | hub.go, presence.go | Should log for monitoring |
| Code duplication | various | ~250 lines could be refactored |
| Missing godoc | various | 20+ exported types undocumented |

### Testing

| Issue | Priority | Notes |
|-------|----------|-------|
| ~3% test coverage | High | Only irido package has tests |
| No store mocking | High | Need interface for testing |
| No integration tests | Medium | End-to-end testing needed |
| No CI/CD pipeline | Medium | Automated testing/deployment |

## Issue Files

- [01-ERROR-HANDLING.md](01-ERROR-HANDLING.md) - Error handling patterns (partially resolved)
- [02-SECURITY.md](02-SECURITY.md) - Security vulnerabilities (mostly resolved)
- [03-CODE-DUPLICATION.md](03-CODE-DUPLICATION.md) - Repeated code patterns (not started)
- [04-NAMING-CONVENTIONS.md](04-NAMING-CONVENTIONS.md) - Naming inconsistencies (not started)
- [05-TYPE-DEFINITIONS.md](05-TYPE-DEFINITIONS.md) - Type and struct issues (not started)
- [06-DATABASE-SCHEMA.md](06-DATABASE-SCHEMA.md) - Database issues (mostly resolved)
- [07-WEBSOCKET-PROTOCOL.md](07-WEBSOCKET-PROTOCOL.md) - WebSocket issues (partially resolved)
- [08-CONFIGURATION.md](08-CONFIGURATION.md) - Configuration issues (mostly resolved)
- [09-DOCUMENTATION.md](09-DOCUMENTATION.md) - Documentation gaps (not started)
- [10-TESTING.md](10-TESTING.md) - Testing coverage (not started)
- [11-FILE-DEDUPLICATION.md](11-FILE-DEDUPLICATION.md) - File storage (schema fixed, dedup not implemented)
- [IMMEDIATE-PRIORITIES.md](IMMEDIATE-PRIORITIES.md) - Critical fixes checklist (mostly complete)

## Next Steps

1. **Testing Infrastructure** - Add store interface, unit tests for auth/crypto
2. **File Deduplication** - Implement hash calculation on upload
3. **Performance** - Fix N+1 query in GetUserConversations
4. **Code Quality** - Refactor duplicated patterns
5. **Documentation** - Add godoc to exported types

## Migrations Applied

| Migration | Description |
|-----------|-------------|
| 002 | Invite codes table |
| 003 | Contacts table |
| 004 | Fix files schema (uploader_id, status VARCHAR, hash nullable) |
| 005 | Add missing indexes |
| 006 | Add must_change_password to users |
| 007 | Add email to users |
