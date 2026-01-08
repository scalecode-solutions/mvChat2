# Codebase Issues & Improvement Opportunities

This directory contains documentation of issues, inconsistencies, and improvement opportunities identified in the mvChat2 codebase.

**Analysis Date:** January 8, 2026
**Scope:** Primary backend codebase (excluding SDK)

## Summary

| Category | Critical | High | Medium | Low | Total |
|----------|----------|------|--------|-----|-------|
| [Error Handling](01-ERROR-HANDLING.md) | 2 | 3 | 5 | 1 | 11 |
| [Security](02-SECURITY.md) | 3 | 5 | 6 | 2 | 16 |
| [Code Duplication](03-CODE-DUPLICATION.md) | - | 4 | 6 | - | 10 |
| [Naming Conventions](04-NAMING-CONVENTIONS.md) | - | 3 | 3 | 3 | 9 |
| [Type Definitions](05-TYPE-DEFINITIONS.md) | 2 | 2 | 4 | 2 | 10 |
| [Database Schema](06-DATABASE-SCHEMA.md) | 2 | 4 | 4 | 3 | 13 |
| [WebSocket/Protocol](07-WEBSOCKET-PROTOCOL.md) | 3 | 4 | 4 | 4 | 15 |
| [Configuration](08-CONFIGURATION.md) | 3 | 4 | 5 | 6 | 18 |
| [Documentation](09-DOCUMENTATION.md) | - | 4 | 5 | 3 | 12 |
| [Testing](10-TESTING.md) | 1 | - | - | - | 1 |
| [File Deduplication](11-FILE-DEDUPLICATION.md) | 1 | 2 | 1 | - | 4 |
| **Total** | **17** | **35** | **43** | **24** | **119** |

## Issue Files

### [01-ERROR-HANDLING.md](01-ERROR-HANDLING.md)
Issues with error handling patterns including:
- Silently ignored database errors (11 locations)
- Missing error handling on critical operations
- Inconsistent error codes
- Generic error messages without context

### [02-SECURITY.md](02-SECURITY.md)
Security vulnerabilities including:
- Unrestricted CORS allowing all origins
- Hardcoded cryptographic keys
- No rate limiting on any endpoint
- Weak password validation
- File access control bypass
- Missing request timeouts

### [03-CODE-DUPLICATION.md](03-CODE-DUPLICATION.md)
Repeated code patterns that could be refactored:
- Authentication checks (14 occurrences)
- UUID parsing (11+ occurrences)
- Membership verification (4+ occurrences)
- Broadcast messages (7 occurrences)
- Estimated 250+ lines of duplicated code

### [04-NAMING-CONVENTIONS.md](04-NAMING-CONVENTIONS.md)
Naming inconsistencies including:
- Mixed "Conversation" vs "Conv" terminology
- Abbreviated field names (`Uname`, `DelID`)
- Inconsistent JSON tag naming
- Parameter naming variations

### [05-TYPE-DEFINITIONS.md](05-TYPE-DEFINITIONS.md)
Type and struct issues including:
- Missing JSON tags on store types
- Embedded struct field duplication
- Missing validation tags
- Inconsistent time type usage

### [06-DATABASE-SCHEMA.md](06-DATABASE-SCHEMA.md)
Database and query issues including:
- Schema mismatch (`uploader_id` vs `user_id`)
- N+1 query problem in `GetUserConversations`
- Missing indexes on `members` and `message_deletions`
- Missing transaction in `AddContact`
- Race condition in reaction updates

### [07-WEBSOCKET-PROTOCOL.md](07-WEBSOCKET-PROTOCOL.md)
WebSocket implementation issues including:
- Data race in Session fields
- No rate limiting on messages
- Race condition in Send()
- Missing message ID validation
- Multiple message types allowed simultaneously
- Aggressive close on buffer full

