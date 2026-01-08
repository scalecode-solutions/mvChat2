# mvChat2 Logging Implementation Guide

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 10:12 AM CST. Review the actual code before assuming anything here is still current.

## Overview

This document provides specific implementation details for logging in mvChat2.

---

## 1. Recommended Logger: zerolog

**Why zerolog**:
- Zero allocation JSON logger (fast)
- Structured logging by default
- Leveled logging
- Context support
- Small dependency footprint

**Installation**:
```bash
go get github.com/rs/zerolog
```

---

## 2. Logger Configuration

### 2.1 Logger Setup

```go
package logging

import (
    "os"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func Setup(level string, pretty bool) {
    // Set time format
    zerolog.TimeFieldFormat = time.RFC3339Nano

    // Parse level
    lvl, err := zerolog.ParseLevel(level)
    if err != nil {
        lvl = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(lvl)

    // Pretty print for development
    if pretty {
        log.Logger = log.Output(zerolog.ConsoleWriter{
            Out:        os.Stdout,
            TimeFormat: "15:04:05",
        })
    }

    // Add default fields
    log.Logger = log.With().
        Str("service", "mvchat2").
        Str("version", "0.1.0").
        Logger()
}
```

### 2.2 Config Options

```yaml
logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json or pretty
  output: "stdout"        # stdout, file, or both
  file_path: "/var/log/mvchat2/app.log"
  max_size_mb: 100
  max_backups: 5
  max_age_days: 30
```

---

## 3. Log Entry Examples

### 3.1 Connection Events

```go
// User connected
log.Info().
    Str("event", "connect").
    Str("user_id", userID.String()).
    Str("session_id", sessionID).
    Str("remote_addr", remoteAddr).
    Str("user_agent", userAgent).
    Msg("user connected")

// User disconnected
log.Info().
    Str("event", "disconnect").
    Str("user_id", userID.String()).
    Str("session_id", sessionID).
    Dur("duration", time.Since(connectedAt)).
    Msg("user disconnected")
```

### 3.2 Authentication Events

```go
// Login success
log.Info().
    Str("event", "auth_success").
    Str("user_id", userID.String()).
    Str("scheme", "basic").
    Str("remote_addr", remoteAddr).
    Msg("authentication successful")

// Login failure
log.Warn().
    Str("event", "auth_failure").
    Str("username", username).  // OK to log username, not password
    Str("scheme", "basic").
    Str("remote_addr", remoteAddr).
    Str("reason", "invalid_password").
    Msg("authentication failed")
```

### 3.3 Message Events

```go
// Message sent (don't log content!)
log.Debug().
    Str("event", "message_sent").
    Str("user_id", userID.String()).
    Str("conv_id", convID.String()).
    Int("seq", seq).
    Int("content_size", len(content)).
    Msg("message sent")
```

### 3.4 Error Events

```go
// Database error
log.Error().
    Err(err).
    Str("event", "db_error").
    Str("function", "GetMessages").
    Str("user_id", userID.String()).
    Str("conv_id", convID.String()).
    Str("trace_id", traceID).
    Dur("duration", time.Since(start)).
    Msg("database query failed")

// Recoverable error
log.Warn().
    Err(err).
    Str("event", "redis_reconnect").
    Int("attempt", attempt).
    Dur("backoff", backoff).
    Msg("redis connection lost, reconnecting")
```

### 3.5 Slow Operations

```go
// Slow query warning
if duration > time.Second {
    log.Warn().
        Str("event", "slow_query").
        Str("function", functionName).
        Str("query", queryName).  // NOT the full SQL
        Dur("duration", duration).
        Msg("slow database query")
}
```

---

## 4. Trace ID Implementation

### 4.1 Middleware

