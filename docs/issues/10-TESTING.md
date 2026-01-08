# Testing Coverage Issues

This document catalogs the testing gaps and recommendations for improving test coverage.

## Executive Summary

The codebase has **critically low test coverage**:
- **1 test file** (`irido/irido_test.go`) out of 28 implementation files
- **~295 lines of tests** covering only the `irido` package
- **~100% of core functionality is untested**

## Critical Untested Components

### 1. Authentication & Security (0% coverage)

**File:** `auth/auth.go` (199 LOC)

| Function | Purpose | Risk if Untested |
|----------|---------|------------------|
| `HashPassword()` | Argon2id password hashing | Weak passwords, storage issues |
| `VerifyPassword()` | Password verification | Auth bypass, timing attacks |
| `GenerateToken()` | JWT token generation | Token forgery |
| `ValidateToken()` | JWT token validation | Auth bypass |
| `ValidateUsername()` | Username validation | Invalid usernames |
| `ValidatePassword()` | Password strength | Weak passwords |

---

### 2. Encryption (0% coverage)

**File:** `crypto/crypto.go` (120 LOC)

| Function | Purpose | Risk if Untested |
|----------|---------|------------------|
| `NewEncryptor()` | AES-GCM cipher setup | Key validation issues |
| `Encrypt()` | Message encryption | Data exposure |
| `Decrypt()` | Message decryption | Data loss |
| `EncryptString/DecryptString` | Base64 variants | Encoding issues |
| `GenerateKey()` | Key generation | Weak keys |

---

### 3. Database Operations (0% coverage)

**Files:** `store/*.go` (1,442 LOC total)

**56 untested functions including:**

| Category | Functions | Impact |
|----------|-----------|--------|
| Users | `CreateUser`, `GetUserByID`, `UpdatePassword` | Account issues |
| Messages | `CreateMessage`, `EditMessage`, `UnsendMessage` | Data loss |
| Conversations | `CreateDM`, `CreateRoom`, `IsMember` | Access control |
| Files | `CreateFile`, `CanAccessFile`, `DeleteFile` | File access |
| Contacts | `AddContact`, `RemoveContact`, `IsContact` | Contact sync |
| Invites | `CreateInviteCode`, `UseInviteCode` | Invite abuse |

---

### 4. Configuration (0% coverage)

**File:** `config/config.go` (264 LOC)

| Function | Purpose | Risk |
|----------|---------|------|
| `Load()` | YAML parsing | Config errors |
| `expandEnvVars()` | Environment substitution | Security issues |
| `applyDefaults()` | Default values | Wrong defaults |
| `validate()` | Required field checks | Missing validation |

---

### 5. WebSocket/Protocol (0% coverage)

**Files:** `hub.go`, `session.go`, `presence.go` (~650 LOC)

| Component | Functions | Risk |
|-----------|-----------|------|
| Hub | `Run()`, `SendToUsers()` | Message loss |
| Session | `readPump()`, `writePump()` | Connection issues |
| Presence | `UserOnline()`, `UserOffline()` | Wrong status |

---

### 6. Handlers (0% coverage)

**Files:** `handlers*.go` (~1,400 LOC)

| Handler | Purpose | Risk |
|---------|---------|------|
| `HandleLogin` | Authentication | Auth bypass |
| `HandleAcc` | Account management | Account issues |
| `HandleDM` | Direct messages | Message loss |
| `HandleSend` | Message sending | Data corruption |
| `HandleEdit` | Message editing | Edit bugs |
| `HandleUpload` | File uploads | Upload failures |

---

## Currently Tested

### irido Package (~45% coverage)

**File:** `irido/irido_test.go` (295 LOC)

| Test | Coverage |
|------|----------|
| `TestNew` | Message creation |
| `TestIsIrido` | Type detection |
| `TestParse` | Message parsing |
| `TestPlainText` | Text extraction |
| `TestPreview` | Preview generation |
| `TestGetFileRefs` | File reference extraction |
| `TestGetMentionedUsers` | Mention extraction |
| `TestValidate` | Validation rules |
| `TestToJSON` | Serialization |

**Quality:** Good use of table-driven tests, covers happy path and edge cases.

---

## Missing Test Infrastructure

### 1. No Database Mocking

Current code uses concrete `*store.DB` everywhere:
```go
type Handlers struct {
    db *store.DB  // Concrete type, not interface
}
```

**Needed:** Database interface for mocking:
```go
type Store interface {
    CreateUser(ctx context.Context, public json.RawMessage) (uuid.UUID, error)
    GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
    // ... 50+ methods
}
```

---

### 2. No Redis Mocking

Redis client used directly:
```go
type Hub struct {
    redis *redis.Client  // Concrete type
}
```

**Needed:** Redis interface:
```go
type RedisClient interface {
    SetOnline(ctx context.Context, userID string) error
    IsOnline(ctx context.Context, userID string) (bool, error)
    Publish(ctx context.Context, channel, msgType string, payload any) error
}
```

