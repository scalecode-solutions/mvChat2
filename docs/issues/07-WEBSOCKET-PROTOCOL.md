# WebSocket and Protocol Issues

This document catalogs issues with WebSocket handling, protocol implementation, and real-time communication.

## Critical Issues

### 1. Data Race in Session Fields

**Files:** `session.go:33-38`, `hub.go:126-129,276-278`

Session fields are accessed without synchronization from multiple goroutines:

```go
// session.go - Fields without mutex protection
type Session struct {
    userID     uuid.UUID  // Written in handleHi, read in hub operations
    userAgent  string     // Written in handleHi
    deviceID   string     // Written in handleHi
    lang       string     // Written in handleHi
    ver        string     // Written in handleHi
}

// Called from readPump goroutine
func (s *Session) handleHi(msg *ClientMessage) {
    s.ver = hi.Version      // RACE CONDITION
    s.userAgent = hi.UserAgent
    s.deviceID = hi.DeviceID
    s.lang = hi.Lang
}

// Called from hub goroutine
func (h *Hub) AuthenticateSession(sess *Session, userID uuid.UUID) {
    sess.userID = userID  // RACE CONDITION with reads in other goroutines
}
```

**Impact:** Data corruption, undefined behavior under concurrent access.

**Recommendation:** Add mutex to Session or use atomic operations:
```go
type Session struct {
    mu        sync.RWMutex
    userID    uuid.UUID
    // ...
}

func (s *Session) SetUserID(id uuid.UUID) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.userID = id
}
```

---

### 2. No Rate Limiting on WebSocket Messages

**File:** `session.go:119-135`

```go
// readPump accepts unlimited messages
for {
    _, message, err := s.conn.ReadMessage()  // No rate limit!
    if err != nil {
        break
    }
    var msg ClientMessage
    json.Unmarshal(message, &msg)
    s.dispatch(&msg)  // No throttling
}
```

**Impact:** DoS vulnerability - clients can flood the server with messages. Especially dangerous for:
- Typing indicators (spammable)
- Read receipts (spammable)
- Reactions (spammable)
- Login attempts (brute force)

**Recommendation:** Implement token bucket rate limiting:
```go
type Session struct {
    rateLimiter *rate.Limiter  // golang.org/x/time/rate
}

func (s *Session) readPump() {
    for {
        if !s.rateLimiter.Allow() {
            s.Send(CtrlError("", CodeTooManyRequests, "rate limit exceeded"))
            continue
        }
        // ... process message
    }
}
```

---

### 3. Race Condition in Send() - TOCTOU

**File:** `session.go:77-86`

```go
func (s *Session) Send(msg *ServerMessage) {
    if atomic.LoadInt32(&s.closing) == 1 {  // Check
        return
    }
    select {
    case s.send <- msg:  // Use - session could close between check and send!
    default:
        s.Close()
    }
}
```

**Impact:** Potential panic if session closes after check but before send.

**Recommendation:** Use mutex or accept the race (channel operations are safe, but close detection may be late):
```go
func (s *Session) Send(msg *ServerMessage) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.closing {
        return
    }
    select {
    case s.send <- msg:
    default:
        go s.Close()  // Close asynchronously to avoid deadlock
    }
}
```

---

## High Severity Issues

### 4. Message ID Not Validated

**File:** `types.go:12`, `session.go:171-207`

```go
type ClientMessage struct {
    ID string `json:"id,omitempty"`  // Optional!
}

// Responses use the ID without validation
s.Send(CtrlSuccess(msg.ID, CodeOK, ...))  // May be empty
```

**Impact:** Clients cannot correlate responses if ID is missing. Should require ID for stateful operations.

**Recommendation:** Validate ID is present for operations that return responses:
```go
if msg.ID == "" {
    s.Send(CtrlError("", CodeBadRequest, "message id required"))
    return
}
```

---

### 5. Multiple Message Types Can Be Set Simultaneously

**File:** `session.go:171-207`

```go
func (s *Session) dispatch(msg *ClientMessage) {
    switch {
    case msg.Hi != nil:
        s.handleHi(msg)  // Only first match processed
    case msg.Login != nil:
        // Never reached if both Hi and Login are set
    }
}
```

