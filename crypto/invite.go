package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

var (
	ErrInvalidInviteToken = errors.New("invalid invite token")
	ErrInviteTokenExpired = errors.New("invite token expired")
	ErrInviteKeyTooShort  = errors.New("invite key must be at least 32 bytes")
)

// InviteTokenData contains the decoded invite token payload.
type InviteTokenData struct {
	InviterEmail string
	InviteeEmail string
	CreatedAt    time.Time
}

// InviteTokenGenerator creates and verifies cryptographic invite tokens.
type InviteTokenGenerator struct {
	key       []byte
	ttl       time.Duration
	encryptor *Encryptor // For encrypting tokens before DB storage
}

// NewInviteTokenGenerator creates a new invite token generator.
// Key must be at least 32 bytes for security.
// TTL is how long tokens are valid (e.g., 7 days).
func NewInviteTokenGenerator(key []byte, ttl time.Duration) (*InviteTokenGenerator, error) {
	if len(key) < 32 {
		return nil, ErrInviteKeyTooShort
	}

	// Create encryptor for DB storage encryption using the same key
	encryptor, err := NewEncryptor(key[:32])
	if err != nil {
		return nil, err
	}

	return &InviteTokenGenerator{key: key, ttl: ttl, encryptor: encryptor}, nil
}

// Generate creates a compact cryptographic invite token.
// Format: base64url(entropy || timestamp || inviterLen || inviter || invitee || hmac_truncated)
// - entropy: 8 random bytes (64 bits - still very secure for invite codes)
// - timestamp: 4 bytes (unix seconds, good until 2106)
// - inviterLen: 1 byte (max 255 chars for username)
// - inviter: inviter username bytes
// - invitee: invitee email bytes
// - hmac: 16 bytes (128 bits - truncated HMAC-SHA256, still cryptographically secure)
func (g *InviteTokenGenerator) Generate(inviterEmail, inviteeEmail string) (string, error) {
	// Random entropy (8 bytes)
	entropy := make([]byte, 8)
	if _, err := rand.Read(entropy); err != nil {
		return "", err
	}

	// Timestamp (4 bytes, unix seconds)
	timestamp := make([]byte, 4)
	binary.BigEndian.PutUint32(timestamp, uint32(time.Now().Unix()))

	// Inviter length (1 byte, max 255 chars)
	inviterBytes := []byte(inviterEmail)
	inviteeBytes := []byte(inviteeEmail)
	if len(inviterBytes) > 255 {
		inviterBytes = inviterBytes[:255]
	}

	// Build payload: entropy || timestamp || inviterLen || inviter || invitee
	payload := make([]byte, 0, 8+4+1+len(inviterBytes)+len(inviteeBytes))
	payload = append(payload, entropy...)
	payload = append(payload, timestamp...)
	payload = append(payload, byte(len(inviterBytes)))
	payload = append(payload, inviterBytes...)
	payload = append(payload, inviteeBytes...)

	// Compute HMAC-SHA256 and truncate to 16 bytes
	mac := hmac.New(sha256.New, g.key)
	mac.Write(payload)
	sig := mac.Sum(nil)[:16] // Truncate to 128 bits

	// Final token: payload || signature
	token := append(payload, sig...)

	// Encode as URL-safe base64 (no padding for cleaner URLs)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

// Verify decodes and verifies an invite token.
// Returns the decoded data if valid, or an error if invalid or expired.
func (g *InviteTokenGenerator) Verify(token string) (*InviteTokenData, error) {
	// Decode URL-safe base64
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidInviteToken
	}

	// Minimum size: 8 (entropy) + 4 (timestamp) + 1 (inviterLen) + 16 (hmac) = 29 bytes
	if len(data) < 29 {
		return nil, ErrInvalidInviteToken
	}

	// Extract signature (last 16 bytes)
	payload := data[:len(data)-16]
	sig := data[len(data)-16:]

	// Verify HMAC (truncated)
	mac := hmac.New(sha256.New, g.key)
	mac.Write(payload)
	expectedSig := mac.Sum(nil)[:16]

	if !hmac.Equal(sig, expectedSig) {
		return nil, ErrInvalidInviteToken
	}

	// Parse payload
	// entropy: bytes 0-7
	// timestamp: bytes 8-11
	timestamp := binary.BigEndian.Uint32(payload[8:12])
	createdAt := time.Unix(int64(timestamp), 0)

	// Check expiration
	if time.Since(createdAt) > g.ttl {
		return nil, ErrInviteTokenExpired
	}

	// inviterLen: byte 12
	inviterLen := int(payload[12])

	// Validate lengths
	if 13+inviterLen > len(payload) {
		return nil, ErrInvalidInviteToken
	}

	// Extract emails
	inviterEmail := string(payload[13 : 13+inviterLen])
	inviteeEmail := string(payload[13+inviterLen:])

	return &InviteTokenData{
		InviterEmail: inviterEmail,
		InviteeEmail: inviteeEmail,
		CreatedAt:    createdAt,
	}, nil
}

// VerifyForRecipient verifies the token and confirms it was intended for the given email.
func (g *InviteTokenGenerator) VerifyForRecipient(token, recipientEmail string) (*InviteTokenData, error) {
	data, err := g.Verify(token)
	if err != nil {
		return nil, err
	}

	if data.InviteeEmail != recipientEmail {
		return nil, ErrInvalidInviteToken
	}

	return data, nil
}

// ShortCode generates a short 10-character alphanumeric code from a token.
// This is a deterministic hash - the same token always produces the same short code.
// The short code is used as a user-friendly lookup key; the full token is stored in DB.
func (g *InviteTokenGenerator) ShortCode(token string) string {
	// Hash the token with the key to create a deterministic short code
	mac := hmac.New(sha256.New, g.key)
	mac.Write([]byte(token))
	hash := mac.Sum(nil)

	// Take first 6 bytes and encode as base64url, then trim to 10 chars
	// 6 bytes = 8 base64 chars, but we want 10 for more entropy
	// Use first 8 bytes (gives us 11 base64 chars, take first 10)
	encoded := base64.RawURLEncoding.EncodeToString(hash[:8])
	if len(encoded) > 10 {
		encoded = encoded[:10]
	}

	return encoded
}

// EncryptForStorage encrypts a token for secure database storage.
// This prevents exposure of embedded emails if the database is compromised.
// Returns a base64-encoded encrypted token.
func (g *InviteTokenGenerator) EncryptForStorage(token string) (string, error) {
	return g.encryptor.EncryptString(token)
}

// DecryptFromStorage decrypts a token retrieved from the database.
// Returns the original token that can be verified with Verify().
func (g *InviteTokenGenerator) DecryptFromStorage(encryptedToken string) (string, error) {
	return g.encryptor.DecryptString(encryptedToken)
}
