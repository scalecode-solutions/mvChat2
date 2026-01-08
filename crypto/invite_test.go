package crypto

import (
	"strings"
	"testing"
	"time"
)

func TestInviteTokenGenerator(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gen, err := NewInviteTokenGenerator(key, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewInviteTokenGenerator failed: %v", err)
	}

	t.Run("generate and verify", func(t *testing.T) {
		inviter := "alice@example.com"
		invitee := "bob@example.com"

		token, err := gen.Generate(inviter, invitee)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		// Token should be URL-safe base64 (alphanumeric plus - and _)
		for _, c := range token {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' || c == '_') {
				t.Errorf("Token contains non-base64url character %q: %s", c, token)
				break
			}
		}

		t.Logf("Token length: %d chars", len(token))

		data, err := gen.Verify(token)
		if err != nil {
			t.Fatalf("Verify failed: %v", err)
		}

		if data.InviterEmail != inviter {
			t.Errorf("InviterEmail = %q, want %q", data.InviterEmail, inviter)
		}
		if data.InviteeEmail != invitee {
			t.Errorf("InviteeEmail = %q, want %q", data.InviteeEmail, invitee)
		}

		// CreatedAt should be recent
		if time.Since(data.CreatedAt) > time.Minute {
			t.Errorf("CreatedAt too old: %v", data.CreatedAt)
		}
	})

	t.Run("different tokens for same emails", func(t *testing.T) {
		inviter := "alice@example.com"
		invitee := "bob@example.com"

		token1, _ := gen.Generate(inviter, invitee)
		token2, _ := gen.Generate(inviter, invitee)

		if token1 == token2 {
			t.Error("Generated tokens should be unique due to entropy")
		}
	})

	t.Run("tampered token rejected", func(t *testing.T) {
		token, _ := gen.Generate("alice@example.com", "bob@example.com")

		// Tamper with middle of token
		tampered := []byte(token)
		tampered[len(tampered)/2] ^= 0xFF

		_, err := gen.Verify(string(tampered))
		if err != ErrInvalidInviteToken {
			t.Errorf("Verify(tampered) = %v, want ErrInvalidInviteToken", err)
		}
	})

	t.Run("wrong key rejected", func(t *testing.T) {
		otherKey := make([]byte, 32)
		for i := range otherKey {
			otherKey[i] = byte(i + 100)
		}
		otherGen, _ := NewInviteTokenGenerator(otherKey, 24*time.Hour)

		token, _ := gen.Generate("alice@example.com", "bob@example.com")

		_, err := otherGen.Verify(token)
		if err != ErrInvalidInviteToken {
			t.Errorf("Verify(wrong key) = %v, want ErrInvalidInviteToken", err)
		}
	})

	t.Run("verify for recipient", func(t *testing.T) {
		token, _ := gen.Generate("alice@example.com", "bob@example.com")

		// Correct recipient
		data, err := gen.VerifyForRecipient(token, "bob@example.com")
		if err != nil {
			t.Errorf("VerifyForRecipient failed: %v", err)
		}
		if data.InviteeEmail != "bob@example.com" {
			t.Errorf("InviteeEmail = %q, want bob@example.com", data.InviteeEmail)
		}

		// Wrong recipient
		_, err = gen.VerifyForRecipient(token, "charlie@example.com")
		if err != ErrInvalidInviteToken {
			t.Errorf("VerifyForRecipient(wrong) = %v, want ErrInvalidInviteToken", err)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		// Create generator with very short TTL
		shortGen, _ := NewInviteTokenGenerator(key, time.Millisecond)

		token, _ := shortGen.Generate("alice@example.com", "bob@example.com")

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		_, err := shortGen.Verify(token)
		if err != ErrInviteTokenExpired {
			t.Errorf("Verify(expired) = %v, want ErrInviteTokenExpired", err)
		}
	})

	t.Run("key too short rejected", func(t *testing.T) {
		shortKey := make([]byte, 16)
		_, err := NewInviteTokenGenerator(shortKey, 24*time.Hour)
		if err != ErrInviteKeyTooShort {
			t.Errorf("NewInviteTokenGenerator(short key) = %v, want ErrInviteKeyTooShort", err)
		}
	})

	t.Run("empty emails", func(t *testing.T) {
		token, err := gen.Generate("", "")
		if err != nil {
			t.Fatalf("Generate with empty emails failed: %v", err)
		}

		data, err := gen.Verify(token)
		if err != nil {
			t.Fatalf("Verify with empty emails failed: %v", err)
		}

		if data.InviterEmail != "" || data.InviteeEmail != "" {
			t.Error("Empty emails not preserved")
		}
	})

	t.Run("long emails", func(t *testing.T) {
		// Inviter is limited to 255 chars, invitee can be longer
		longInviter := strings.Repeat("a", 100) + "@example.com" // 112 chars
		longInvitee := strings.Repeat("b", 200) + "@example.com" // 212 chars
		token, err := gen.Generate(longInviter, longInvitee)
		if err != nil {
			t.Fatalf("Generate with long emails failed: %v", err)
		}

		data, err := gen.Verify(token)
		if err != nil {
			t.Fatalf("Verify with long emails failed: %v", err)
		}

		if data.InviterEmail != longInviter {
			t.Errorf("Long inviter email not preserved: got %d chars, want %d", len(data.InviterEmail), len(longInviter))
		}
		if data.InviteeEmail != longInvitee {
			t.Errorf("Long invitee email not preserved: got %d chars, want %d", len(data.InviteeEmail), len(longInvitee))
		}
	})
}

