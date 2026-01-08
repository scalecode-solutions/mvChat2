package irido

import (
	"encoding/json"
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
