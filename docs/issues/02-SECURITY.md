# Security Issues

This document catalogs security vulnerabilities and concerns identified in the codebase.

## Critical Issues

### 1. Unrestricted CORS Configuration

**File:** `server.go:15-17`

```go
CheckOrigin: func(r *http.Request) bool {
    return true // Allow all origins for now
},
```

The WebSocket upgrader accepts connections from ANY origin without validation.

**Impact:** Enables Cross-Origin WebSocket Hijacking attacks. Malicious websites can initiate WebSocket connections on behalf of authenticated users.

**Recommendation:** Implement origin validation against allowed domains from configuration.

---

### 2. Hardcoded Cryptographic Keys

**Files:** `mvchat2.yaml`, `Dockerfile`

Default cryptographic keys are embedded in configuration:

| Key | File | Line | Purpose |
|-----|------|------|---------|
| `uid_key` | `mvchat2.yaml` | 22 | User ID derivation |
| `encryption_key` | `mvchat2.yaml` | 23 | Message encryption (AES-256-GCM) |
| `api_key_salt` | `mvchat2.yaml` | 42 | API key salting |
| `token.key` | `mvchat2.yaml` | 49 | JWT signing (HS256) |

```yaml
uid_key: ${UID_KEY:la6YsO+bNX/+XIkOqc5Svw==}
encryption_key: ${ENCRYPTION_KEY:k8Jz9mN2pQ4rT6vX8yB1dF3hK5nP7sU0wA2cE4gI6jL=}
api_key_salt: ${API_KEY_SALT:T713/rYYgW7g4m3vG6zGRh7+FM1t0T8j13koXScOAj4=}
key: ${TOKEN_KEY:wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=}
```

**Impact:** If repository is public or exposed:
- All encrypted messages can be decrypted
- All JWT tokens can be forged
- User IDs can be predicted

**Recommendation:**
- Remove all default values from configuration
- Require keys via environment variables with no fallback
- Add validation to prevent startup with default/empty keys

---

### 3. Dynamic SQL Column Names (Potential SQL Injection)

**File:** `store/conversations.go:298-314`

```go
func (db *DB) UpdateMemberSettings(ctx context.Context, convID, userID uuid.UUID, updates map[string]any) error {
    for key, val := range updates {
        setClauses += key + " = $" + string(rune('0'+i))  // Column name not parameterized!
    }
    query := "UPDATE members SET " + setClauses + " WHERE conversation_id = $1 AND user_id = $2"
}
```

While column names currently come from code (not user input), this pattern is dangerous:
- No whitelist validation of allowed column names
- If `updates` map ever receives user-supplied keys, SQL injection is possible

**Recommendation:** Use a whitelist of allowed column names and validate before building query.

---

## High Severity Issues

### 4. Missing Request Context Timeouts

**Files:** All handler files

All handlers use `context.Background()` with no timeout:

```go
func (h *Handlers) HandleLogin(s *Session, msg *ClientMessage) {
    ctx := context.Background()  // No timeout!
    // ... database operations that can hang indefinitely
}
```

**Impact:** Slow database queries or network issues can block handler goroutines indefinitely, causing resource exhaustion (DoS).

**Recommendation:** Use `context.WithTimeout()` for all database operations:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

---

### 5. No Rate Limiting

**Files:** All handlers

No rate limiting exists for:
- Login attempts (brute force vulnerability)
- Account creation (spam accounts)
- Message sending (spam messages)
- File uploads (storage exhaustion)
- Invite creation (invite abuse)
- Search operations (enumeration attacks)

**Recommendation:** Implement rate limiting using token bucket or sliding window algorithm per:
- IP address
- User ID (for authenticated endpoints)
- Endpoint

---

### 6. Weak Password Validation

**File:** `auth/auth.go:194-198`

```go
func (a *Auth) ValidatePassword(password string) error {
    if len(password) < a.config.MinPasswordLength {  // Default: 6
        return ErrWeakPassword
    }
    return nil
}
```

**Issues:**
- Only checks minimum length (default 6 characters)
- No complexity requirements (uppercase, numbers, symbols)
- No check against common password lists
- Combined with no rate limiting, enables brute force

**Recommendation:**
- Require minimum 8 characters
- Add complexity requirements or use zxcvbn-style strength checking
- Implement account lockout after failed attempts

---

### 7. Weak Invite Code Generation

**File:** `store/invites.go:27-40`

```go
func GenerateInviteCode() (string, error) {
    b := make([]byte, 5)  // 40 bits of entropy
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    num := uint64(b[0])<<32 | uint64(b[1])<<24 | uint64(b[2])<<16 | uint64(b[3])<<8 | uint64(b[4])
    code := fmt.Sprintf("%010d", num%10000000000)  // 10-digit number
    return code, nil
}
```

