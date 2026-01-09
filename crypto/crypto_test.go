package crypto

import (
	"encoding/base64"
	"testing"
)

func TestNewEncryptor_ValidKeys(t *testing.T) {
	testCases := []struct {
		name    string
		keySize int
	}{
		{"AES-128", 16},
		{"AES-192", 24},
		{"AES-256", 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := make([]byte, tc.keySize)
			enc, err := NewEncryptor(key)
			if err != nil {
				t.Fatalf("NewEncryptor failed: %v", err)
			}
			if enc == nil {
				t.Error("expected non-nil encryptor")
			}
		})
	}
}

func TestNewEncryptor_InvalidKeySize(t *testing.T) {
	testCases := []int{0, 1, 15, 17, 31, 33, 64}

	for _, size := range testCases {
		key := make([]byte, size)
		_, err := NewEncryptor(key)
		if err != ErrInvalidKey {
			t.Errorf("key size %d: expected ErrInvalidKey, got %v", size, err)
		}
	}
}

func TestNewEncryptorFromBase64_Valid(t *testing.T) {
	// Generate a valid 32-byte key
	key := make([]byte, 32)
	keyB64 := base64.StdEncoding.EncodeToString(key)

	enc, err := NewEncryptorFromBase64(keyB64)
	if err != nil {
		t.Fatalf("NewEncryptorFromBase64 failed: %v", err)
	}
	if enc == nil {
		t.Error("expected non-nil encryptor")
	}
}

func TestNewEncryptorFromBase64_InvalidBase64(t *testing.T) {
	_, err := NewEncryptorFromBase64("not-valid-base64!!!")
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestNewEncryptorFromBase64_InvalidKeySize(t *testing.T) {
	// 10 bytes is not a valid AES key size
	key := make([]byte, 10)
	keyB64 := base64.StdEncoding.EncodeToString(key)

	_, err := NewEncryptorFromBase64(keyB64)
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	plaintext := []byte("Hello, World!")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round trip failed: expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_ProducesDifferentCiphertext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	plaintext := []byte("Same message")
	ct1, _ := enc.Encrypt(plaintext)
	ct2, _ := enc.Encrypt(plaintext)

	// Same plaintext should produce different ciphertext due to random nonce
	if string(ct1) == string(ct2) {
		t.Error("same plaintext should produce different ciphertext")
	}
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	// Too short
	_, err := enc.Decrypt([]byte("short"))
	if err != ErrInvalidCiphertext {
		t.Errorf("expected ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	plaintext := []byte("Secret message")
	ciphertext, _ := enc.Encrypt(plaintext)

	// Tamper with ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := enc.Decrypt(ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1 // Different key

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	plaintext := []byte("Secret message")
	ciphertext, _ := enc1.Encrypt(plaintext)

	_, err := enc2.Decrypt(ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestEncryptString_DecryptString_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	plaintext := "Hello, World!"
	ciphertextB64, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("EncryptString failed: %v", err)
	}

	decrypted, err := enc.DecryptString(ciphertextB64)
	if err != nil {
		t.Fatalf("DecryptString failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("round trip failed: expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecryptString_InvalidBase64(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	_, err := enc.DecryptString("not-valid-base64!!!")
	if err != ErrInvalidCiphertext {
		t.Errorf("expected ErrInvalidCiphertext, got %v", err)
	}
}

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(key))
	}

	// Keys should be different
	key2, _ := GenerateKey()
	if string(key) == string(key2) {
		t.Error("generated keys should be different")
	}
}

func TestGenerateKeyBase64(t *testing.T) {
	keyB64, err := GenerateKeyBase64()
	if err != nil {
		t.Fatalf("GenerateKeyBase64 failed: %v", err)
	}

	// Should be valid base64
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		t.Fatalf("invalid base64: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(key))
	}
}

func TestGenerateKey_UsableForEncryption(t *testing.T) {
	key, _ := GenerateKey()
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}

	plaintext := []byte("Test message")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Error("round trip with generated key failed")
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	ciphertext, err := enc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("expected empty plaintext, got %q", decrypted)
	}
}

func TestEncrypt_LargePlaintext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptor(key)

	// 1MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if len(decrypted) != len(plaintext) {
		t.Errorf("length mismatch: expected %d, got %d", len(plaintext), len(decrypted))
	}
}
