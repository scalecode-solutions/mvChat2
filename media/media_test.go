package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewProcessor(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		p := NewProcessor(Config{
			UploadPath:    "/tmp/uploads",
			MaxUploadSize: 1024 * 1024,
		})
		if p.config.ThumbWidth != 256 {
			t.Errorf("expected ThumbWidth 256, got %d", p.config.ThumbWidth)
		}
		if p.config.ThumbHeight != 256 {
			t.Errorf("expected ThumbHeight 256, got %d", p.config.ThumbHeight)
		}
		if p.config.ThumbQuality != 80 {
			t.Errorf("expected ThumbQuality 80, got %d", p.config.ThumbQuality)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		p := NewProcessor(Config{
			UploadPath:    "/tmp/uploads",
			MaxUploadSize: 1024 * 1024,
			ThumbWidth:    128,
			ThumbHeight:   128,
			ThumbQuality:  90,
		})
		if p.config.ThumbWidth != 128 {
			t.Errorf("expected ThumbWidth 128, got %d", p.config.ThumbWidth)
		}
		if p.config.ThumbHeight != 128 {
			t.Errorf("expected ThumbHeight 128, got %d", p.config.ThumbHeight)
		}
		if p.config.ThumbQuality != 90 {
			t.Errorf("expected ThumbQuality 90, got %d", p.config.ThumbQuality)
		}
	})
}

func TestExtensionFromMime(t *testing.T) {
	tests := []struct {
		mime string
		ext  string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"video/mp4", ".mp4"},
		{"video/webm", ".webm"},
		{"video/quicktime", ".mov"},
		{"audio/mpeg", ".mp3"},
		{"audio/ogg", ".ogg"},
		{"audio/wav", ".wav"},
		{"audio/aac", ".aac"},
		{"application/pdf", ".pdf"},
		{"unknown/type", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := extensionFromMime(tt.mime)
			if got != tt.ext {
				t.Errorf("extensionFromMime(%q) = %q, want %q", tt.mime, got, tt.ext)
			}
		})
	}
}

