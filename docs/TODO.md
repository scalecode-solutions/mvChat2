# mvChat2 TODO & Design

## Overview

mvChat2 is a secure chat backend for Clingy, a DV survivor support app disguised as a pregnancy tracker. This document covers the invite flow, security architecture, and pending features.

## Scenarios

### 1. New User Signup (Alice invites Bob)

**Context:** Alice wants to invite her lawyer Bob to chat. Bob doesn't have an account yet.

**Flow:**
1. Alice creates invite: `{"invite":{"create":{"email":"bob@law.com","name":"Bob"}}}`
2. Bob receives email with 10-digit code (e.g., `6793336885`)
3. Bob goes to signup page, enters the invite code
4. Backend creates account with invite code as temporary password
5. Bob is immediately prompted to set a new password
6. DM is created between Alice and Bob
7. Alice and Bob are added as contacts
8. Bob lands in the DM with Alice

**Backend Response:**
```json
{
  "ctrl": {
    "code": 201,
    "params": {
      "user": "bob-uuid",
      "token": "jwt...",
      "inviters": ["alice-uuid"]
    }
  }
}
```

### 2. Existing User Redeems Invite (Cathy invites existing Bob)

**Context:** Cathy also wants to invite Bob. Bob already has an account from Alice's invite.

**Flow:**
1. Cathy creates invite to `bob@law.com`
2. Bob receives email with a different code
3. Bob goes to invite redeem page, enters the code
4. System detects Bob already has an account (by email)
5. Bob is redirected to sign in
6. After login, Bob sends: `{"invite":{"redeem":"0987654321"}}`
7. DM is created between Cathy and Bob
8. Cathy and Bob are added as contacts
9. Bob lands in the DM with Cathy

**Backend Response:**
```json
{
  "ctrl": {
    "code": 200,
    "params": {
      "inviter": "cathy-uuid",
      "inviterPublic": {"fn": "Cathy"},
      "conv": "dm-uuid"
    }
  }
}
```

### 3. Manual Contact Add (Alice searches for Cathy)

**Context:** Alice and Cathy don't know each other, but Alice finds Cathy via search.

**Flow:**
1. Alice searches: `{"search":{"query":"cathy"}}`
2. Alice adds Cathy as contact: `{"contact":{"add":"cathy-uuid"}}`
3. Cathy is added to Alice's contacts (bidirectional)

**Backend Response:**
```json
{
  "ctrl": {
    "code": 200,
    "params": {
      "user": "cathy-uuid",
      "public": {"fn": "Cathy"}
    }
  }
}
```

## Privacy

- **Contacts are private:** Alice cannot see Bob's contacts, Cathy cannot see Alice's contacts
- **Invites are private:** Only the inviter can see their sent invites
- **DMs are separate:** Alice-Bob DM is separate from Cathy-Bob DM

## Wire Protocol

### Create Invite
```json
{"id":"1","invite":{"create":{"email":"bob@law.com","name":"Bob"}}}
```

### List My Invites
```json
{"id":"2","invite":{"list":true}}
```

### Revoke Invite
```json
{"id":"3","invite":{"revoke":"invite-uuid"}}
```

### Redeem Invite (Existing User)
```json
{"id":"4","invite":{"redeem":"6793336885"}}
```

### Get Contacts
```json
{"id":"5","get":{"what":"contacts"}}
```

### Add Contact Manually
```json
{"id":"6","contact":{"add":"user-uuid"}}
```

### Remove Contact
```json
{"id":"7","contact":{"remove":"user-uuid"}}
```

### Update Contact Nickname
```json
{"id":"8","contact":{"user":"user-uuid","nickname":"My Lawyer"}}
```

### Create Room
```json
{"id":"9","room":{"action":"create","desc":{"public":{"name":"Room Name"}}}}
```

### Invite to Room
```json
{"id":"10","room":{"id":"room-uuid","action":"invite","user":"user-uuid"}}
```

### Leave Room
```json
{"id":"11","room":{"id":"room-uuid","action":"leave"}}
```

### Kick from Room
```json
{"id":"12","room":{"id":"room-uuid","action":"kick","user":"user-uuid"}}
```

### Update Room
```json
{"id":"13","room":{"id":"room-uuid","action":"update","desc":{"public":{"name":"New Name"}}}}
```

## Database Schema

### invite_codes
| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| inviter_id | UUID | Who created the invite |
| code | VARCHAR(10) | Short 10-char base64url code (user-friendly) |
| token | TEXT | AES-256-GCM encrypted full cryptographic token |
| email | VARCHAR(255) | Recipient email |
| invitee_name | VARCHAR(128) | Optional display name |
| status | VARCHAR(16) | 'pending', 'used', 'expired', 'revoked' |
| used_at | TIMESTAMPTZ | When redeemed |
| used_by | UUID | Who redeemed it |
| expires_at | TIMESTAMPTZ | Default: 7 days from creation |

**Note:** The invite system uses a two-layer architecture. See [docs/invite-tokens.md](invite-tokens.md) for full cryptographic details.

### contacts
| Column | Type | Description |
|--------|------|-------------|
| user_id | UUID | The user who has this contact |
| contact_id | UUID | The contact user |
| source | VARCHAR(16) | 'invite' or 'manual' |
| nickname | VARCHAR(64) | Optional custom name |
| invite_id | UUID | Reference to invite (if source='invite') |

## Completed