---

### 3. No Test Utilities

Missing:
- Test fixtures
- Database setup/teardown helpers
- Mock factories
- Assertion helpers

---

### 4. No Integration Tests

No tests for:
- End-to-end auth flow
- Message send/receive
- Multi-node scenarios
- Database transactions

---

### 5. No CI/CD Integration

Missing:
- GitHub Actions workflow
- Test coverage reporting
- Pre-commit hooks

---

## Recommended Test Implementation

### Phase 1: Critical Infrastructure (Week 1-2)

**1.1 Create Store Interface**
```go
// store/interface.go
type Store interface {
    // Users
    CreateUser(ctx context.Context, public json.RawMessage) (uuid.UUID, error)
    GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
    // ... all store methods
}
```

**1.2 Add Auth Tests**
```go
// auth/auth_test.go
func TestHashPassword(t *testing.T) { }
func TestVerifyPassword(t *testing.T) { }
func TestGenerateToken(t *testing.T) { }
func TestValidateToken(t *testing.T) { }
func TestValidatePassword(t *testing.T) { }
```

**1.3 Add Crypto Tests**
```go
// crypto/crypto_test.go
func TestEncryptDecrypt(t *testing.T) { }
func TestEncryptString(t *testing.T) { }
func TestInvalidKey(t *testing.T) { }
```

**1.4 Add Config Tests**
```go
// config/config_test.go
func TestLoad(t *testing.T) { }
func TestExpandEnvVars(t *testing.T) { }
func TestValidate(t *testing.T) { }
func TestDefaults(t *testing.T) { }
```

---

### Phase 2: Store Package (Week 3-4)

**2.1 Create Mock Store**
```go
// store/mock.go
type MockStore struct {
    CreateUserFn    func(...) (uuid.UUID, error)
    GetUserByIDFn   func(...) (*User, error)
    // ... mock implementations
}
```

**2.2 Add Store Tests (with test database)**
```go
// store/users_test.go
func TestCreateUser(t *testing.T) { }
func TestGetUserByID(t *testing.T) { }
func TestUpdatePassword(t *testing.T) { }

// store/messages_test.go
func TestCreateMessage(t *testing.T) { }
func TestEditMessage(t *testing.T) { }
func TestGetMessages(t *testing.T) { }
```

---

### Phase 3: Handler Tests (Week 5-6)

**3.1 Add Handler Tests with Mocks**
```go
// handlers_test.go
func TestHandleLogin_Basic(t *testing.T) { }
func TestHandleLogin_Token(t *testing.T) { }
func TestHandleLogin_InvalidCredentials(t *testing.T) { }
func TestHandleAcc_Create(t *testing.T) { }
func TestHandleAcc_Update(t *testing.T) { }
```

---

### Phase 4: Integration Tests (Week 7-8)

**4.1 End-to-End Flow Tests**
```go
// integration/auth_test.go
func TestAuthFlow(t *testing.T) {
    // Create account
    // Login with credentials
    // Validate token
    // Refresh token
}

// integration/messaging_test.go
func TestMessagingFlow(t *testing.T) {
    // Create two users
    // Start DM
    // Send message
    // Verify delivery
    // Edit message
    // Delete message
}
```

---

## Priority Matrix

| Component | Priority | Effort | Risk if Untested |
|-----------|----------|--------|------------------|
| auth/ | Critical | Low | Auth bypass |
| crypto/ | Critical | Low | Data exposure |
| config/ | High | Low | Startup failures |
| store/ (core) | High | Medium | Data corruption |
| handlers (auth) | High | Medium | Security issues |
| handlers (msg) | Medium | Medium | Message loss |
| WebSocket | Medium | High | Connection issues |
| Integration | Medium | High | E2E failures |

---

## Suggested Test Coverage Goals

| Phase | Target Coverage | Timeline |
|-------|----------------|----------|
| Phase 1 | auth 90%, crypto 90%, config 80% | 2 weeks |
| Phase 2 | store 70% | 2 weeks |
| Phase 3 | handlers 60% | 2 weeks |
| Phase 4 | integration 50% | 2 weeks |
| Ongoing | All packages 80%+ | Continuous |

---

## CI/CD Configuration

**`.github/workflows/test.yml`:**
```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: mvchat2_test
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: go test -v -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v3
```

---

## Summary

| Metric | Current | Target |
|--------|---------|--------|
| Test files | 1 | 15+ |
| Lines of tests | 295 | 3,000+ |
| Coverage | ~3% | 80%+ |
| Integration tests | 0 | 10+ |
| CI/CD | No | Yes |

**Immediate Actions:**
1. Create store interface for mocking
2. Add auth and crypto unit tests
3. Add config tests
4. Set up CI/CD pipeline

**The codebase should not be deployed to production without significantly improved test coverage.**
