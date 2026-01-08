# mvChat2 Error Handling & Logging Strategy

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 10:12 AM CST. Review the actual code before assuming anything here is still current.

## Overview

This document outlines the error handling, logging, and self-recovery strategies for mvChat2. The goal is to ensure:

1. **Observability**: Know exactly what happened, when, why, and where
2. **Recoverability**: Self-heal from transient failures when possible
3. **Graceful degradation**: Continue operating with reduced functionality rather than crashing
4. **User experience**: Provide meaningful error messages to clients

---

## 1. Error Categories

### 1.1 Client Errors (4xx)

Errors caused by invalid client requests. These are NOT logged as errors (they're expected).

| Code | Name | Description | Self-Recoverable |
|------|------|-------------|------------------|
| 400 | Bad Request | Malformed request, missing fields | No - client must fix |
| 401 | Unauthorized | Missing or invalid auth | Yes - client re-authenticates |
| 403 | Forbidden | Valid auth but not permitted | No - client lacks permission |
| 404 | Not Found | Resource doesn't exist | No - resource missing |
| 409 | Conflict | Duplicate resource (e.g., username taken) | No - client must choose different |
| 410 | Gone | Resource was deleted | No - resource removed |

### 1.2 Server Errors (5xx)

Errors caused by server-side issues. These ARE logged as errors.

| Code | Name | Description | Self-Recoverable |
|------|------|-------------|------------------|
| 500 | Internal Error | Unexpected server error | Maybe - depends on cause |
| 502 | Bad Gateway | Upstream service failed (Redis, DB) | Yes - retry with backoff |
| 503 | Service Unavailable | Server overloaded or in maintenance | Yes - retry later |
| 504 | Gateway Timeout | Upstream service timeout | Yes - retry with backoff |

### 1.3 Transient vs Permanent Errors

**Transient** (self-recoverable):
- Database connection dropped
- Redis connection lost
- Network timeout
- Rate limiting

**Permanent** (require intervention):
- Invalid configuration
- Corrupted data
- Authentication failures
- Permission denied

---

## 2. Logging Strategy

### 2.1 Log Levels

| Level | When to Use | Example |
|-------|-------------|---------|
| **DEBUG** | Detailed flow tracing (dev only) | "Parsing message from user X" |
| **INFO** | Normal operations | "User X connected", "Message sent" |
| **WARN** | Recoverable issues | "Redis reconnecting", "Slow query" |
| **ERROR** | Failures requiring attention | "Database error", "Encryption failed" |
| **FATAL** | Unrecoverable, server must stop | "Config invalid", "Port in use" |

### 2.2 Structured Logging Format

All logs should be structured JSON for easy parsing:

```json
{
  "ts": "2026-01-08T10:12:00.000Z",
  "level": "error",
  "msg": "database query failed",
  "error": "connection refused",
  "component": "store",
  "function": "GetMessages",
  "user_id": "uuid",
  "conv_id": "uuid",
  "duration_ms": 5000,
  "retry_count": 3,
  "trace_id": "abc123"
}
```

### 2.3 What to Log

**Always log:**
- User authentication (success/failure)
- Connection open/close
- Errors with full context
- Slow operations (>1s)
- Security events (failed auth, permission denied)
- Admin actions

**Never log:**
- Passwords or secrets
- Full message content (encrypted anyway)
- PII beyond user IDs
- Auth tokens

### 2.4 Correlation IDs

Every request should have a trace ID that flows through:
1. Client request → Session → Handler → Store → Response
2. Allows tracing a single request across all log entries
3. SDK should generate and send `X-Trace-ID` header

---

## 3. Self-Recovery Strategies

### 3.1 Database Connection

**Problem**: PostgreSQL connection lost

**Strategy**:
```
1. Detect connection error
2. Log WARN with error details
3. Retry with exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s (max)
4. After 5 retries, log ERROR and return 503 to client
5. Continue retrying in background
6. When connection restored, log INFO
```

**Implementation**:
- Use connection pool with health checks
- pgx has built-in reconnection
- Add circuit breaker to prevent thundering herd

### 3.2 Redis Connection

**Problem**: Redis connection lost

**Strategy**:
```
1. Detect connection error
2. Log WARN
3. Fall back to local-only mode (no pub/sub across instances)
4. Retry connection with backoff
5. When restored, re-subscribe to channels
6. Log INFO when recovered
```

**Graceful degradation**:
- Presence updates only work on local instance
- Messages still delivered to local sessions
- Cross-instance delivery resumes when Redis returns

### 3.3 WebSocket Connection (Client-Side)

**Problem**: WebSocket disconnected

**Strategy** (SDK handles this):
```
1. Detect disconnect
2. Emit 'disconnect' event
3. Retry with exponential backoff: 1s, 1.5s, 2.25s... (max 30s)
4. On reconnect:
   a. Re-authenticate with stored token
   b. Re-subscribe to conversations
   c. Fetch missed messages (history recovery)
5. Emit 'reconnect' event
```

### 3.4 Message Send Failure

**Problem**: Client sends message but server returns error

**Strategy** (SDK handles this):
```
1. Mark message as "pending" in local state
2. If 5xx error: retry up to 3 times with backoff
3. If 4xx error: mark as "failed", show error to user
4. If network error: queue for retry when reconnected
5. Emit 'messageFailed' event with error details
```

### 3.5 Encryption Failure

**Problem**: Message encryption/decryption fails

**Strategy**:
```
1. Log ERROR with details (NOT the plaintext)
2. For encryption failure: return 500, don't send message
3. For decryption failure: return message with content: null, flag: "decrypt_failed"
4. Client shows "Message could not be decrypted"
```

### 3.6 File Upload Failure

**Problem**: File upload fails mid-stream

**Strategy**:
```
1. Support resumable uploads (store partial uploads)
2. Client can resume from last successful chunk
3. Partial uploads cleaned up after 24 hours
4. Return progress info so client knows where to resume
```

---

## 4. Error Response Format

All errors returned to clients should follow this format:

```json
{
  "ctrl": {
    "id": "request-id",
    "code": 500,
    "text": "database error",
    "ts": "2026-01-08T10:12:00.000Z"
  }
}
```

### 4.1 Error Text Guidelines

**DO**:
- Be specific enough to debug: "invalid conv id"
- Be consistent: always use same text for same error
- Be actionable: "authentication required" (client knows to re-auth)

**DON'T**:
- Expose internal details: "pgx: connection refused to 10.0.0.5:5432"
- Be vague: "error occurred"
- Include stack traces in response (log them instead)

### 4.2 Error Codes to Text Mapping

```go
var errorTexts = map[int]string{
    400: "bad request",
    401: "authentication required",
    403: "forbidden",
    404: "not found",
    409: "conflict",
    410: "gone",
    500: "internal error",
    502: "service unavailable",
    503: "service unavailable",
    504: "timeout",
}
```

---

## 5. Health Checks

### 5.1 Liveness Probe

**Endpoint**: `GET /health/live`

**Returns**: 200 if server is running, 503 if not

**Checks**:
- Server process is alive
- Can accept connections

### 5.2 Readiness Probe

**Endpoint**: `GET /health/ready`

**Returns**: 200 if ready to serve, 503 if not

**Checks**:
- Database connection healthy
- Redis connection healthy (or gracefully degraded)
- Not in maintenance mode

### 5.3 Detailed Health

**Endpoint**: `GET /health` (admin only)

**Returns**:
```json
{
  "status": "healthy",
  "checks": {
    "database": { "status": "up", "latency_ms": 2 },
    "redis": { "status": "up", "latency_ms": 1 },
    "disk": { "status": "up", "free_gb": 50 }
  },
  "uptime_seconds": 86400,
  "version": "0.1.0",
  "connections": 150
}
```

---

## 6. Alerting Thresholds

### 6.1 Immediate Alerts (PagerDuty/SMS)

- Server crash / restart loop
- Database connection failed for >1 minute
- Error rate >10% for >2 minutes
- Disk space <10%

### 6.2 Warning Alerts (Slack/Email)

- Redis connection lost (degraded mode)
- Error rate >1% for >5 minutes
- Slow queries >5s
- Memory usage >80%
- Connection count approaching limit

### 6.3 Info Alerts (Dashboard only)

- Deployment completed
- Maintenance mode entered/exited
- Config reload

---

## 7. Implementation Checklist

### 7.1 Backend

- [ ] Add structured logger (zerolog or zap)
- [ ] Add trace ID middleware
- [ ] Add request/response logging middleware
- [ ] Implement circuit breaker for DB/Redis
- [ ] Add retry logic with exponential backoff
- [ ] Add health check endpoints
- [ ] Add metrics endpoint (Prometheus)
- [ ] Standardize error responses
- [ ] Add slow query logging

### 7.2 SDK

- [ ] Add trace ID generation
- [ ] Implement message queue for offline sends
- [ ] Add retry logic for transient failures
- [ ] Emit detailed error events
- [ ] Add connection state machine
- [ ] Implement history recovery on reconnect

### 7.3 Monitoring

- [ ] Set up log aggregation (Loki, CloudWatch, etc.)
- [ ] Set up metrics collection (Prometheus)
- [ ] Create dashboards (Grafana)
- [ ] Configure alerting rules
- [ ] Set up error tracking (Sentry optional)

---

## 8. Error Handling Patterns

### 8.1 Handler Pattern

```go
func (h *Handlers) HandleSomething(s *Session, msg *ClientMessage) {
    ctx := context.Background()
    
    // 1. Validate input
    if msg.Data == nil {
        s.Send(CtrlError(msg.ID, CodeBadRequest, "missing data"))
        return
    }
    
    // 2. Call store with error handling
    result, err := h.db.DoSomething(ctx, ...)
    if err != nil {
        // Log with context
        h.logger.Error("DoSomething failed",
            "error", err,
            "user_id", s.userID,
            "trace_id", msg.TraceID,
        )
        
        // Return generic error to client
        s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
        return
    }
    
    // 3. Success
    s.Send(CtrlSuccess(msg.ID, CodeOK, result))
}
```

### 8.2 Retry Pattern

```go
func withRetry(ctx context.Context, maxRetries int, fn func() error) error {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err != nil {
            lastErr = err
            if !isRetryable(err) {
                return err
            }
            delay := time.Duration(1<<i) * time.Second // 1s, 2s, 4s...
            if delay > 30*time.Second {
                delay = 30 * time.Second
            }
            time.Sleep(delay)
            continue
        }
        return nil
    }
    return lastErr
}

func isRetryable(err error) bool {
    // Network errors, timeouts, connection refused
    // NOT: auth errors, not found, bad request
}
```

### 8.3 Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    failures    int
    lastFailure time.Time
    state       string // "closed", "open", "half-open"
    threshold   int
    timeout     time.Duration
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if cb.state == "open" {
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = "half-open"
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = "open"
        }
        return err
    }
    
    cb.failures = 0
    cb.state = "closed"
    return nil
}
```

---

## 9. Security Considerations

### 9.1 Error Information Leakage

**Never expose**:
- Internal IP addresses
- Database schema details
- Stack traces
- File paths
- Configuration values

**Safe to expose**:
- Error codes
- Generic error messages
- Request IDs (for support)

### 9.2 Rate Limiting Errors

When rate limited, return:
```json
{
  "ctrl": {
    "code": 429,
    "text": "rate limit exceeded",
    "params": {
      "retry_after": 60
    }
  }
}
```

### 9.3 Authentication Errors

Don't differentiate between:
- User not found
- Password incorrect

Both should return: `401 - authentication failed`

This prevents user enumeration attacks.
