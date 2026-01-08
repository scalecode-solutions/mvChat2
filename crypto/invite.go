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
	key []byte
	ttl time.Duration
}

// NewInviteTokenGenerator creates a new invite token generator.
// Key must be at least 32 bytes for security.
// TTL is how long tokens are valid (e.g., 7 days).
func NewInviteTokenGenerator(key []byte, ttl time.Duration) (*InviteTokenGenerator, error) {
	if len(key) < 32 {
		return nil, ErrInviteKeyTooShort
	}
	return &InviteTokenGenerator{key: key, ttl: ttl}, nil
}

// Generate creates a cryptographic invite token encoding inviter and invitee emails.
// The token format is: base64url(entropy || timestamp || inviterLen || inviter || invitee || hmac)
// - entropy: 16 random bytes
// - timestamp: 8 bytes (unix seconds)
// - inviterLen: 2 bytes (length of inviter email)
// - inviter: inviter email bytes
// - invitee: invitee email bytes
// - hmac: 32 bytes HMAC-SHA256 of all preceding data
func (g *InviteTokenGenerator) Generate(inviterEmail, inviteeEmail string) (string, error) {
	// Random entropy (16 bytes)
	entropy := make([]byte, 16)
	if _, err := rand.Read(entropy); err != nil {
		return "", err
	}

	// Timestamp (8 bytes, unix seconds)
	timestamp := make([]byte, 8)
	binary.BigEndian.PutUint64(timestamp, uint64(time.Now().Unix()))

	// Inviter email length (2 bytes, max 65535 chars)
	inviterBytes := []byte(inviterEmail)
	inviteeBytes := []byte(inviteeEmail)
	inviterLen := make([]byte, 2)
	binary.BigEndian.PutUint16(inviterLen, uint16(len(inviterBytes)))

	// Build payload: entropy || timestamp || inviterLen || inviter || invitee
	payload := make([]byte, 0, 16+8+2+len(inviterBytes)+len(inviteeBytes))
	payload = append(payload, entropy...)
	payload = append(payload, timestamp...)
	payload = append(payload, inviterLen...)
	payload = append(payload, inviterBytes...)
	payload = append(payload, inviteeBytes...)

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, g.key)
	mac.Write(payload)
	sig := mac.Sum(nil)

	// Final token: payload || signature
	token := append(payload, sig...)

	// URL-safe base64 encoding
	return base64.URLEncoding.EncodeToString(token), nil
}

// Verify decodes and verifies an invite token.
// Returns the decoded data if valid, or an error if invalid or expired.
func (g *InviteTokenGenerator) Verify(token string) (*InviteTokenData, error) {
	// Decode base64
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidInviteToken
	}

	// Minimum size: 16 (entropy) + 8 (timestamp) + 2 (inviterLen) + 32 (hmac) = 58 bytes
	if len(data) < 58 {
		return nil, ErrInvalidInviteToken
	}

	// Extract signature (last 32 bytes)
	payload := data[:len(data)-32]
	sig := data[len(data)-32:]

	// Verify HMAC
	mac := hmac.New(sha256.New, g.key)
	mac.Write(payload)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, ErrInvalidInviteToken
	}

	// Parse payload
	// entropy: bytes 0-15
	// timestamp: bytes 16-23
	timestamp := binary.BigEndian.Uint64(payload[16:24])
	createdAt := time.Unix(int64(timestamp), 0)

	// Check expiration
	if time.Since(createdAt) > g.ttl {
		return nil, ErrInviteTokenExpired
	}

	// inviterLen: bytes 24-25
	inviterLen := binary.BigEndian.Uint16(payload[24:26])

	// Validate lengths
	if int(26+inviterLen) > len(payload) {
		return nil, ErrInvalidInviteToken
	}

	// Extract emails
	inviterEmail := string(payload[26 : 26+inviterLen])
	inviteeEmail := string(payload[26+inviterLen:])

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
