package irido

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	i := New("Hello world")
	if i.V != 1 {
		t.Errorf("expected V=1, got %d", i.V)
	}
	if i.Text != "Hello world" {
		t.Errorf("expected text 'Hello world', got '%s'", i.Text)
	}
}

func TestIsIrido(t *testing.T) {
	tests := []struct {
		name     string
		content  any
		expected bool
	}{
		{"nil", nil, false},
		{"plain string", "hello", false},
		{"map with v:1", map[string]any{"v": 1, "text": "hi"}, true},
		{"map with v:1 float", map[string]any{"v": 1.0, "text": "hi"}, true},
		{"map with v:2", map[string]any{"v": 2, "text": "hi"}, false},
		{"map without v", map[string]any{"text": "hi"}, false},
		{"Irido struct", &Irido{V: 1, Text: "hi"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIrido(tt.content); got != tt.expected {
				t.Errorf("IsIrido() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Run("plain string", func(t *testing.T) {
		i, err := Parse("hello")
		if err != nil {
			t.Fatal(err)
		}
		if i.V != 1 || i.Text != "hello" {
			t.Errorf("unexpected result: %+v", i)
		}
	})

	t.Run("JSON bytes", func(t *testing.T) {
		data := []byte(`{"v":1,"text":"hello","media":[{"type":"image","ref":"abc123"}]}`)
		i, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		if i.Text != "hello" {
			t.Errorf("expected text 'hello', got '%s'", i.Text)
		}
		if len(i.Media) != 1 {
			t.Errorf("expected 1 media, got %d", len(i.Media))
		}
		if i.Media[0].Type != "image" {
			t.Errorf("expected media type 'image', got '%s'", i.Media[0].Type)
		}
	})

	t.Run("map with mentions", func(t *testing.T) {
		content := map[string]any{
			"v":    1,
			"text": "Hello @user!",
			"mentions": []any{
				map[string]any{
					"userId":   "user123",
					"username": "user",
					"offset":   6.0,
					"length":   5.0,
				},
			},
		}
		i, err := Parse(content)
		if err != nil {
			t.Fatal(err)
		}
		if len(i.Mentions) != 1 {
			t.Fatalf("expected 1 mention, got %d", len(i.Mentions))
		}
		if i.Mentions[0].UserID != "user123" {
			t.Errorf("expected userId 'user123', got '%s'", i.Mentions[0].UserID)
		}
		if i.Mentions[0].Offset != 6 {
			t.Errorf("expected offset 6, got %d", i.Mentions[0].Offset)
		}
	})

	t.Run("map with reply", func(t *testing.T) {
		content := map[string]any{
			"v":    1,
			"text": "This is a reply",
			"reply": map[string]any{
				"seq":     42.0,
				"preview": "Original message",
				"from":    "user456",
			},
		}
		i, err := Parse(content)
		if err != nil {
			t.Fatal(err)
		}
		if i.Reply == nil {
			t.Fatal("expected reply, got nil")
		}
		if i.Reply.Seq != 42 {
			t.Errorf("expected seq 42, got %d", i.Reply.Seq)
		}
		if i.Reply.From != "user456" {
			t.Errorf("expected from 'user456', got '%s'", i.Reply.From)
		}
	})
}

func TestPlainText(t *testing.T) {
	tests := []struct {
		name     string
		content  any
		expected string
	}{
		{"plain string", "hello", "hello"},
		{"text only", map[string]any{"v": 1, "text": "hello world"}, "hello world"},
		{"text with image", map[string]any{
			"v":     1,
			"text":  "Check this out",
			"media": []any{map[string]any{"type": "image", "name": "photo.jpg"}},
		}, "Check this out [IMAGE 'photo.jpg']"},
		{"media only", map[string]any{
			"v":     1,
			"media": []any{map[string]any{"type": "video", "name": "clip.mp4"}},
		}, "[VIDEO 'clip.mp4']"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PlainText(tt.content)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.expected {
				t.Errorf("PlainText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPreview(t *testing.T) {
	t.Run("short text", func(t *testing.T) {
		got, err := Preview("hello", 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != "hello" {
			t.Errorf("expected 'hello', got '%s'", got)
		}
	})

	t.Run("long text truncated", func(t *testing.T) {
		got, err := Preview("hello world this is a long message", 10)
		if err != nil {
			t.Fatal(err)
		}
		if got != "hello worl…" {
			t.Errorf("expected 'hello worl…', got '%s'", got)
		}
	})

	t.Run("emoji handling", func(t *testing.T) {
		// 👨‍👩‍👧‍👦 is a single grapheme cluster (family emoji)
		got, err := Preview("Hi 👨‍👩‍👧‍👦!", 5)
		if err != nil {
			t.Fatal(err)
		}
		// Should be "Hi 👨‍👩‍👧‍👦!" (5 graphemes: H, i, space, family, !)
		if got != "Hi 👨‍👩‍👧‍👦!" {
			t.Errorf("expected 'Hi 👨‍👩‍👧‍👦!', got '%s'", got)
		}
	})
}

func TestGetFileRefs(t *testing.T) {
	content := map[string]any{
		"v":    1,
		"text": "Multiple files",
		"media": []any{
			map[string]any{"type": "image", "ref": "file1"},
			map[string]any{"type": "video", "ref": "file2"},
			map[string]any{"type": "embed"}, // No ref
		},
	}

	refs := GetFileRefs(content)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0] != "file1" || refs[1] != "file2" {
		t.Errorf("unexpected refs: %v", refs)
	}
}

func TestGetMentionedUsers(t *testing.T) {
	content := map[string]any{
		"v":    1,
		"text": "@alice and @bob",
		"mentions": []any{
			map[string]any{"userId": "user1", "username": "alice", "offset": 0.0, "length": 6.0},
			map[string]any{"userId": "user2", "username": "bob", "offset": 11.0, "length": 4.0},
		},
	}

	users := GetMentionedUsers(content)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0] != "user1" || users[1] != "user2" {
		t.Errorf("unexpected users: %v", users)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		content any
		wantErr bool
	}{
		{"valid text", map[string]any{"v": 1, "text": "hello"}, false},
		{"valid media", map[string]any{"v": 1, "media": []any{map[string]any{"type": "image"}}}, false},
		{"empty content", map[string]any{"v": 1}, true},
		{"too many media", map[string]any{
			"v": 1,
			"media": []any{
				map[string]any{"type": "image"}, map[string]any{"type": "image"},
				map[string]any{"type": "image"}, map[string]any{"type": "image"},
				map[string]any{"type": "image"}, map[string]any{"type": "image"},
				map[string]any{"type": "image"}, map[string]any{"type": "image"},
				map[string]any{"type": "image"}, map[string]any{"type": "image"},
				map[string]any{"type": "image"}, // 11th
			},
		}, true},
		{"mentions without text", map[string]any{
			"v":        1,
			"media":    []any{map[string]any{"type": "image"}},
			"mentions": []any{map[string]any{"userId": "user1"}},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	i := &Irido{
		V:    1,
		Text: "Hello",
		Media: []Media{
			{Type: "image", Ref: "abc123", Width: 800, Height: 600},
		},
	}

	data, err := i.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Parse back
	var parsed Irido
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.Text != "Hello" {
		t.Errorf("expected text 'Hello', got '%s'", parsed.Text)
	}
	if len(parsed.Media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(parsed.Media))
	}
	if parsed.Media[0].Width != 800 {
		t.Errorf("expected width 800, got %d", parsed.Media[0].Width)
	}
}

func TestNewWithMedia(t *testing.T) {
	media := []Media{
		{Type: "image", Ref: "abc123", Name: "photo.jpg"},
		{Type: "video", Ref: "def456", Name: "clip.mp4"},
	}
	i := NewWithMedia("Check this out", media)

	if i.V != 1 {
		t.Errorf("expected V=1, got %d", i.V)
	}
	if i.Text != "Check this out" {
		t.Errorf("expected text 'Check this out', got '%s'", i.Text)
	}
	if len(i.Media) != 2 {
		t.Errorf("expected 2 media, got %d", len(i.Media))
	}
}

func TestString(t *testing.T) {
	i := &Irido{V: 1, Text: "Hello"}
	s := i.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Errorf("String() should return valid JSON: %v", err)
	}
}

func TestHasMedia(t *testing.T) {
	tests := []struct {
		name     string
		content  any
		expected bool
	}{
		{"no media", map[string]any{"v": 1, "text": "hello"}, false},
		{"with media", map[string]any{
			"v":     1,
			"text":  "check this",
			"media": []any{map[string]any{"type": "image"}},
		}, true},
		{"empty media", map[string]any{"v": 1, "text": "hello", "media": []any{}}, false},
		{"nil content", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasMedia(tt.content)
			if got != tt.expected {
				t.Errorf("HasMedia() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsReply(t *testing.T) {
	tests := []struct {
		name     string
		content  any
		expected bool
	}{
		{"not a reply", map[string]any{"v": 1, "text": "hello"}, false},
		{"is a reply", map[string]any{
			"v":     1,
			"text":  "reply text",
			"reply": map[string]any{"seq": 42.0},
		}, true},
		{"nil content", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReply(tt.content)
			if got != tt.expected {
				t.Errorf("IsReply() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPreviewIrido(t *testing.T) {
	t.Run("text only", func(t *testing.T) {
		content := map[string]any{"v": 1, "text": "Hello world this is a test"}
		preview, err := PreviewIrido(content, 10)
		if err != nil {
			t.Fatal(err)
		}
		if preview == nil {
			t.Fatal("expected preview")
		}
		if len(preview.Text) > 15 {
			t.Errorf("text should be truncated, got '%s'", preview.Text)
		}
	})

	t.Run("with media", func(t *testing.T) {
		content := map[string]any{
			"v":    1,
			"text": "Check this out",
			"media": []any{
				map[string]any{"type": "image", "ref": "abc"},
			},
		}
		preview, err := PreviewIrido(content, 100)
		if err != nil {
			t.Fatal(err)
		}
		if preview == nil {
			t.Fatal("expected preview")
		}
		if len(preview.Media) != 1 {
			t.Errorf("expected 1 media, got %d", len(preview.Media))
		}
	})

	t.Run("with reply", func(t *testing.T) {
		content := map[string]any{
			"v":     1,
			"text":  "Reply text",
			"reply": map[string]any{"seq": 42.0, "from": "user123", "preview": "Original very long text that should be truncated"},
		}
		preview, err := PreviewIrido(content, 100)
		if err != nil {
			t.Fatal(err)
		}
		if preview == nil {
			t.Fatal("expected preview")
		}
		if preview.Reply == nil {
			t.Fatal("expected reply to be preserved")
		}
	})

	t.Run("nil content", func(t *testing.T) {
		preview, err := PreviewIrido(nil, 100)
		if err != nil {
			t.Fatal(err)
		}
		if preview != nil {
			t.Error("expected nil for nil content")
		}
	})
}

func TestParseWithEmbed(t *testing.T) {
	content := map[string]any{
		"v":    1,
		"text": "Check this link",
		"media": []any{
			map[string]any{
				"type": "embed",
				"embed": map[string]any{
					"url":         "https://example.com",
					"title":       "Example Site",
					"description": "An example website",
					"thumbnail":   "https://example.com/thumb.jpg",
					"siteName":    "Example",
					"embedType":   "article",
				},
			},
		},
	}

	i, err := Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(i.Media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(i.Media))
	}
	if i.Media[0].Embed == nil {
		t.Fatal("expected embed data")
	}
	if i.Media[0].Embed.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got '%s'", i.Media[0].Embed.URL)
	}
	if i.Media[0].Embed.Title != "Example Site" {
		t.Errorf("expected title 'Example Site', got '%s'", i.Media[0].Embed.Title)
	}
	if i.Media[0].Embed.EmbedType != "article" {
		t.Errorf("expected embedType 'article', got '%s'", i.Media[0].Embed.EmbedType)
	}
}

func TestParseWithFullMediaInfo(t *testing.T) {
	content := map[string]any{
		"v":    1,
		"text": "Video",
		"media": []any{
			map[string]any{
				"type":     "video",
				"ref":      "video123",
				"name":     "clip.mp4",
				"mime":     "video/mp4",
				"size":     1048576.0,
				"width":    1920.0,
				"height":   1080.0,
				"duration": 120.5,
			},
		},
	}

	i, err := Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(i.Media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(i.Media))
	}
	m := i.Media[0]
	if m.Type != "video" {
		t.Errorf("expected type 'video', got '%s'", m.Type)
	}
	if m.Size != 1048576 {
		t.Errorf("expected size 1048576, got %d", m.Size)
	}
	if m.Width != 1920 {
		t.Errorf("expected width 1920, got %d", m.Width)
	}
	if m.Height != 1080 {
		t.Errorf("expected height 1080, got %d", m.Height)
	}
	if m.Duration != 120.5 {
		t.Errorf("expected duration 120.5, got %f", m.Duration)
	}
}

func TestParse_Errors(t *testing.T) {
	t.Run("nil content", func(t *testing.T) {
		i, err := Parse(nil)
		if err != nil {
			t.Errorf("nil should not error: %v", err)
		}
		if i != nil {
			t.Error("expected nil result for nil input")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := Parse(12345)
		if err != ErrInvalidContent {
			t.Errorf("expected ErrInvalidContent, got %v", err)
		}
	})

	t.Run("map without v", func(t *testing.T) {
		_, err := Parse(map[string]any{"text": "hello"})
		if err != ErrNotIrido {
			t.Errorf("expected ErrNotIrido, got %v", err)
		}
	})

	t.Run("wrong version", func(t *testing.T) {
		_, err := Parse(map[string]any{"v": 2, "text": "hello"})
		if err != ErrInvalidContent {
			t.Errorf("expected ErrInvalidContent, got %v", err)
		}
	})

	t.Run("invalid v type", func(t *testing.T) {
		_, err := Parse(map[string]any{"v": "1", "text": "hello"})
		if err != ErrInvalidContent {
			t.Errorf("expected ErrInvalidContent, got %v", err)
		}
	})

	t.Run("JSON with wrong version", func(t *testing.T) {
		data := []byte(`{"v":2,"text":"hello"}`)
		_, err := Parse(data)
		if err != ErrInvalidContent {
			t.Errorf("expected ErrInvalidContent, got %v", err)
		}
	})

	t.Run("Irido struct directly", func(t *testing.T) {
		i := Irido{V: 1, Text: "hello"}
		result, err := Parse(i)
		if err != nil {
			t.Fatal(err)
		}
		if result.Text != "hello" {
			t.Errorf("expected text 'hello', got '%s'", result.Text)
		}
	})

	t.Run("Irido pointer directly", func(t *testing.T) {
		i := &Irido{V: 1, Text: "hello"}
		result, err := Parse(i)
		if err != nil {
			t.Fatal(err)
		}
		if result != i {
			t.Error("expected same pointer back")
		}
	})
}

func TestPlainText_MediaTypes(t *testing.T) {
	tests := []struct {
		name     string
		media    map[string]any
		expected string
	}{
		{"audio", map[string]any{"type": "audio", "name": "song.mp3"}, "[AUDIO 'song.mp3']"},
		{"file", map[string]any{"type": "file", "name": "doc.pdf"}, "[FILE 'doc.pdf']"},
		{"unknown type", map[string]any{"type": "unknown", "name": "thing"}, "[UNKNOWN 'thing']"},
		{"no name", map[string]any{"type": "image"}, "[IMAGE 'attachment']"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := map[string]any{
				"v":     1,
				"media": []any{tt.media},
			}
			got, err := PlainText(content)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.expected {
				t.Errorf("PlainText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetFileRefs_EdgeCases(t *testing.T) {
	t.Run("nil content", func(t *testing.T) {
		refs := GetFileRefs(nil)
		if refs != nil {
			t.Errorf("expected nil, got %v", refs)
		}
	})

	t.Run("non-irido content", func(t *testing.T) {
		refs := GetFileRefs("plain string")
		if refs != nil {
			t.Errorf("expected nil, got %v", refs)
		}
	})
}

func TestGetMentionedUsers_EdgeCases(t *testing.T) {
	t.Run("nil content", func(t *testing.T) {
		users := GetMentionedUsers(nil)
		if users != nil {
			t.Errorf("expected nil, got %v", users)
		}
	})

	t.Run("no mentions", func(t *testing.T) {
		users := GetMentionedUsers(map[string]any{"v": 1, "text": "hello"})
		if users != nil {
			t.Errorf("expected nil, got %v", users)
		}
	})
}

func TestIsIrido_StructValue(t *testing.T) {
	// Test non-pointer Irido struct
	i := Irido{V: 1, Text: "hello"}
	if !IsIrido(i) {
		t.Error("IsIrido should return true for Irido struct value")
	}
}

func TestMediaDescription_EmbedCases(t *testing.T) {
	t.Run("embed with title", func(t *testing.T) {
		content := map[string]any{
			"v": 1,
			"media": []any{map[string]any{
				"type": "embed",
				"embed": map[string]any{
					"title": "My Webpage",
					"url":   "https://example.com",
				},
			}},
		}
		got, err := PlainText(content)
		if err != nil {
			t.Fatal(err)
		}
		if got != "[LINK 'My Webpage']" {
			t.Errorf("expected [LINK 'My Webpage'], got %q", got)
		}
	})

	t.Run("embed with URL only", func(t *testing.T) {
		content := map[string]any{
			"v": 1,
			"media": []any{map[string]any{
				"type": "embed",
				"embed": map[string]any{
					"url": "https://example.com",
				},
			}},
		}
		got, err := PlainText(content)
		if err != nil {
			t.Fatal(err)
		}
		if got != "[LINK 'https://example.com']" {
			t.Errorf("expected [LINK 'https://example.com'], got %q", got)
		}
	})
}

func TestPreview_EdgeCases(t *testing.T) {
	t.Run("media only no text", func(t *testing.T) {
		content := map[string]any{
			"v": 1,
			"media": []any{map[string]any{
				"type": "image",
				"name": "photo.jpg",
			}},
		}
		got, err := Preview(content, 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != "[IMAGE 'photo.jpg']" {
			t.Errorf("expected [IMAGE 'photo.jpg'], got %q", got)
		}
	})

	t.Run("nil content", func(t *testing.T) {
		got, err := Preview(nil, 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

func TestPreviewIrido_LongReply(t *testing.T) {
	// Test truncation of long reply preview
	longReply := "This is a very long reply text that exceeds fifty characters and should be truncated"
	content := map[string]any{
		"v":    1,
		"text": "short text",
		"reply": map[string]any{
			"seq":     1,
			"preview": longReply,
			"from":    "user123",
		},
	}
	result, err := PreviewIrido(content, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Reply == nil {
		t.Fatal("expected reply, got nil")
	}
	// Ellipsis is 3 bytes (UTF-8), so 50 chars + 3 bytes = 53 bytes max
	if len(result.Reply.Preview) > 53 {
		t.Errorf("reply preview should be truncated, got %d bytes", len(result.Reply.Preview))
	}
	if !strings.Contains(result.Reply.Preview, "…") {
		t.Error("truncated reply should end with ellipsis")
	}
}

func TestValidate_EdgeCases(t *testing.T) {
	t.Run("invalid content type", func(t *testing.T) {
		err := Validate(123) // invalid type
		if err == nil {
			t.Error("expected error for invalid content type")
		}
	})
}

func TestPlainText_ErrorCase(t *testing.T) {
	// Test with content that causes Parse to error
	content := map[string]any{
		"v": 2, // wrong version
	}
	_, err := PlainText(content)
	if err == nil {
		t.Error("expected error for wrong version")
	}
}

func TestPreview_ErrorCase(t *testing.T) {
	// Test with content that causes Parse to error
	content := map[string]any{
		"v": 2, // wrong version
	}
	_, err := Preview(content, 100)
	if err == nil {
		t.Error("expected error for wrong version")
	}
}

func TestPreviewIrido_ErrorCase(t *testing.T) {
	// Test with content that causes Parse to error
	content := map[string]any{
		"v": 2, // wrong version
	}
	_, err := PreviewIrido(content, 100)
	if err == nil {
		t.Error("expected error for wrong version")
	}
}

func TestParse_BytesFallback(t *testing.T) {
	// Test []byte that isn't valid JSON - falls back to plain text
	content := []byte("plain text message")
	result, err := Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "plain text message" {
		t.Errorf("expected 'plain text message', got %q", result.Text)
	}
}

func TestParse_IridoStructValue(t *testing.T) {
	// Test non-pointer Irido struct
	content := Irido{V: 1, Text: "hello"}
	result, err := Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "hello" {
		t.Errorf("expected 'hello', got %q", result.Text)
	}
}

func TestValidate_NilContent(t *testing.T) {
	err := Validate(nil)
	if err != ErrInvalidContent {
		t.Errorf("expected ErrInvalidContent, got %v", err)
	}
}