```go
func TraceMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceID := r.Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = uuid.New().String()
        }

        // Add to context
        ctx := context.WithValue(r.Context(), "trace_id", traceID)

        // Add to response header
        w.Header().Set("X-Trace-ID", traceID)

        // Add to logger context
        logger := log.With().Str("trace_id", traceID).Logger()
        ctx = logger.WithContext(ctx)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 4.2 WebSocket Trace ID

```go
// In session message handling
func (s *Session) handleMessage(data []byte) {
    var msg ClientMessage
    if err := json.Unmarshal(data, &msg); err != nil {
        return
    }

    // Generate trace ID for this request
    traceID := uuid.New().String()

    // Create logger with context
    logger := log.With().
        Str("trace_id", traceID).
        Str("user_id", s.userID.String()).
        Str("session_id", s.id).
        Str("msg_id", msg.ID).
        Logger()

    // Pass logger through context
    ctx := logger.WithContext(context.Background())

    // Handle message with traced context
    s.handlers.Handle(ctx, s, &msg)
}
```

---

## 5. Request/Response Logging

### 5.1 HTTP Requests

```go
func RequestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer to capture status
        wrapped := &responseWriter{ResponseWriter: w, status: 200}

        next.ServeHTTP(wrapped, r)

        log.Info().
            Str("event", "http_request").
            Str("method", r.Method).
            Str("path", r.URL.Path).
            Int("status", wrapped.status).
            Dur("duration", time.Since(start)).
            Str("remote_addr", r.RemoteAddr).
            Msg("http request")
    })
}
```

### 5.2 WebSocket Messages

```go
// Log incoming message (debug level)
log.Debug().
    Str("event", "ws_recv").
    Str("session_id", s.id).
    Str("msg_type", getMsgType(&msg)).
    Int("size", len(data)).
    Msg("received message")

// Log outgoing message (debug level)
log.Debug().
    Str("event", "ws_send").
    Str("session_id", s.id).
    Str("msg_type", getMsgType(&msg)).
    Int("size", len(data)).
    Msg("sent message")
```

---

## 6. Log Aggregation

### 6.1 Docker Logging

```yaml
# docker-compose.yml
services:
  mvchat:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

### 6.2 Loki Integration (Optional)

```yaml
# promtail config
scrape_configs:
  - job_name: mvchat2
    static_configs:
      - targets:
          - localhost
        labels:
          job: mvchat2
          __path__: /var/log/mvchat2/*.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            event: event
            trace_id: trace_id
      - labels:
          level:
          event:
```

---

## 7. Metrics (Prometheus)

### 7.1 Key Metrics

```go
var (
    // Connections
    connectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "mvchat_connections_total",
        Help: "Total WebSocket connections",
    })
    connectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "mvchat_connections_active",
        Help: "Active WebSocket connections",
    })

    // Messages
    messagesSent = promauto.NewCounter(prometheus.CounterOpts{
        Name: "mvchat_messages_sent_total",
        Help: "Total messages sent",
    })

    // Errors
    errorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "mvchat_errors_total",
        Help: "Total errors by type",
    }, []string{"type"})

    // Latency
    requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "mvchat_request_duration_seconds",
        Help:    "Request duration in seconds",
        Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
    }, []string{"handler"})
)
```

### 7.2 Metrics Endpoint

```go
// Add to main.go
http.Handle("/metrics", promhttp.Handler())
```

---

## 8. Log Retention

### 8.1 Retention Policy

| Log Type | Retention | Reason |
|----------|-----------|--------|
| Access logs | 30 days | Debugging, analytics |
| Error logs | 90 days | Incident investigation |
| Auth logs | 1 year | Security auditing |
| Debug logs | 7 days | Development only |

### 8.2 Log Rotation

```bash
# /etc/logrotate.d/mvchat2
/var/log/mvchat2/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 mvchat mvchat
    postrotate
        systemctl reload mvchat2
    endscript
}
```

---

## 9. Sensitive Data Handling

### 9.1 Never Log

- Passwords
- Auth tokens
- Private keys
- Message content (encrypted or not)
- Full IP addresses in production (consider hashing)

### 9.2 Redaction Helper

```go
func redact(s string) string {
    if len(s) <= 4 {
        return "****"
    }
    return s[:2] + "****" + s[len(s)-2:]
}

// Usage
log.Info().
    Str("token", redact(token)).  // "ab****xy"
    Msg("token used")
```

### 9.3 PII Considerations

For GDPR/privacy compliance:
- Log user IDs, not emails
- Hash IP addresses if needed for analytics
- Provide log deletion capability for user data requests