### Security (January 2025)
- [x] **Two-layer invite code system** - Short 10-char base64url codes for sharing, full cryptographic tokens stored encrypted
- [x] **Invite token encryption** - AES-256-GCM encryption for tokens before database storage (prevents exposure if DB compromised)
- [x] **Email header injection prevention** - Sanitization for CR/LF characters in email headers
- [x] **Email HTML injection prevention** - html.EscapeString() for all user-controlled template data
- [x] **Email validation** - mail.ParseAddress() validation before sending
- [x] **Invite token HMAC signatures** - HMAC-SHA256 verification for token integrity
- [x] **Password change endpoint** - Users can change their password
- [x] **Unsend time limit** - 5-minute window for unsending messages
- [x] **Edit limits** - 10 edits per message within 15 minutes

### SDKs (January 2025)
- [x] **React Native SDK** - Full TypeScript SDK with hooks (useAuth, useMessages, useConversations, useContacts, useTyping)
- [x] **SDK documentation** - Comprehensive docs in sdk/README.md

### Rooms (January 2025)
- [x] **Room management** - invite, leave, kick, update actions with role-based permissions
- [x] **Role system** - owner/admin/member roles with permission hierarchy
- [x] **Member broadcasts** - Real-time notifications for member_joined, member_left, member_kicked, room_updated

### Infrastructure
- [x] **Rate limiting** - Caddy-based rate limiting (100 req/min API, 10 req/min auth)
- [x] **WebSocket typing indicators** - Client-side debouncing (3s), not subject to HTTP rate limits

## Future Enhancements

### Backend (mvChat2)
- [x] Password change endpoint
- [x] `must_change_password` flag for temp passwords
- [x] Store email on user account (from invite, updateable)
- [x] Email verification flow (optional, disabled by default for DV safety)
- [ ] SMS invite codes (alternative to email)
- [ ] Message search (metadata only - sender, date, conversation)
- [ ] User-controlled encrypted search index (client builds, encrypts, uploads; only user can search their own content)
- [x] Room management (invite/leave/kick/update with role-based permissions)
- [ ] In-app audio calls (WebRTC without CallKit - stealth mode)
- [ ] Webhooks (optional - for enterprise/professional integrations)
- [ ] @mention notifications (special indicator when mentioned in rooms)
- [ ] @everyone / @here for rooms
- [ ] Trusted account badges (`verified` flag + `credentials` JSONB for professionals)
- [ ] Account suspension (`suspended_at`, `suspended_reason`)
- [ ] Admin endpoints for user management
- [ ] User language preference in profile (for client-side translation)
- [ ] Scheduled messages (send at future time)
- [ ] Pinned messages in rooms
- [ ] Disappearing messages (auto soft-delete after X time)
- [x] Unsend time limit enforcement (5 minutes)
- [ ] Delete for everyone (separate from unsend, no time limit)
- [x] Edit limits (10 edits per message within 15 minutes, then locked)
- [ ] Location sharing for emergencies
- [ ] Pre-recorded distress messages (record when safe, send with one tap when in danger)
- [ ] Emergency quick-send to all trusted contacts
- [ ] No-screenshots preference (stored server-side, enforced client-side)
- [ ] Graceful maintenance mode (stop accepting new connections, let existing finish)
- [ ] Session resumption (reconnect with session ID, restore state without re-auth)
- [ ] History recovery (get missed messages since disconnect)
- [ ] Delta updates (send only changed fields, reduce bandwidth)
- [ ] SSE/HTTP-streaming fallback (for environments blocking WebSocket)
- [ ] Channel patterns/wildcards (subscribe to `room:*`)
- [ ] Room deletion/archival (owner can archive, data persists for evidence)
- [ ] Room cleanup policy (configurable TTL for inactive rooms, or keep forever)

### SDKs
- [x] React Native SDK (for Clingy and future mobile apps)
- [x] TypeScript/JavaScript SDK (for web clients) - included in React Native SDK
- [ ] Swift SDK (native iOS)
- [ ] Kotlin SDK (native Android)
- [x] SDK documentation and examples

### Web Client (chat.mvchat.app)
- [ ] Web version of Clingy chat interface
- [ ] Invite code redemption page
- [ ] Sign in / sign up flow
- [ ] Password change UI
- [ ] DM and room messaging
- [ ] File upload/download
- [ ] Contacts management
- [ ] Profile settings
- [ ] Integrated PDF viewer (no external app trail)
- [ ] Screenshot blocking (based on server preference)

## Design Decisions

### No Push Notifications
Push notifications are intentionally NOT implemented. This is a security feature for DV survivors - the app disguises as a pregnancy tracker (Clingy), and push notifications would reveal the hidden chat functionality. Users must open the app to check messages.

### No CallKit Integration
Future audio calls will use WebRTC WITHOUT iOS CallKit. This prevents calls from appearing in the phone's call log, which would expose the hidden chat. Calls only work when the app is open in chat mode.

### Private Rooms Only
All rooms are private/invite-only by default. No public room discovery - doesn't fit the DV survivor use case.

### Triple-Layer Security
1. **E2EE (client-to-client)** - Messages encrypted on sender's device, decrypted on recipient's device. Server never sees plaintext.
2. **Encryption at rest (AES-GCM)** - Server stores the already-E2EE ciphertext, then encrypts it again. Double encrypted.
3. **Soft delete + admin restore** - Nothing is truly deleted. If abuser finds phone and deletes evidence, admin can restore soft-deleted messages.

### Evidence Preservation
- All deletes are soft deletes (records kept with `deleted_at` timestamp)
- Admin can restore deleted messages for a user
- User exports as plaintext (their client has E2EE keys)
- Evidence preserved for court proceedings

### Subpoena-Proof
- Server only stores double-encrypted blobs
- No plaintext ever touches the server
- Even with a court order, only encrypted data can be provided
- Only the clients hold the decryption keys
