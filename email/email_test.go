package email

import (
	"net/smtp"
	"testing"
)

func TestSanitizeForHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal text",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "CR injection attempt",
			input: "Hello\rBcc: attacker@evil.com",
			want:  "HelloBcc: attacker@evil.com",
		},
		{
			name:  "LF injection attempt",
			input: "Hello\nBcc: attacker@evil.com",
			want:  "HelloBcc: attacker@evil.com",
		},
		{
			name:  "CRLF injection attempt",
			input: "Hello\r\nBcc: attacker@evil.com",
			want:  "HelloBcc: attacker@evil.com",
		},
		{
			name:  "null byte injection",
			input: "Hello\x00World",
			want:  "HelloWorld",
		},
		{
			name:  "whitespace trimming",
			input: "  Hello  ",
			want:  "Hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only injection chars",
			input: "\r\n\r\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeForHeader(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeForHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeForHeader_LengthLimit(t *testing.T) {
	// Create a string longer than 200 chars
	long := ""
	for i := 0; i < 300; i++ {
		long += "a"
	}

	result := sanitizeForHeader(long)
	if len(result) > 200 {
		t.Errorf("sanitizeForHeader should limit to 200 chars, got %d", len(result))
	}
}

func TestSanitizeForDisplay(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal text",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "control characters removed",
			input: "Hello\x01\x02\x03World",
			want:  "HelloWorld",
		},
		{
			name:  "tab preserved",
			input: "Hello\tWorld",
			want:  "Hello\tWorld",
		},
		{
			name:  "newlines removed",
			input: "Hello\nWorld",
			want:  "HelloWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeForDisplay(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeForDisplay(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeForDisplay_LengthLimit(t *testing.T) {
	// Create a string longer than 200 chars
	long := ""
	for i := 0; i < 300; i++ {
		long += "a"
	}

	result := sanitizeForDisplay(long)
	if len(result) > 200 {
		t.Errorf("sanitizeForDisplay should limit to 200 chars, got %d", len(result))
	}
}

func TestNew(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user",
		Password: "pass",
		From:     "from@example.com",
		FromName: "Test Sender",
		BaseURL:  "https://test.example.com",
	}

	s := New(cfg)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if !s.IsEnabled() {
		t.Error("expected service to be enabled")
	}
}

func TestIsEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		s := New(Config{Enabled: true})
		if !s.IsEnabled() {
			t.Error("expected true")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		s := New(Config{Enabled: false})
		if s.IsEnabled() {
			t.Error("expected false")
		}
	})
}

func TestSendVerification_Disabled(t *testing.T) {
	s := New(Config{Enabled: false})
	err := s.SendVerification("test@example.com", "token123")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestSendVerification_InvalidEmail(t *testing.T) {
	s := New(Config{Enabled: true})
	err := s.SendVerification("not-an-email", "token123")
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestSendInvite_Disabled(t *testing.T) {
	s := New(Config{Enabled: false})
	err := s.SendInvite("test@example.com", "Test User", "ABC123", "Inviter")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestSendInvite_InvalidEmail(t *testing.T) {
	s := New(Config{Enabled: true})
	err := s.SendInvite("not-an-email", "Test User", "ABC123", "Inviter")
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestLoginAuth(t *testing.T) {
	auth := LoginAuth("testuser", "testpass")
	if auth == nil {
		t.Fatal("LoginAuth returned nil")
	}
}

func TestLoginAuth_Start(t *testing.T) {
	auth := LoginAuth("testuser", "testpass")
	mech, data, err := auth.Start(&smtp.ServerInfo{Name: "test"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if mech != "LOGIN" {
		t.Errorf("expected mechanism LOGIN, got %s", mech)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
}

func TestLoginAuth_Next(t *testing.T) {
	auth := LoginAuth("testuser", "testpass")

	t.Run("more=false", func(t *testing.T) {
		data, err := auth.Next(nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data != nil {
			t.Errorf("expected nil data, got %v", data)
		}
	})

	t.Run("username challenge", func(t *testing.T) {
		data, err := auth.Next([]byte("Username:"), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "testuser" {
			t.Errorf("expected testuser, got %s", data)
		}
	})

	t.Run("password challenge", func(t *testing.T) {
		data, err := auth.Next([]byte("Password:"), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "testpass" {
			t.Errorf("expected testpass, got %s", data)
		}
	})

	t.Run("unknown challenge", func(t *testing.T) {
		_, err := auth.Next([]byte("Unknown:"), true)
		if err == nil {
			t.Error("expected error for unknown challenge")
		}
	})
}

func TestRenderTemplate(t *testing.T) {
	s := New(Config{})

	t.Run("valid template", func(t *testing.T) {
		tmpl := "Hello {{.Name}}!"
		result, err := s.renderTemplate(tmpl, map[string]string{"Name": "World"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "Hello World!" {
			t.Errorf("expected 'Hello World!', got %s", result)
		}
	})

	t.Run("invalid template", func(t *testing.T) {
		tmpl := "Hello {{.Name"
		_, err := s.renderTemplate(tmpl, nil)
		if err == nil {
			t.Error("expected error for invalid template")
		}
	})
}