### [08-CONFIGURATION.md](08-CONFIGURATION.md)
Configuration and deployment issues including:
- Hardcoded cryptographic keys with defaults
- Empty encryption key allowed
- Inconsistent Docker credentials
- Database SSL disabled by default
- Missing logging configuration
- No graceful shutdown timeout

### [09-DOCUMENTATION.md](09-DOCUMENTATION.md)
Documentation gaps including:
- Missing godoc on 20+ exported types
- Critical TODOs not addressed
- Missing protocol/API documentation
- Complex code without explanation
- Outdated comments

### [10-TESTING.md](10-TESTING.md)
Testing coverage issues including:
- Only 1 test file in entire codebase
- 0% coverage on auth, crypto, config, store
- No database mocking infrastructure
- No integration tests
- No CI/CD pipeline

### [11-FILE-DEDUPLICATION.md](11-FILE-DEDUPLICATION.md)
File storage deduplication issues including:
- Schema mismatch (`user_id` vs `uploader_id`)
- Hash field never populated (dedup broken)
- Status type mismatch (INT vs string)
- File access control bypass
- See also: [FILE-DEDUPLICATION-ANALYSIS.md](FILE-DEDUPLICATION-ANALYSIS.md) for detailed comparison with FilesAPI and original mvChat

## Priority Recommendations

> **See [IMMEDIATE-PRIORITIES.md](IMMEDIATE-PRIORITIES.md) for detailed implementation guide with code examples.**

### Immediate (Before Production)

1. **Security:** Remove hardcoded cryptographic keys
2. **Security:** Implement CORS origin validation
3. **Security:** Fix file access control bypass
4. **Security:** Add request timeouts
5. **Database:** Fix schema mismatch (`uploader_id`)
6. **Testing:** Add auth and crypto unit tests

### High Priority (Sprint 1-2)

1. **Security:** Implement rate limiting
2. **Security:** Strengthen password validation
3. **Error Handling:** Fix silently ignored errors in broadcasts
4. **Database:** Add missing indexes
5. **Database:** Wrap `AddContact` in transaction
6. **Code Quality:** Refactor authentication check duplication
7. **WebSocket:** Add mutex protection to Session fields
8. **Testing:** Create store interface for mocking

### Medium Priority (Sprint 3-4)

1. **Configuration:** Add logging configuration
2. **Configuration:** Make WebSocket parameters configurable
3. **Code Quality:** Refactor UUID parsing duplication
4. **Code Quality:** Refactor broadcast message duplication
5. **Documentation:** Add godoc to exported types
6. **Documentation:** Document protocol message types
7. **Naming:** Standardize JSON tag conventions
8. **Testing:** Add handler tests

### Low Priority (Ongoing)

1. **Naming:** Rename abbreviated fields
2. **Types:** Add validation tags
3. **Documentation:** Resolve TODO comments
4. **Testing:** Add integration tests
5. **Configuration:** Add health check for dependencies

## Files by Lines of Code

| File | LOC | Test Coverage |
|------|-----|---------------|
| store/conversations.go | 379 | 0% |
| handlers_conv.go | 400+ | 0% |
| handlers.go | 365 | 0% |
| redis/redis.go | 292 | 0% |
| media/media.go | 271 | 0% |
| config/config.go | 264 | 0% |
| store/messages.go | 261 | 0% |
| store/invites.go | 222 | 0% |
| email/email.go | 208 | 0% |
| auth/auth.go | 199 | 0% |
| store/users.go | 181 | 0% |
| store/files.go | 164 | 0% |
| irido/irido.go | 532 | ~45% |

## How to Use This Documentation

1. **For Sprint Planning:** Use priority recommendations to plan work
2. **For Code Review:** Reference specific issue files when reviewing related code
3. **For New Contributors:** Read to understand known technical debt
4. **For Tracking:** Update issue files as problems are resolved

## Contributing

When fixing issues documented here:
1. Reference the issue file in your commit message
2. Update the issue file to mark items as resolved
3. Add tests for fixed issues where applicable
