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

		// Token should be URL-safe base64
		if strings.ContainsAny(token, "+/") {
			t.Errorf("Token contains non-URL-safe characters: %s", token)
		}

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
		longEmail := strings.Repeat("a", 200) + "@example.com"
		token, err := gen.Generate(longEmail, longEmail)
		if err != nil {
			t.Fatalf("Generate with long emails failed: %v", err)
		}

		data, err := gen.Verify(token)
		if err != nil {
			t.Fatalf("Verify with long emails failed: %v", err)
		}

		if data.InviterEmail != longEmail || data.InviteeEmail != longEmail {
			t.Error("Long emails not preserved")
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
		{"not base64", "not-valid-base64!@#"},
		{"too short", "AAAA"},
		{"truncated", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
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