func TestInvalidTokenFormats(t *testing.T) {
	key := make([]byte, 32)
	gen, _ := NewInviteTokenGenerator(key, 24*time.Hour)

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"invalid chars", "not-valid!@#"},
		{"too short", "AAAA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gen.Verify(tt.token)
			if err == nil {
				t.Error("Expected error for invalid token format")
			}
		})
	}
}

func TestShortCode(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gen, err := NewInviteTokenGenerator(key, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewInviteTokenGenerator failed: %v", err)
	}

	token, err := gen.Generate("alice@example.com", "bob@example.com")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Generate short code
	code := gen.ShortCode(token)

	// Should be exactly 10 characters
	if len(code) != 10 {
		t.Errorf("ShortCode length = %d, want 10", len(code))
	}

	// Should be URL-safe base64 characters
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' || c == '_') {
			t.Errorf("ShortCode contains invalid character %q: %s", c, code)
			break
		}
	}

	// Same token should produce same short code (deterministic)
	code2 := gen.ShortCode(token)
	if code != code2 {
		t.Errorf("ShortCode not deterministic: %s != %s", code, code2)
	}

	// Different tokens should produce different short codes
	token2, _ := gen.Generate("alice@example.com", "bob@example.com")
	code3 := gen.ShortCode(token2)
	if code == code3 {
		t.Errorf("Different tokens produced same short code: %s", code)
	}

	t.Logf("Sample short code: %s", code)
}

func TestTokenEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gen, err := NewInviteTokenGenerator(key, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewInviteTokenGenerator failed: %v", err)
	}

	// Generate a token
	token, err := gen.Generate("alice@example.com", "bob@example.com")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	t.Run("encrypt and decrypt", func(t *testing.T) {
		// Encrypt for storage
		encrypted, err := gen.EncryptForStorage(token)
		if err != nil {
			t.Fatalf("EncryptForStorage failed: %v", err)
		}

		// Encrypted should be different from original
		if encrypted == token {
			t.Error("Encrypted token should differ from original")
		}

		t.Logf("Original token length: %d, Encrypted length: %d", len(token), len(encrypted))

		// Decrypt
		decrypted, err := gen.DecryptFromStorage(encrypted)
		if err != nil {
			t.Fatalf("DecryptFromStorage failed: %v", err)
		}

		// Should match original
		if decrypted != token {
			t.Errorf("Decrypted token doesn't match original")
		}

		// Decrypted token should still verify
		data, err := gen.Verify(decrypted)
		if err != nil {
			t.Fatalf("Verify decrypted token failed: %v", err)
		}
		if data.InviterEmail != "alice@example.com" {
			t.Errorf("InviterEmail = %q, want alice@example.com", data.InviterEmail)
		}
	})

	t.Run("different encryptions for same token", func(t *testing.T) {
		// Each encryption should produce different ciphertext (due to random nonce)
		enc1, _ := gen.EncryptForStorage(token)
		enc2, _ := gen.EncryptForStorage(token)

		if enc1 == enc2 {
			t.Error("Same token encrypted twice should produce different ciphertexts")
		}

		// But both should decrypt to the same value
		dec1, _ := gen.DecryptFromStorage(enc1)
		dec2, _ := gen.DecryptFromStorage(enc2)

		if dec1 != dec2 {
			t.Error("Different encryptions of same token should decrypt to same value")
		}
	})

	t.Run("tampered ciphertext rejected", func(t *testing.T) {
		encrypted, _ := gen.EncryptForStorage(token)

		// Tamper with the ciphertext
		tampered := []byte(encrypted)
		if len(tampered) > 10 {
			tampered[10] ^= 0xFF
		}

		_, err := gen.DecryptFromStorage(string(tampered))
		if err == nil {
			t.Error("Expected error for tampered ciphertext")
		}
	})

	t.Run("wrong key rejected", func(t *testing.T) {
		// Encrypt with original key
		encrypted, _ := gen.EncryptForStorage(token)

		// Create generator with different key
		otherKey := make([]byte, 32)
		for i := range otherKey {
			otherKey[i] = byte(i + 100)
		}
		otherGen, _ := NewInviteTokenGenerator(otherKey, 24*time.Hour)

		// Decryption with wrong key should fail
		_, err := otherGen.DecryptFromStorage(encrypted)
		if err == nil {
			t.Error("Expected error when decrypting with wrong key")
		}
	})
}
