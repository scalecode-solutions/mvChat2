# Invite Token System

This document explains how the invite code system works, including the cryptographic token generation, short code derivation, and database encryption.

## Overview

The invite system has a **two-layer architecture**:

1. **Short Code** (10 characters) - User-friendly code that gets shared (e.g., `9FAdTEmVCt`)
2. **Full Token** (~82 characters) - Cryptographic token stored encrypted in the database

## Why Two Layers?

- **Short codes are easy to share** - Users can copy/paste or even type them manually
- **Full tokens contain verification data** - Inviter identity, invitee email, timestamp, and HMAC signature
- **Privacy protection** - Even if the database is compromised, the encrypted tokens cannot be read

## Token Generation Flow

```
1. User creates invite for bob@example.com
                ↓
2. Generate full token with:
   - 8 bytes random entropy
   - 4 bytes timestamp (unix seconds)
   - 1 byte inviter username length
   - Inviter username bytes
   - Invitee email bytes
   - 16 bytes HMAC-SHA256 signature
                ↓
3. Encode as URL-safe base64 → Full Token (~82 chars)
                ↓
4. Derive short code: HMAC-SHA256(token) → first 8 bytes → base64 → trim to 10 chars
                ↓
5. Encrypt full token with AES-256-GCM before storing
                ↓
6. Store in database:
   - code: "9FAdTEmVCt" (short code, UNIQUE index)
   - token: encrypted blob (~148 chars base64)
   - email: "bob@example.com"
```

## Token Redemption Flow

```
1. User enters short code: "9FAdTEmVCt"
                ↓
2. Database lookup by short code
                ↓
3. Decrypt the stored token with AES-256-GCM
                ↓
4. Verify HMAC signature on decrypted token
                ↓
5. Check token hasn't expired (7 days TTL)
                ↓
6. Mark invite as used, create DM with inviter
```

## Cryptographic Details

### Full Token Format

```
base64url(
  entropy[8]      ||  # Random bytes for uniqueness
  timestamp[4]    ||  # Unix timestamp (seconds)
  inviterLen[1]   ||  # Length of inviter username
  inviter[...]    ||  # Inviter username bytes
  invitee[...]    ||  # Invitee email bytes
  hmac[16]            # Truncated HMAC-SHA256
)
```

### Short Code Derivation

```go
mac := hmac.New(sha256.New, key)
mac.Write([]byte(token))
hash := mac.Sum(nil)
shortCode := base64url(hash[:8])[:10]  // Deterministic 10-char code
```

The short code is **deterministic** - the same token always produces the same short code. This is important because we generate the short code before storing, and use it for database lookup.

### Database Encryption

Tokens are encrypted before storage using AES-256-GCM:

```go
// Encrypt before storing
encryptedToken, _ := inviteTokens.EncryptForStorage(token)
db.CreateInviteCode(code, encryptedToken, email, ...)

// Decrypt when retrieving
invite := db.GetInviteByCode(shortCode)
token, _ := inviteTokens.DecryptFromStorage(invite.Token)
inviteTokens.Verify(token)
```

**Why AES-GCM?**
- Provides both confidentiality and integrity
- Each encryption uses a random 12-byte nonce
- Same token encrypted twice produces different ciphertexts
- Tampering is detected (authentication tag)

## Privacy Guarantees

| Data | Visible to Inviter | Visible to Invitee | Visible in DB (if breached) |
|------|-------------------|--------------------|-----------------------------|
| Inviter's email/username | Yes | No | No (encrypted) |
| Invitee's email | Yes | Yes (they provided it) | Yes (stored in plaintext for lookups) |
| Short code | Yes | Yes (shared with them) | Yes |
| Full token | Never exposed | Never exposed | No (encrypted) |
| Inviter's display name | Yes | Yes (after redeem) | Yes (in users table) |

## Key Management

The same 32-byte key is used for:
1. HMAC signing of tokens
2. Short code derivation
3. AES-256-GCM encryption of stored tokens

This key is configured in `mvchat2.yaml`:

```yaml
security:
  token_key: "your-32-byte-or-longer-key-here"
```

**Important**: If this key is rotated, all existing invite codes become invalid.

## Database Schema

```sql
CREATE TABLE invite_codes (
    id UUID PRIMARY KEY,
    inviter_id UUID NOT NULL REFERENCES users(id),
    code VARCHAR(10) NOT NULL UNIQUE,  -- Short code for lookup
    token TEXT NOT NULL,                -- AES-GCM encrypted full token
    email VARCHAR(255) NOT NULL,        -- Invitee's email (for other lookups)
    invitee_name VARCHAR(128),
    status VARCHAR(16) DEFAULT 'pending',
    used_at TIMESTAMPTZ,
    used_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '7 days')
);

CREATE INDEX idx_invite_codes_code ON invite_codes(code) WHERE status = 'pending';
```

## Security Considerations

1. **Token expiration**: Tokens expire after 7 days (configurable TTL)
2. **Single use**: Once redeemed, status changes to 'used' and cannot be reused
3. **No email exposure**: The inviter's email is never sent to the invitee
4. **Rate limiting**: Should be implemented at the API level (not in this package)
5. **Brute force protection**: 10-char base64 code = 64^10 ≈ 1.15 × 10^18 combinations

## Testing

Run the crypto tests:

```bash
go test ./crypto/... -v
```

Key test cases:
- Token generation and verification
- Short code determinism
- Encryption/decryption round-trip
- Tampered ciphertext rejection
- Wrong key rejection
- Token expiration

## Troubleshooting

### "invalid invite code" error
- Token may have been tampered with
- Encryption key may have been rotated
- Token format may be corrupted

### "invite code expired" error
- Token TTL (7 days) has passed
- Check `expires_at` in database

### Invite not found
- Short code doesn't exist in database
- Invite already used (status != 'pending')
- Invite was revoked
