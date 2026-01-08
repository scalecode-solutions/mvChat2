# mvChat2

A ground-up rewrite of the mvChat backend - a real-time messaging server for the Clingy app.

## Overview

mvChat2 is a simplified, robust WebSocket-based messaging backend built with Go and PostgreSQL. It provides:

- **Direct Messages (1:1)** and **Group Chats**
- **Rich Messaging** with Irido format (text, media, replies, mentions)
- **Message Actions** (reactions, edit, unsend, delete for me/everyone)
- **Presence** (online/offline/last seen, typing indicators, read receipts)
- **File Uploads** with thumbnails and media processing
- **Encryption at Rest** for message content (AES-256-GCM)
- **JWT Authentication** with 2-week token expiry

## Quick Start

```bash
# Prerequisites: Go 1.21+, PostgreSQL 15+, FFmpeg, ImageMagick

# Clone and build
git clone https://github.com/scalecode-solutions/mvchat2.git
cd mvchat2
go build .

# Initialize database
createdb mvchat2
./mvchat2 -init-db

# Run
./mvchat2
```

## Docker

```bash
docker-compose up -d
```

## Configuration

Configuration is in `mvchat2.yaml` with environment variable support:

```yaml
server:
  listen: ":6060"
  use_x_forwarded_for: true

database:
  host: ${DB_HOST:localhost}
  port: 5432
  name: ${DB_NAME:mvchat2}
  user: ${DB_USER:postgres}
  password: ${DB_PASSWORD:}
  encryption_key: ${ENCRYPTION_KEY:base64-encoded-32-byte-key}

auth:
  token:
    key: ${TOKEN_KEY:base64-encoded-32-byte-key}
    expire_in: 1209600  # 2 weeks
```

## Architecture

```
mvChat2/
├── main.go              # Entry point, server initialization
├── hub.go               # Session registry, message routing
├── session.go           # WebSocket connection management
├── server.go            # HTTP server, WebSocket upgrade
├── presence.go          # Online/offline notifications
├── types.go             # Wire protocol message types
├── handlers.go          # Auth handlers (login, register, search)
├── handlers_conv.go     # Conversation/message handlers
├── handlers_files.go    # File upload/download HTTP handlers
├── auth/                # JWT tokens, password hashing (Argon2id)
├── config/              # YAML configuration loading
├── crypto/              # AES-GCM encryption for messages
├── irido/               # Message content format (Unicode 17.0)
├── media/               # Image/video/audio processing
├── store/               # PostgreSQL data access layer
│   ├── db.go            # Connection pool, schema init
│   ├── users.go         # User CRUD, auth records
│   ├── conversations.go # DM/group management
│   ├── messages.go      # Message CRUD, reactions
│   ├── files.go         # File metadata
│   └── schema.sql       # Database schema
├── Dockerfile           # Multi-stage build with FFmpeg
└── docker-compose.yml   # PostgreSQL + mvChat2
```

## Wire Protocol

All communication uses JSON over WebSocket at `/v0/ws`.

### Client → Server Messages

| Message | Description |
|---------|-------------|
| `{hi}` | Handshake with version, user agent |
| `{login}` | Authenticate (basic or token) |
| `{acc}` | Create/update account |
| `{search}` | Search users by name |
| `{dm}` | Start DM or manage settings |
| `{group}` | Create/manage group |
| `{send}` | Send message |
| `{get}` | Get conversations, messages, members |
| `{edit}` | Edit message (15 min window, max 10) |
| `{unsend}` | Unsend message (10 min window) |
| `{delete}` | Delete for me or everyone |
| `{react}` | Toggle emoji reaction |
| `{typing}` | Typing indicator |
| `{read}` | Mark messages as read |

### Server → Client Messages

| Message | Description |
|---------|-------------|
| `{ctrl}` | Response to client request |
| `{data}` | New message in conversation |
| `{info}` | Notification (edit, unsend, react, typing, read) |
| `{pres}` | Presence update (online/offline) |

