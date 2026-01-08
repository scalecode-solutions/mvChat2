package media

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCalculateFileHash(t *testing.T) {
	// Create temp file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Known SHA-256 hash of "Hello, World!"
	expectedHash := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

	hash, err := CalculateFileHash(testFile)
	if err != nil {
		t.Fatalf("CalculateFileHash failed: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("hash mismatch: got %s, want %s", hash, expectedHash)
	}
}

func TestCalculateFileHash_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Known SHA-256 hash of empty string
	expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	hash, err := CalculateFileHash(testFile)
	if err != nil {
		t.Fatalf("CalculateFileHash failed: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("hash mismatch: got %s, want %s", hash, expectedHash)
	}
}

func TestCalculateFileHash_NonExistent(t *testing.T) {
	_, err := CalculateFileHash("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestSaveUploadWithHash(t *testing.T) {
	tmpDir := t.TempDir()

	proc := NewProcessor(Config{
		UploadPath: tmpDir,
	})

	// Create a reader with known content
	content := "test file content for hashing"
	reader := strings.NewReader(content)

	// Use a test UUID
	fileID := mustParseUUID("12345678-1234-1234-1234-123456789abc")

	path, hash, size, err := proc.SaveUploadWithHash(nil, fileID, reader, "text/plain")
	if err != nil {
		t.Fatalf("SaveUploadWithHash failed: %v", err)
	}

	// Verify size
	if size != int64(len(content)) {
		t.Errorf("size mismatch: got %d, want %d", size, len(content))
	}

	// Verify hash is a 64-char hex string
	if len(hash) != 64 {
		t.Errorf("hash length: got %d, want 64", len(hash))
	}

	// Verify file was written
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}

	// Verify hash matches recalculated hash
	recalcHash, err := CalculateFileHash(path)
	if err != nil {
		t.Fatalf("recalculate hash failed: %v", err)
	}

	if hash != recalcHash {
		t.Errorf("hash verification failed: streaming=%s, recalc=%s", hash, recalcHash)
	}
}

func TestSaveUploadWithHash_SameContentSameHash(t *testing.T) {
	tmpDir := t.TempDir()

	proc := NewProcessor(Config{
		UploadPath: tmpDir,
	})

	content := "identical content for both uploads"

	// Upload first file
	fileID1 := mustParseUUID("aaaaaaaa-1111-1111-1111-111111111111")
	_, hash1, _, err := proc.SaveUploadWithHash(nil, fileID1, strings.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}

	// Upload second file with same content
	fileID2 := mustParseUUID("bbbbbbbb-2222-2222-2222-222222222222")
	_, hash2, _, err := proc.SaveUploadWithHash(nil, fileID2, strings.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}

	// Hashes should be identical
	if hash1 != hash2 {
		t.Errorf("same content should produce same hash: got %s and %s", hash1, hash2)
	}
}

func TestSaveUploadWithHash_DifferentContentDifferentHash(t *testing.T) {
	tmpDir := t.TempDir()

	proc := NewProcessor(Config{
		UploadPath: tmpDir,
	})

	// Upload first file
	fileID1 := mustParseUUID("cccccccc-1111-1111-1111-111111111111")
	_, hash1, _, err := proc.SaveUploadWithHash(nil, fileID1, strings.NewReader("content A"), "text/plain")
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}

	// Upload second file with different content
	fileID2 := mustParseUUID("dddddddd-2222-2222-2222-222222222222")
	_, hash2, _, err := proc.SaveUploadWithHash(nil, fileID2, strings.NewReader("content B"), "text/plain")
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}

	// Hashes should be different
	if hash1 == hash2 {
		t.Errorf("different content should produce different hash: both are %s", hash1)
	}
}

func TestDeleteFile(t *testing.T) {
	tmpDir := t.TempDir()

	proc := NewProcessor(Config{
		UploadPath: tmpDir,
	})

	// Create a file first
	fileID := mustParseUUID("eeeeeeee-1111-1111-1111-111111111111")
	path, _, _, err := proc.SaveUploadWithHash(nil, fileID, bytes.NewReader([]byte("test")), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file should exist before delete")
	}

	// Delete the file
	if err := proc.DeleteFile(fileID, "text/plain"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after delete")
	}
}

// mustParseUUID is a test helper.
func mustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
