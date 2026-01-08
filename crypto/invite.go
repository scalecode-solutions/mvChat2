package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"strings"
	"time"
)

var (
	ErrInvalidInviteToken = errors.New("invalid invite token")
	ErrInviteTokenExpired = errors.New("invite token expired")
	ErrInviteKeyTooShort  = errors.New("invite key must be at least 32 bytes")
)

// base62 alphabet for compact encoding
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

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

// Generate creates a compact cryptographic invite token.
// Format: base62(entropy || timestamp || inviterLen || inviter || invitee || hmac_truncated)
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

	// Encode as base62 for compact representation
	return encodeBase62(token), nil
}

// Verify decodes and verifies an invite token.
// Returns the decoded data if valid, or an error if invalid or expired.
func (g *InviteTokenGenerator) Verify(token string) (*InviteTokenData, error) {
	// Decode base62
	data, err := decodeBase62(token)
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

// encodeBase62 encodes bytes to base62 string.
func encodeBase62(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Convert bytes to big integer
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result strings.Builder
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		result.WriteByte(base62Alphabet[mod.Int64()])
	}

	// Handle leading zeros in input
	for _, b := range data {
		if b != 0 {
			break
		}
		result.WriteByte('0')
	}

	// Reverse the string
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// decodeBase62 decodes a base62 string to bytes.
func decodeBase62(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, errors.New("empty string")
	}

	// Build index map
	indexMap := make(map[rune]int64)
	for i, c := range base62Alphabet {
		indexMap[c] = int64(i)
	}

	// Convert base62 to big integer
	num := big.NewInt(0)
	base := big.NewInt(62)

	for _, c := range s {
		idx, ok := indexMap[c]
		if !ok {
			return nil, errors.New("invalid character")
		}
		num.Mul(num, base)
		num.Add(num, big.NewInt(idx))
	}

	// Convert to bytes
	result := num.Bytes()

	// Handle leading zeros
	leadingZeros := 0
	for _, c := range s {
		if c != '0' {
			break
		}
		leadingZeros++
	}

	if leadingZeros > 0 {
		padded := make([]byte, leadingZeros+len(result))
		copy(padded[leadingZeros:], result)
		return padded, nil
	}

	return result, nil
}