### Example: Login Flow

```json
// Client sends handshake
{"hi": {"ver": "0.1.0", "ua": "Clingy/1.0"}}

// Server responds
{"ctrl": {"code": 200, "params": {"ver": "0.1.0", "sid": "session-uuid"}}}

// Client logs in with token
{"login": {"scheme": "token", "secret": "jwt-token-here"}}

// Server responds with user info and refreshed token
{"ctrl": {"code": 200, "params": {"user": "user-uuid", "token": "new-jwt", "expires": "..."}}}
```

### Example: Send Message

```json
// Client sends message
{"id": "123", "send": {"conv": "conv-uuid", "content": {"v": 1, "text": "Hello!"}}}

// Server confirms
{"ctrl": {"id": "123", "code": 202, "params": {"conv": "conv-uuid", "seq": 42, "ts": "..."}}}

// Other members receive
{"data": {"conv": "conv-uuid", "seq": 42, "from": "user-uuid", "content": {"v": 1, "text": "Hello!"}, "ts": "..."}}
```

## Database Schema

### Tables

| Table | Description |
|-------|-------------|
| `users` | User accounts with public profile data |
| `auth` | Authentication records (basic login, tokens) |
| `conversations` | DMs and groups |
| `dm_participants` | Fast DM lookup between two users |
| `members` | Conversation membership with settings |
| `messages` | Messages with encrypted content |
| `message_deletions` | Per-user soft deletes |
| `files` | Uploaded file metadata |
| `file_metadata` | Media dimensions, thumbnails |
| `schema_version` | Migration tracking |

### Key Design Decisions

- **UUIDs** for all primary keys
- **Hybrid soft delete**: `clear_seq` for bulk, `message_deletions` for per-message
- **Encryption at rest**: Message content stored as encrypted BYTEA
- **Sequence numbers**: Per-conversation message ordering

## Irido Message Format

Irido is the message content format, supporting:

```json
{
  "v": 1,
  "text": "Hello @user! Check this out",
  "media": [
    {"type": "image", "ref": "file-uuid", "mime": "image/jpeg", "width": 800, "height": 600}
  ],
  "reply": {"seq": 41, "preview": "Previous message...", "from": "user-uuid"},
  "mentions": [{"userId": "user-uuid", "username": "user", "offset": 6, "length": 5}]
}
```

Features:
- Markdown text
- Up to 10 media attachments (image, video, audio, file, embed)
- Reply references with preview
- User mentions with grapheme-aware offsets (Unicode 17.0 via runeseg)

## HTTP Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v0/ws` | GET | WebSocket upgrade |
| `/v0/file/upload` | POST | Upload file (multipart) |
| `/v0/file/{id}` | GET | Download file |
| `/v0/file/{id}/thumb` | GET | Download thumbnail |
| `/health` | GET | Health check |

## Security

- **Passwords**: Argon2id hashing (memory-hard)
- **Tokens**: JWT with HS256, 2-week expiry, auto-refresh on login
- **Messages**: AES-256-GCM encryption at rest
- **Files**: Auth required for upload/download

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `pgx/v5` | 5.8.0 | PostgreSQL driver |
| `gorilla/websocket` | 1.5.3 | WebSocket |
| `golang-jwt/jwt/v5` | 5.3.0 | JWT tokens |
| `google/uuid` | 1.6.0 | UUID generation |
| `scalecode-solutions/runeseg` | 1.0.4 | Unicode 17.0 segmentation |
| `golang.org/x/crypto` | 0.46.0 | Argon2id |
| `golang.org/x/image` | 0.34.0 | Image processing |
| `gopkg.in/yaml.v3` | 3.0.1 | Config parsing |

## Stats

- **~5,300 lines** of Go code
- **22 source files**
- **9 database tables**
- **Zero external runtime dependencies** (except PostgreSQL, FFmpeg, ImageMagick)

## License

Proprietary - ScaleCode Solutions