**Impact:** Confusing protocol behavior - only first type is processed.

**Recommendation:** Validate exactly one message type:
```go
func (s *Session) dispatch(msg *ClientMessage) {
    count := 0
    if msg.Hi != nil { count++ }
    if msg.Login != nil { count++ }
    // ... count all types

    if count != 1 {
        s.Send(CtrlError(msg.ID, CodeBadRequest, "exactly one message type required"))
        return
    }
    // ... dispatch
}
```

---

### 6. Connection Leak on Channel Full

**File:** `session.go:105-109`, `hub.go:41-42,115-116`

```go
// hub.go - Buffered channel
unregister: make(chan *Session, 256),

// session.go - Non-blocking send
defer func() {
    s.hub.Unregister(s)  // Can block if channel full
    s.Close()
}()

// hub.go - Sends to channel
func (h *Hub) Unregister(sess *Session) {
    h.unregister <- sess  // Blocks if buffer full!
}
```

**Impact:** If hub is blocked or slow, unregister operations hang, leaving zombie sessions.

**Recommendation:** Use non-blocking send or select with timeout:
```go
func (h *Hub) Unregister(sess *Session) {
    select {
    case h.unregister <- sess:
    default:
        // Log warning, channel full
        go func() { h.unregister <- sess }()  // Async retry
    }
}
```

---

### 7. Aggressive Close on Buffer Full

**File:** `session.go:76-86`

```go
func (s *Session) Send(msg *ServerMessage) {
    select {
    case s.send <- msg:
    default:
        s.Close()  // Immediate disconnect on buffer full!
    }
}
```

**Impact:** Slow clients disconnected immediately without warning.

**Recommendation:** Implement backpressure or warning:
```go
func (s *Session) Send(msg *ServerMessage) {
    select {
    case s.send <- msg:
    case <-time.After(5 * time.Second):
        // Log warning
        s.Close()
    }
}
```

---

## Medium Severity Issues

### 8. Missing Authentication Check in dispatch()

**File:** `session.go:171-207`

Some handlers check authentication, others may not:

```go
func (s *Session) dispatch(msg *ClientMessage) {
    switch {
    case msg.Hi != nil:
        s.handleHi(msg)     // No auth required (correct)
    case msg.Login != nil:
        s.handleLogin(msg)  // No auth required (correct)
    case msg.Search != nil:
        s.handleSearch(msg) // Auth checked in handler
    case msg.Send != nil:
        s.handleSend(msg)   // Auth checked in handler
    }
}
```

**Risk:** Inconsistent protection if handler forgets to check auth.

**Recommendation:** Add auth check in dispatch for protected operations:
```go
case msg.Send != nil:
    if !s.IsAuthenticated() {
        s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
        return
    }
    s.handleSend(msg)
```

---

### 9. Error Response ID Lost on Malformed JSON

**File:** `session.go:129-133`

```go
var msg ClientMessage
if err := json.Unmarshal(message, &msg); err != nil {
    s.Send(CtrlError("", CodeBadRequest, "malformed message"))  // No ID!
    continue
}
```

**Impact:** Client cannot correlate error to request.

**Recommendation:** Try to extract ID even from malformed JSON:
```go
var partial struct { ID string `json:"id"` }
json.Unmarshal(message, &partial)  // May fail, that's ok
s.Send(CtrlError(partial.ID, CodeBadRequest, "malformed message"))
```

---

### 10. Hub Lock Released Too Early

**File:** `hub.go:219-252`

```go
func (h *Hub) SendToUsers(userIDs []uuid.UUID, msg *ServerMessage, skipSession string) {
    h.mu.RLock()
    localUsers := make(map[uuid.UUID]bool)
    for _, userID := range userIDs {
        sessions := h.userSessions[userID]
        for _, sess := range sessions {
            sess.Send(msg)  // Uses session pointer
        }
    }
    h.mu.RUnlock()  // Lock released

    // Still using data after unlock
    if h.redis != nil {
        for _, userID := range userIDs {
            if !localUsers[userID] {
                // ...
            }
        }
    }
}
```