func TestIsImage(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/gif", true},
		{"video/mp4", false},
		{"audio/mpeg", false},
		{"application/pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := IsImage(tt.mime)
			if got != tt.want {
				t.Errorf("IsImage(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

func TestIsVideo(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"video/mp4", true},
		{"video/webm", true},
		{"image/jpeg", false},
		{"audio/mpeg", false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := IsVideo(tt.mime)
			if got != tt.want {
				t.Errorf("IsVideo(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

func TestIsAudio(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"audio/mpeg", true},
		{"audio/ogg", true},
		{"image/jpeg", false},
		{"video/mp4", false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := IsAudio(tt.mime)
			if got != tt.want {
				t.Errorf("IsAudio(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

func TestGetFilePath(t *testing.T) {
	p := NewProcessor(Config{
		UploadPath: "/tmp/uploads",
	})

	fileID := uuid.MustParse("abcd1234-0000-0000-0000-000000000000")
	path := p.GetFilePath(fileID, "image/jpeg")

	if !strings.Contains(path, "ab") {
		t.Errorf("path should contain first 2 chars of ID: %s", path)
	}
	if !strings.Contains(path, "cd") {
		t.Errorf("path should contain chars 2-4 of ID: %s", path)
	}
	if !strings.HasSuffix(path, ".jpg") {
		t.Errorf("path should end with .jpg: %s", path)
	}
}

func TestSaveUpload(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "media-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewProcessor(Config{
		UploadPath: tmpDir,
	})

	data := []byte("test file content")
	fileID := uuid.New()

	path, err := p.SaveUpload(context.Background(), fileID, bytes.NewReader(data), "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("path should not be empty")
	}
}

func TestProcessImage(t *testing.T) {
	// Create a test PNG image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	tmpFile, err := os.CreateTemp("", "image-test-*.png")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if err := png.Encode(tmpFile, img); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	p := NewProcessor(Config{
		ThumbWidth:   50,
		ThumbHeight:  50,
		ThumbQuality: 80,
	})

	info, err := p.ProcessImage(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Width != 100 {
		t.Errorf("expected width 100, got %d", info.Width)
	}
	if info.Height != 100 {
		t.Errorf("expected height 100, got %d", info.Height)
	}
	if len(info.Thumbnail) == 0 {
		t.Error("expected thumbnail to be generated")
	}
}

func TestProcessImage_NotFound(t *testing.T) {
	p := NewProcessor(Config{})
	_, err := p.ProcessImage("/nonexistent/file.png")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSaveUploadFromPath(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "media-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "upload-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	data := []byte("test content")
	tmpFile.Write(data)
	tmpFile.Close()

	p := NewProcessor(Config{
		UploadPath: tmpDir,
	})

	fileID := uuid.New()
	path, err := p.SaveUploadFromPath(context.Background(), fileID, tmpPath, "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("path should not be empty")
	}

	// Verify new file exists
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != string(data) {
		t.Error("content mismatch")
	}
}

func TestCopyFile(t *testing.T) {
	// Create temp source file
	srcFile, err := os.CreateTemp("", "copy-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(srcFile.Name())

	data := []byte("content to copy")
	srcFile.Write(data)
	srcFile.Close()

	// Create temp destination path
	dstPath := filepath.Join(os.TempDir(), "copy-dst-"+uuid.New().String())
	defer os.Remove(dstPath)

	err = copyFile(srcFile.Name(), dstPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content was copied
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(content) != string(data) {
		t.Error("content mismatch")
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	err := copyFile("/nonexistent/source", "/tmp/dest")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestGenerateThumbnail_Landscape(t *testing.T) {
	// Create landscape image (wider than tall)
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))

	p := NewProcessor(Config{
		ThumbWidth:  50,
		ThumbHeight: 50,
	})

	thumb := p.generateThumbnail(img)
	if thumb == nil {
		t.Fatal("thumbnail should not be nil")
	}

	bounds := thumb.Bounds()
	// For a 200x100 image, fitting to 50x50, we should get 50x25
	if bounds.Dx() != 50 {
		t.Errorf("expected width 50, got %d", bounds.Dx())
	}
	if bounds.Dy() != 25 {
		t.Errorf("expected height 25, got %d", bounds.Dy())
	}
}

func TestGenerateThumbnail_Portrait(t *testing.T) {
	// Create portrait image (taller than wide)
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))

	p := NewProcessor(Config{
		ThumbWidth:  50,
		ThumbHeight: 50,
	})

	thumb := p.generateThumbnail(img)
	if thumb == nil {
		t.Fatal("thumbnail should not be nil")
	}

	bounds := thumb.Bounds()
	// For a 100x200 image, fitting to 50x50, we should get 25x50
	if bounds.Dx() != 25 {
		t.Errorf("expected width 25, got %d", bounds.Dx())
	}
	if bounds.Dy() != 50 {
		t.Errorf("expected height 50, got %d", bounds.Dy())
	}
}

func TestGenerateThumbnail_SmallImage(t *testing.T) {
	// Create small image (smaller than thumbnail size)
	img := image.NewRGBA(image.Rect(0, 0, 30, 30))

	p := NewProcessor(Config{
		ThumbWidth:  50,
		ThumbHeight: 50,
	})

	thumb := p.generateThumbnail(img)
	if thumb == nil {
		t.Fatal("thumbnail should not be nil")
	}

	bounds := thumb.Bounds()
	// Should not upscale
	if bounds.Dx() != 30 {
		t.Errorf("expected width 30 (no upscale), got %d", bounds.Dx())
	}
	if bounds.Dy() != 30 {
		t.Errorf("expected height 30 (no upscale), got %d", bounds.Dy())
	}
}
