// Package media handles file processing, thumbnails, and media metadata extraction.
package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

// Config holds media processing configuration.
type Config struct {
	UploadPath    string
	MaxUploadSize int64
	ThumbWidth    int
	ThumbHeight   int
	ThumbQuality  int
}

// Processor handles media file processing.
type Processor struct {
	config Config
}

// NewProcessor creates a new media processor.
func NewProcessor(cfg Config) *Processor {
	if cfg.ThumbWidth == 0 {
		cfg.ThumbWidth = 256
	}
	if cfg.ThumbHeight == 0 {
		cfg.ThumbHeight = 256
	}
	if cfg.ThumbQuality == 0 {
		cfg.ThumbQuality = 80
	}
	return &Processor{config: cfg}
}

// MediaInfo holds extracted media information.
type MediaInfo struct {
	Width     int
	Height    int
	Duration  float64 // seconds, for video/audio
	Thumbnail []byte  // JPEG thumbnail data
}

// SaveUpload saves an uploaded file and returns the storage path.
func (p *Processor) SaveUpload(ctx context.Context, fileID uuid.UUID, data io.Reader, mimeType string) (string, error) {
	// Create directory structure: uploads/ab/cd/abcd1234-...
	idStr := fileID.String()
	dir := filepath.Join(p.config.UploadPath, idStr[:2], idStr[2:4])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Determine extension from mime type
	ext := extensionFromMime(mimeType)
	filename := idStr + ext
	path := filepath.Join(dir, filename)

	// Write file
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return path, nil
}

// ProcessImage extracts info and generates thumbnail for an image.
func (p *Processor) ProcessImage(path string) (*MediaInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	info := &MediaInfo{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}

	// Generate thumbnail
	thumb := p.generateThumbnail(img)
	if thumb != nil {
		var buf bytes.Buffer
		if format == "png" {
			png.Encode(&buf, thumb)
		} else {
			jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: p.config.ThumbQuality})
		}
		info.Thumbnail = buf.Bytes()
	}

	return info, nil
}

// ProcessVideo extracts info and generates thumbnail for a video using FFmpeg.
func (p *Processor) ProcessVideo(path string) (*MediaInfo, error) {
	info := &MediaInfo{}

	// Get video info using ffprobe
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration",
		"-of", "csv=p=0",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) >= 2 {
		info.Width, _ = strconv.Atoi(parts[0])
		info.Height, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		info.Duration, _ = strconv.ParseFloat(parts[2], 64)
	}

	// Generate thumbnail at 1 second mark (or 0 if shorter)
	thumbPath := path + ".thumb.jpg"
	defer os.Remove(thumbPath)

	seekTime := "00:00:01"
	if info.Duration < 1 {
		seekTime = "00:00:00"
	}

	cmd = exec.Command("ffmpeg",
		"-y",
		"-ss", seekTime,
		"-i", path,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", p.config.ThumbWidth, p.config.ThumbHeight),
		"-q:v", "2",
		thumbPath,
	)
	if err := cmd.Run(); err == nil {
		thumbData, err := os.ReadFile(thumbPath)
		if err == nil {
			info.Thumbnail = thumbData
		}
	}

	return info, nil
}

// ProcessAudio extracts duration from an audio file using FFmpeg.
func (p *Processor) ProcessAudio(path string) (*MediaInfo, error) {
	info := &MediaInfo{}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	info.Duration, _ = strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return info, nil
}

// generateThumbnail creates a thumbnail from an image.
func (p *Processor) generateThumbnail(img image.Image) image.Image {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()

	// Calculate thumbnail dimensions maintaining aspect ratio
	thumbW, thumbH := p.config.ThumbWidth, p.config.ThumbHeight
	if srcW > srcH {
		thumbH = srcH * thumbW / srcW
	} else {
		thumbW = srcW * thumbH / srcH
	}

	// Don't upscale
	if thumbW > srcW {
		thumbW = srcW
	}
	if thumbH > srcH {
		thumbH = srcH
	}

	thumb := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, bounds, draw.Over, nil)
	return thumb
}

// GetFilePath returns the full path for a file ID.
func (p *Processor) GetFilePath(fileID uuid.UUID, mimeType string) string {
	idStr := fileID.String()
	ext := extensionFromMime(mimeType)
	return filepath.Join(p.config.UploadPath, idStr[:2], idStr[2:4], idStr+ext)
}

func extensionFromMime(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav":
		return ".wav"
	case "audio/aac":
		return ".aac"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

// IsImage checks if a mime type is an image.
func IsImage(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// IsVideo checks if a mime type is a video.
func IsVideo(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

// IsAudio checks if a mime type is audio.
func IsAudio(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/")
}
