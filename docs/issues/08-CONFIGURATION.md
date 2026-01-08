# Configuration and Deployment Issues

This document catalogs issues with configuration management, environment handling, and deployment setup.

## Critical Issues

### 1. Hardcoded Cryptographic Keys in Default Configuration

**Files:** `mvchat2.yaml`, `Dockerfile`

All cryptographic keys have hardcoded defaults:

```yaml
# mvchat2.yaml
uid_key: ${UID_KEY:la6YsO+bNX/+XIkOqc5Svw==}
encryption_key: ${ENCRYPTION_KEY:k8Jz9mN2pQ4rT6vX8yB1dF3hK5nP7sU0wA2cE4gI6jL=}
api_key_salt: ${API_KEY_SALT:T713/rYYgW7g4m3vG6zGRh7+FM1t0T8j13koXScOAj4=}
key: ${TOKEN_KEY:wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=}
```

**Impact:**
- All deployments using defaults share same encryption keys
- All JWT tokens can be forged
- All encrypted messages can be decrypted

**Recommendation:**
1. Remove all default values from YAML
2. Add startup validation that rejects empty/default keys
3. Document key generation process

---

### 2. Encryption Key Can Be Empty

**Files:** `Dockerfile:42`, `mvchat2.yaml:23`, `config/config.go`

```dockerfile
# Dockerfile
ENV ENCRYPTION_KEY=  # Empty!
```

```yaml
# mvchat2.yaml
encryption_key: ${ENCRYPTION_KEY:k8Jz9mN2pQ4rT6vX8yB1dF3hK5nP7sU0wA2cE4gI6jL=}
```

```go
// config/config.go - validate() does NOT check encryption_key
func (c *Config) validate() error {
    if c.Auth.APIKeySalt == "" {
        return fmt.Errorf("auth.api_key_salt is required")
    }
    if c.Auth.Token.Key == "" {
        return fmt.Errorf("auth.token.key is required")
    }
    // MISSING: encryption_key validation!
    return nil
}
```

**Impact:** Empty encryption key causes runtime crash when processing messages.

**Recommendation:** Add to validate():
```go
if c.Database.EncryptionKey == "" {
    return fmt.Errorf("database.encryption_key is required")
}
```

---

### 3. Inconsistent Database Credentials

**Files:** `docker-compose.yml`, `Dockerfile`

```yaml
# docker-compose.yml
POSTGRES_USER: mvchat2
POSTGRES_PASSWORD: mvchat2_dev
DB_USER: mvchat2
DB_PASSWORD: mvchat2_dev
```

```dockerfile
# Dockerfile
ENV DB_USER=postgres
ENV DB_PASSWORD=
```

**Impact:** Standalone Docker image uses different credentials than docker-compose setup.

**Recommendation:**
- Remove all credential defaults from Dockerfile
- Require environment variables at runtime
- Document required environment variables

---

## High Severity Issues

### 4. Database SSL Disabled by Default

**File:** `config/config.go:190-192`

```go
if c.Database.SSLMode == "" {
    c.Database.SSLMode = "disable"
}
```

**Impact:** Database connections unencrypted by default.

**Recommendation:** Default to `require` and document SSL configuration.

---

### 5. WebSocket Buffer Sizes Not Configurable

**File:** `session.go:13-24`

```go
const (
    writeWait      = 10 * time.Second
    pongWait       = 60 * time.Second
    pingPeriod     = (pongWait * 9) / 10
    maxMessageSize = 64 * 1024  // 64KB - conflicts with config!
    sendBufferSize = 128
)
```

**Issues:**
1. Cannot tune for different network conditions
2. `maxMessageSize = 64KB` conflicts with `config.Limits.MaxMessageSize = 128KB`

**Recommendation:** Move to configuration:
```yaml
websocket:
  write_wait: 10s
  pong_wait: 60s
  max_message_size: 65536
  send_buffer_size: 128
```

---

### 6. Missing Request Context Timeouts

**Files:** All handlers

All handlers use `context.Background()` without timeout:

```go
func (h *Handlers) HandleLogin(s *Session, msg *ClientMessage) {
    ctx := context.Background()  // No timeout!
}
```

**Impact:** Slow database queries can hang indefinitely.

**Recommendation:** Add configurable request timeout:
```yaml
server:
  request_timeout: 30s
```

```go
ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.RequestTimeout)
defer cancel()
```

---

### 7. No Logging Configuration

**File:** `config/config.go`

No configuration exists for:
- Log level (debug/info/warn/error)
- Log format (json/text)
- Log output (stdout/file)

**Recommendation:** Add logging configuration:
```yaml
logging:
  level: info
  format: json
  output: stdout
```

---

### 8. Thumbnail Dimensions Hardcoded

**File:** `main.go:157-159`

```go
ThumbWidth:    256,
ThumbHeight:   256,
ThumbQuality:  80,
```

**Recommendation:** Add to configuration:
```yaml
media:
  thumbnail:
    width: 256
    height: 256
    quality: 80
```

---

## Medium Severity Issues

### 9. Health Check Doesn't Verify Dependencies

