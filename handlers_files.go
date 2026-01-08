package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/media"
	"github.com/scalecode-solutions/mvchat2/store"
)

// FileHandlers handles file upload/download HTTP requests.
type FileHandlers struct {
	db        *store.DB
	processor *media.Processor
	auth      AuthValidator
}

// AuthValidator validates auth tokens for HTTP requests.
type AuthValidator interface {
	ValidateToken(token string) (uuid.UUID, error)
}

// NewFileHandlers creates a new file handlers instance.
func NewFileHandlers(db *store.DB, processor *media.Processor, auth AuthValidator) *FileHandlers {
	return &FileHandlers{
		db:        db,
		processor: processor,
		auth:      auth,
	}
}

// SetupRoutes adds file routes to the mux.
func (fh *FileHandlers) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v0/file/upload", fh.handleUpload)
	mux.HandleFunc("/v0/file/", fh.handleDownload)
}

// handleUpload handles file uploads.
func (fh *FileHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate
	userID, err := fh.authenticateRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form (32MB max in memory)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get mime type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		// Detect from first 512 bytes
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		mimeType = http.DetectContentType(buf[:n])
		file.Seek(0, 0)
	}

	// Create file record
	fileID := uuid.New()
	fileRecord, err := fh.db.CreateFile(r.Context(), userID, mimeType, header.Size, "")
	if err != nil {
		http.Error(w, "failed to create file record", http.StatusInternalServerError)
		return
	}
	fileID = fileRecord.ID

	// Save file to disk
	path, err := fh.processor.SaveUpload(r.Context(), fileID, file, mimeType)
	if err != nil {
		fh.db.UpdateFileStatus(r.Context(), fileID, "failed")
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	// Process media (extract metadata, generate thumbnail)
	go fh.processMedia(fileID, path, mimeType)

	// Return file info
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"id":"%s","mime":"%s","size":%d}`, fileID, mimeType, header.Size)
}

// processMedia processes uploaded media in the background.
func (fh *FileHandlers) processMedia(fileID uuid.UUID, path, mimeType string) {
	ctx := context.Background()

	var info *media.MediaInfo
	var err error

	if media.IsImage(mimeType) {
		info, err = fh.processor.ProcessImage(path)
	} else if media.IsVideo(mimeType) {
		info, err = fh.processor.ProcessVideo(path)
	} else if media.IsAudio(mimeType) {
		info, err = fh.processor.ProcessAudio(path)
	}

	if err != nil {
		fh.db.UpdateFileStatus(ctx, fileID, "failed")
		return
	}

	// Save metadata
	if info != nil {
		var width, height *int
		var duration *float64
		var thumbnail *string

		if info.Width > 0 {
			width = &info.Width
		}
		if info.Height > 0 {
			height = &info.Height
		}
		if info.Duration > 0 {
			duration = &info.Duration
		}
		if len(info.Thumbnail) > 0 {
			thumbB64 := base64.StdEncoding.EncodeToString(info.Thumbnail)
			thumbnail = &thumbB64
		}

		fh.db.CreateFileMetadata(ctx, fileID, width, height, duration, thumbnail, nil)
	}

	fh.db.UpdateFileStatus(ctx, fileID, "ready")
}

// handleDownload handles file downloads.
func (fh *FileHandlers) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract file ID from path: /v0/file/{id}
	path := strings.TrimPrefix(r.URL.Path, "/v0/file/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing file id", http.StatusBadRequest)
		return
	}

	fileID, err := uuid.Parse(parts[0])
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	// Check if requesting thumbnail
	isThumbnail := len(parts) > 1 && parts[1] == "thumb"

	// Authenticate
	userID, err := fh.authenticateRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check access
	canAccess, err := fh.db.CanAccessFile(r.Context(), fileID, userID)
	if err != nil || !canAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Get file info
	fileWithMeta, err := fh.db.GetFileWithMetadata(r.Context(), fileID)
	if err != nil || fileWithMeta == nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	if fileWithMeta.Status != "ready" {
		http.Error(w, "file not ready", http.StatusAccepted)
		return
	}

	// Return thumbnail if requested
	if isThumbnail && fileWithMeta.Metadata != nil && fileWithMeta.Metadata.Thumbnail != nil {
		thumbData, err := base64.StdEncoding.DecodeString(*fileWithMeta.Metadata.Thumbnail)
		if err == nil {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", strconv.Itoa(len(thumbData)))
			w.Header().Set("Cache-Control", "public, max-age=31536000")
			w.Write(thumbData)
			return
		}
	}

	// Serve file
	filePath := fh.processor.GetFilePath(fileID, fileWithMeta.MimeType)
	w.Header().Set("Content-Type", fileWithMeta.MimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, filePath)
}

// authenticateRequest extracts and validates the auth token from a request.
func (fh *FileHandlers) authenticateRequest(r *http.Request) (uuid.UUID, error) {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		return fh.auth.ValidateToken(token)
	}

	// Check query param
	token := r.URL.Query().Get("token")
	if token != "" {
		return fh.auth.ValidateToken(token)
	}

	return uuid.Nil, fmt.Errorf("no auth token")
}