**Issues:**
- Only 40 bits of entropy
- 10-digit numbers = 10 billion combinations
- No rate limiting on redemption attempts
- Codes are vulnerable to brute force

**Recommendation:**
- Increase to 128+ bits of entropy
- Use alphanumeric codes for more combinations
- Add rate limiting on code redemption
- Implement code expiration

---

### 8. File Access Control Bypass

**File:** `store/files.go:140-164`

```go
func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
    // TODO: Check if file is in a message in a conversation the user is in
    // For now, allow access (files are typically shared in messages)
    return true, nil  // ALWAYS RETURNS TRUE!
}
```

Any authenticated user can download any file, bypassing intended access control.

**Impact:** Information disclosure - users can access private files from any conversation.

**Recommendation:** Implement proper access control checking if file is in a conversation the user is a member of.

---

## Medium Severity Issues

### 9. Database SSL Disabled by Default

**File:** `config/config.go:190-192`

```go
if c.Database.SSLMode == "" {
    c.Database.SSLMode = "disable"
}
```

**Impact:** Database connections are unencrypted by default, exposing credentials and data on the network.

**Recommendation:** Default to `require` or `verify-full` for production.

---

### 10. Missing X-Forwarded-For Validation

**File:** `server.go:52-56`

```go
if s.config.Server.UseXForwardedFor {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        remoteAddr = xff  // Directly trusts client header
    }
}
```

`X-Forwarded-For` header is trusted without validation. Clients can spoof their IP address.

**Impact:** If rate limiting were implemented, it could be bypassed. Logging accuracy is affected.

**Recommendation:** Only trust X-Forwarded-For from known proxy IPs.

---

### 11. Sensitive Data in Error Messages/Logs

**File:** `handlers_invite.go:85`

```go
go func() {
    if err := h.email.SendInvite(...); err != nil {
        fmt.Printf("Failed to send invite email to %s: %v\n", invite.Email, err)
    }
}()
```

Email addresses and error details logged to stdout.

**Impact:** Information leakage through logs.

**Recommendation:** Use structured logging with appropriate log levels and PII redaction.

---

### 12. Missing Security Headers

No HTTP security headers are set:
- `X-Frame-Options` (clickjacking protection)
- `X-Content-Type-Options` (MIME sniffing protection)
- `Strict-Transport-Security` (HTTPS enforcement)
- `Content-Security-Policy` (XSS protection)

**Recommendation:** Add security headers middleware.

---

### 13. Missing HTTPS Configuration

**File:** `main.go:173-176`

```go
httpServer := &http.Server{
    Addr:    cfg.Server.Listen,
    Handler: mux,
}
```

Server only supports HTTP. No TLS configuration available.

**Impact:** Credentials and messages transmitted in plaintext unless behind HTTPS proxy.

**Recommendation:** Add TLS configuration option or document reverse proxy requirement.

---

### 14. Argon2 Parameters May Be Weak

**File:** `auth/auth.go:74`

```go
hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
// time=1, memory=64MB, threads=4, keyLen=32
```

Using `time=1` iteration is on the lower end. OWASP recommends `time=2` or higher.

**Recommendation:** Increase to `time=2` or `time=3` for better GPU/ASIC resistance.

---

## Low Severity Issues

### 15. User Agent Stored in Database

**File:** `store/users.go:69-76`

User agent strings stored for all sessions. While not critical, this creates privacy concerns.

---

### 16. Potential Race Condition in Message Reactions

**File:** `store/messages.go:139-149`

Non-atomic read-modify-write pattern for edit count:
```sql
head = jsonb_build_object('edit_count', COALESCE((head->>'edit_count')::int, 0) + 1, ...)
```

Under concurrent edits, the count could be inaccurate.

---

## Summary by Severity

| Severity | Count | Key Issues |
|----------|-------|------------|
| Critical | 3 | CORS bypass, hardcoded keys, SQL injection risk |
| High | 5 | No timeouts, no rate limiting, weak passwords, weak invites, file access bypass |
| Medium | 6 | SSL disabled, header spoofing, info disclosure, missing headers |
| Low | 2 | User agent logging, race conditions |

## Recommended Actions

### Immediate (Before Production)
1. Fix CORS configuration to validate origins
2. Remove all hardcoded cryptographic keys
3. Implement file access control
4. Add request timeouts

### High Priority
1. Implement rate limiting for all endpoints
2. Strengthen password validation
3. Improve invite code generation
4. Add database SSL requirement for production

### Medium Priority
1. Add security headers middleware
2. Validate X-Forwarded-For
3. Implement structured logging with PII redaction
4. Add whitelist validation to UpdateMemberSettings