**File:** `Dockerfile:48-49`

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -q --spider http://localhost:6060/health || exit 1
```

Current health endpoint only checks sessions. Doesn't verify:
- Database connectivity
- Redis connectivity (if enabled)

**Recommendation:** Implement comprehensive health check:
```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{}

    // Database check
    if err := db.Ping(ctx); err != nil {
        checks["database"] = err.Error()
    } else {
        checks["database"] = "ok"
    }

    // Redis check (if enabled)
    if redis != nil {
        if err := redis.Ping(ctx); err != nil {
            checks["redis"] = err.Error()
        } else {
            checks["redis"] = "ok"
        }
    }

    json.NewEncoder(w).Encode(checks)
}
```

---

### 10. No Graceful Shutdown Timeout

**File:** `main.go:195`

```go
httpServer.Close()  // Immediate close, no timeout
```

**Recommendation:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
httpServer.Shutdown(ctx)
```

---

### 11. Argon2 Parameters Hardcoded

**File:** `auth/auth.go:74`

```go
hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
// time=1, memory=64MB, threads=4, keyLen=32
```

**Recommendation:** Make configurable:
```yaml
auth:
  argon2:
    time: 2
    memory: 65536  # 64MB
    threads: 4
    key_length: 32
```

---

### 12. Debug Endpoint Exposed Without Protection

**File:** `mvchat2.yaml:67`, `config/config.go:96-98`

```yaml
debug:
  expvar_path: "/debug/vars"
```

**Impact:** Runtime metrics exposed to all requests.

**Recommendation:** Add authentication or disable in production.

---

### 13. Hub Channel Buffer Sizes Hardcoded

**File:** `hub.go:41-42`

```go
register:     make(chan *Session, 256),
unregister:   make(chan *Session, 256),
```

**Recommendation:** Make configurable for high-concurrency deployments.

---

## Low Severity Issues

### 14. Upload Directory Uses Relative Path

**File:** `config/config.go:224-226`

```go
if c.Media.UploadDir == "" {
    c.Media.UploadDir = "./uploads"
}
```

**Impact:** Relative paths can break in containerized environments.

**Recommendation:** Default to absolute path or require configuration.

---

### 15. Email Link Hardcoded

**File:** `email/email.go:49`

```go
"Link": "https://chat.mvchat.app",
```

**Recommendation:** Add to configuration:
```yaml
email:
  app_link: "https://chat.mvchat.app"
```

---

### 16. No Validation of Numeric Ranges

**File:** `config/config.go`

No validation that:
- `MinLoginLength > 0`
- `MinPasswordLength > 0`
- `MaxMessageSize > 0`
- Connection pool sizes are reasonable

**Recommendation:** Add range validation in validate().

---

### 17. No Base64 Key Validation in Config

**File:** `config/config.go`

Base64 key validation happens in main() after config validation:

```go
// main.go
tokenKey, err := base64.StdEncoding.DecodeString(cfg.Auth.Token.Key)
if err != nil {
    os.Exit(1)
}
```

**Recommendation:** Validate base64 encoding in config validate().

---

### 18. Config File Not Injected at Runtime in Docker

**File:** `Dockerfile:32`

```dockerfile
COPY --from=builder /build/mvchat2.yaml .
```

**Impact:** Cannot override config without rebuilding image.

**Recommendation:** Mount config at runtime or use environment variables for all settings.

---

## Summary

| Issue | Severity | Category |
|-------|----------|----------|
| Hardcoded crypto keys | Critical | Security |
| Empty encryption key allowed | Critical | Security |
| Inconsistent DB credentials | Critical | Deployment |
| SSL disabled by default | High | Security |
| WebSocket buffers not configurable | High | Performance |
| No request timeouts | High | Reliability |
| No logging configuration | High | Operations |
| Hardcoded thumbnails | Medium | Configuration |
| Health check incomplete | Medium | Operations |
| No graceful shutdown timeout | Medium | Reliability |
| Argon2 params hardcoded | Medium | Security |
| Debug endpoint exposed | Medium | Security |
| Hub buffers hardcoded | Medium | Performance |
| Relative upload path | Low | Deployment |
| Hardcoded email link | Low | Configuration |
| No numeric validation | Low | Validation |
| No Base64 validation in config | Low | Validation |
| Config not runtime injectable | Low | Deployment |

## Recommended Configuration Structure

```yaml
server:
  listen: ":6060"
  request_timeout: 30s
  shutdown_timeout: 30s
  use_x_forwarded_for: false
  cors_origins: []  # Empty = reject all, ["*"] = allow all (dev only)

database:
  host: localhost
  port: 5432
  name: mvchat2
  user: ""  # Required
  password: ""  # Required
  ssl_mode: require
  encryption_key: ""  # Required, no default
  uid_key: ""  # Required, no default
  pool:
    max_open: 25
    max_idle: 10
    conn_max_lifetime: 1h

redis:
  enabled: false
  host: localhost
  port: 6379
  password: ""
  db: 0
  namespace: mvchat2

auth:
  api_key_salt: ""  # Required, no default
  token:
    key: ""  # Required, no default
    expiry: 720h
  argon2:
    time: 2
    memory: 65536
    threads: 4
  min_login_length: 3
  min_password_length: 8

email:
  enabled: false
  host: ""
  port: 587
  username: ""
  password: ""
  from: ""
  app_name: "Clingy"
  app_link: ""

media:
  upload_dir: /data/uploads
  ffmpeg_path: /usr/bin/ffmpeg
  ffprobe_path: /usr/bin/ffprobe
  max_upload_size: 52428800
  thumbnail:
    width: 256
    height: 256
    quality: 80

websocket:
  write_wait: 10s
  pong_wait: 60s
  max_message_size: 65536
  send_buffer_size: 128

logging:
  level: info
  format: json
  output: stdout

debug:
  enabled: false
  expvar_path: ""
```