**Impact:** Session pointers accessed after lock released; map iteration continues after unlock.

**Recommendation:** Keep lock until done with shared data:
```go
func (h *Hub) SendToUsers(...) {
    h.mu.RLock()
    // ... gather local users
    localUsersCopy := make(map[uuid.UUID]bool)
    for k, v := range localUsers {
        localUsersCopy[k] = v
    }
    h.mu.RUnlock()

    // Use copy for Redis operations
}
```

---

### 11. Goroutine Spawning Without Lifecycle Management

**Files:** `hub.go:153,284`, `presence.go:176-186`

```go
// Fire-and-forget goroutines
go h.presence.UserOffline(sess.userID)  // hub.go:153
go h.presence.UserOnline(userID)        // hub.go:284

// No cancellation mechanism
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.refreshOnlineUsers(ctx)  // What if this panics?
        }
    }
}()
```

**Impact:** Goroutines can hang or panic without cleanup. No way to cancel UserOnline/UserOffline.

**Recommendation:** Add error handling and context support:
```go
func (p *PresenceManager) UserOnline(ctx context.Context, userID uuid.UUID) error {
    // Use context for cancellation
    // Return error for caller to handle
}
```

---

### 12. lastAction Never Used

**File:** `session.go:127`

```go
atomic.StoreInt64(&s.lastAction, time.Now().UnixNano())  // Stored but never read
```

**Impact:** Idle timeout feature not implemented.

**Recommendation:** Either:
- Implement idle timeout using lastAction
- Remove unused field

---

## Low Severity Issues

### 13. Silent Errors in WriteJSON

**File:** `session.go:157-159`

```go
if err := s.conn.WriteJSON(msg); err != nil {
    return  // Silent failure - no logging
}
```

**Recommendation:** Add error logging for debugging.

---

### 14. Redis Errors Silently Ignored

**Files:** `hub.go`, `presence.go`

```go
// hub.go:88-89
if err := json.Unmarshal(msg.Payload, &payload); err != nil {
    return  // Silent
}

// presence.go:33-35
p.redis.SetOnline(ctx, userID.String())  // No error check
```

**Recommendation:** Log Redis errors for monitoring.

---

### 15. Presence Race Condition

**File:** `presence.go:28-51`

If user connects on multiple nodes simultaneously:
```go
// Node 1: UserOnline fires, checks hub.IsOnline -> false
// Node 2: UserOnline fires, checks hub.IsOnline -> false
// Both broadcast "online" notification
```

**Recommendation:** Use Redis SETNX or distributed locking for presence state changes.

---

## Summary

| Issue | Severity | Category |
|-------|----------|----------|
| Session field data races | Critical | Concurrency |
| No rate limiting | Critical | Security/DoS |
| Send() TOCTOU race | Critical | Concurrency |
| Missing message ID validation | High | Protocol |
| Multiple message types allowed | High | Protocol |
| Connection leak on channel full | High | Resource |
| Aggressive close on buffer full | High | UX |
| Missing auth check in dispatch | Medium | Security |
| Error ID lost on bad JSON | Medium | Protocol |
| Hub lock released early | Medium | Concurrency |
| Unmanaged goroutines | Medium | Resource |
| lastAction unused | Low | Dead code |
| Silent WriteJSON errors | Low | Observability |
| Redis errors ignored | Low | Observability |
| Presence race condition | Low | Consistency |

## Recommended Actions

### Immediate
1. Add rate limiting to prevent DoS
2. Add mutex protection to Session fields
3. Fix Send() race condition

### High Priority
1. Validate message ID is present for stateful operations
2. Validate exactly one message type per request
3. Implement non-blocking unregister

### Medium Priority
1. Add auth check in dispatch for protected operations
2. Keep hub lock until done with shared data
3. Add lifecycle management to goroutines
4. Implement idle timeout using lastAction

### Low Priority
1. Add error logging for WriteJSON failures
2. Log Redis errors for monitoring
3. Consider distributed locking for presence
