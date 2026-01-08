# Immediate Priorities - Critical Fixes Before Production

**Created:** January 8, 2026
**Total Critical Issues:** 17
**Estimated Total Effort:** 5-7 days

This document outlines the critical issues that must be fixed before the application can be considered production-ready. Issues are ordered by risk and dependency.

---

## Priority 1: Security Hardening (CRITICAL)

These issues expose the application to immediate security vulnerabilities.

### 1.1 Remove Hardcoded Cryptographic Keys

**Risk:** Critical - Keys in source code are compromised by default
**Effort:** 2-4 hours
**File:** `config/config.go`

**Current Problem:**
```go
// config/config.go
JWTSecret:      getEnv("JWT_SECRET", "your-256-bit-secret-key-here"),
EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),  // Empty allowed!
```

**Fix:**
```go
func Load() (*Config, error) {
    cfg := &Config{...}

    // Require JWT secret in production
    if cfg.JWTSecret == "" || cfg.JWTSecret == "your-256-bit-secret-key-here" {
        if cfg.Environment == "production" {
            return nil, errors.New("JWT_SECRET must be set in production")
        }
        log.Warn("Using default JWT secret - NOT FOR PRODUCTION")
    }

    // Require encryption key
    if cfg.EncryptionKey == "" {
        return nil, errors.New("ENCRYPTION_KEY is required")
    }
    if len(cfg.EncryptionKey) != 32 {
        return nil, errors.New("ENCRYPTION_KEY must be exactly 32 bytes")
    }

    return cfg, nil
}
```

---

### 1.2 Implement CORS Origin Validation

**Risk:** Critical - Current config allows any origin
**Effort:** 1-2 hours
**File:** `config/config.go`, `handlers.go`

**Current Problem:**
```go
// handlers.go
w.Header().Set("Access-Control-Allow-Origin", "*")  // Allows everything!
```

**Fix:**
```go
// config/config.go
type Config struct {
    AllowedOrigins []string `json:"allowed_origins"`
}

// handlers.go
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            for _, allowed := range allowedOrigins {
                if allowed == "*" || allowed == origin {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    break
                }
            }

            if r.Method == "OPTIONS" {
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
                w.WriteHeader(http.StatusNoContent)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

---

### 1.3 Add Request Timeouts

**Risk:** Critical - Server vulnerable to slowloris attacks
**Effort:** 30 minutes
**File:** `main.go`

**Current Problem:**
```go
// main.go
server := &http.Server{
    Addr:    cfg.ListenAddr,
    Handler: mux,
    // No timeouts configured!
}
```

**Fix:**
```go
server := &http.Server{
    Addr:         cfg.ListenAddr,
    Handler:      mux,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

---

### 1.4 Fix File Access Control Bypass

**Risk:** High - Any authenticated user can access any file
**Effort:** 1-2 hours
**File:** `store/files.go`

**Current Problem:**
```go
func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
    // ...
    // TODO: Check if file is referenced in a message...
    return true, nil  // ALWAYS ALLOWS ACCESS!
}
```

**Fix:** See [11-FILE-DEDUPLICATION.md](11-FILE-DEDUPLICATION.md) for complete implementation with message attachment tracking.

Quick interim fix:
```go
func (db *DB) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
    var uploaderID uuid.UUID
    err := db.pool.QueryRow(ctx, `
        SELECT uploader_id FROM files WHERE id = $1 AND deleted_at IS NULL
    `, fileID).Scan(&uploaderID)

    if errors.Is(err, pgx.ErrNoRows) {
        return false, nil
    }
    if err != nil {
        return false, err
    }

    // Only uploader can access (restrictive but safe)
    return uploaderID == userID, nil
}
```

---

## Priority 2: Database Schema Fixes (CRITICAL)

These issues will cause runtime failures.

### 2.1 Fix Schema/Code Mismatch

**Risk:** Critical - INSERT statements will fail
**Effort:** 2-3 hours
**Files:** `store/schema.sql`, `store/files.go`

**Current Problem:**
- Schema has `user_id`, code uses `uploader_id`
- Schema has `status INT`, code uses `'pending'` string
- Schema requires `hash NOT NULL`, code never provides it

**Fix - Option A (Change Schema):**
```sql
-- Migration
ALTER TABLE files RENAME COLUMN user_id TO uploader_id;
ALTER TABLE files ALTER COLUMN status TYPE VARCHAR(16) USING
    CASE status WHEN 0 THEN 'pending' WHEN 1 THEN 'ready' ELSE 'failed' END;
ALTER TABLE files ALTER COLUMN hash DROP NOT NULL;  -- Until backfill complete
```

**Fix - Option B (Change Code):**
Update all queries in `store/files.go` to use `user_id` and integer status values.

**Recommendation:** Option A - the code naming is more explicit.

---

### 2.2 Add Missing Database Indexes

**Risk:** High - Query performance degrades with scale
**Effort:** 30 minutes
**File:** `store/schema.sql`

```sql
-- Add to schema.sql or create migration
CREATE INDEX idx_members_conv_user ON members(conversation_id, user_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_message_deletions_user ON message_deletions(user_id);
CREATE INDEX idx_messages_created ON messages(created_at DESC);
```

---

## Priority 3: WebSocket Data Race Fixes (CRITICAL)

These issues can cause crashes or data corruption under load.

### 3.1 Add Mutex Protection to Session Fields

**Risk:** Critical - Data race causes undefined behavior
**Effort:** 2-3 hours
**File:** `session.go`

**Current Problem:**
```go
type Session struct {
    userID uuid.UUID  // Written in Login, read everywhere
    user   *store.User
    subs   map[uuid.UUID]*Subscription  // Concurrent access
}
```

**Fix:**
```go
type Session struct {
    mu     sync.RWMutex
    userID uuid.UUID
    user   *store.User
    subs   map[uuid.UUID]*Subscription
}

func (s *Session) GetUserID() uuid.UUID {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.userID
}

func (s *Session) SetUserID(id uuid.UUID) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.userID = id
}

func (s *Session) Subscribe(convID uuid.UUID, sub *Subscription) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.subs[convID] = sub
}
```

---

### 3.2 Fix Send() Race Condition

**Risk:** High - Concurrent writes to WebSocket can corrupt messages
**Effort:** 1 hour
**File:** `session.go`

**Current Problem:**
```go
func (s *Session) Send(msg interface{}) {
    s.conn.WriteJSON(msg)  // Not thread-safe!
}
```

**Fix:**
```go
type Session struct {
    writeMu sync.Mutex  // Separate mutex for writes
    // ...
}

func (s *Session) Send(msg interface{}) error {
    s.writeMu.Lock()
    defer s.writeMu.Unlock()
    return s.conn.WriteJSON(msg)
}
```

---

## Priority 4: Error Handling (HIGH)

### 4.1 Fix Silently Ignored Errors in Broadcasts

**Risk:** High - Failed broadcasts go unnoticed
**Effort:** 2-3 hours
**Files:** `hub.go`, `handlers.go`

**Current Problem:**
```go
// hub.go
for _, sess := range h.sessions {
    sess.Send(msg)  // Error ignored!
}
```

**Fix:**
```go
func (h *Hub) broadcast(msg interface{}, sessions []*Session) {
    for _, sess := range sessions {
        if err := sess.Send(msg); err != nil {
            log.Printf("Failed to send to session %s: %v", sess.ID(), err)
            // Optionally: mark session for cleanup
            h.markDisconnected(sess)
        }
    }
}
```

---

## Priority 5: Rate Limiting (HIGH)

### 5.1 Add Basic Rate Limiting

**Risk:** High - Server vulnerable to abuse
**Effort:** 2-3 hours
**Files:** New `ratelimit/` package, `handlers.go`

**Implementation:**
```go
// ratelimit/ratelimit.go
package ratelimit

import (
    "sync"
    "time"
)

type Limiter struct {
    mu       sync.Mutex
    requests map[string][]time.Time
    limit    int
    window   time.Duration
}

func New(limit int, window time.Duration) *Limiter {
    return &Limiter{
        requests: make(map[string][]time.Time),
        limit:    limit,
        window:   window,
    }
}

func (l *Limiter) Allow(key string) bool {
    l.mu.Lock()
    defer l.mu.Unlock()

    now := time.Now()
    windowStart := now.Add(-l.window)

    // Filter old requests
    valid := l.requests[key][:0]
    for _, t := range l.requests[key] {
        if t.After(windowStart) {
            valid = append(valid, t)
        }
    }
    l.requests[key] = valid

    if len(valid) >= l.limit {
        return false
    }

    l.requests[key] = append(l.requests[key], now)
    return true
}
```

**Usage:**
```go
// main.go
authLimiter := ratelimit.New(5, time.Minute)    // 5 login attempts/minute
uploadLimiter := ratelimit.New(10, time.Minute) // 10 uploads/minute
wsLimiter := ratelimit.New(100, time.Second)    // 100 messages/second

// handlers.go
func (h *Handlers) handleLogin(w http.ResponseWriter, r *http.Request) {
    ip := r.RemoteAddr
    if !h.authLimiter.Allow(ip) {
        http.Error(w, "rate limited", http.StatusTooManyRequests)
        return
    }
    // ... rest of handler
}
```

---

## Implementation Order

| # | Task | Effort | Blocks |
|---|------|--------|--------|
| 1 | Remove hardcoded keys | 2-4h | - |
| 2 | Add request timeouts | 30m | - |
| 3 | Fix CORS | 1-2h | - |
| 4 | Fix schema mismatch | 2-3h | File dedup |
| 5 | Add Session mutex | 2-3h | - |
| 6 | Fix Send() race | 1h | Session mutex |
| 7 | Fix file access control | 1-2h | Schema fix |
| 8 | Add missing indexes | 30m | - |
| 9 | Fix broadcast errors | 2-3h | - |
| 10 | Add rate limiting | 2-3h | - |

**Total Estimated Effort:** 15-22 hours (2-3 days focused work)

---

## Verification Checklist

After implementing fixes:

- [ ] `JWT_SECRET` and `ENCRYPTION_KEY` required in production
- [ ] CORS only allows configured origins
- [ ] Server has read/write/idle timeouts
- [ ] Schema matches code (uploader_id, string status)
- [ ] File access restricted to uploader (or conversation members)
- [ ] No data races detected by `go run -race`
- [ ] Broadcast errors logged
- [ ] Rate limiting active on auth and upload endpoints
- [ ] All indexes created

---

## Next Steps After Immediate Fixes

1. **File Deduplication** - Implement full solution from [11-FILE-DEDUPLICATION.md](11-FILE-DEDUPLICATION.md)
2. **Test Coverage** - Add tests for auth, crypto, store packages
3. **Code Deduplication** - Refactor repeated patterns
4. **Documentation** - Add godoc to exported types
