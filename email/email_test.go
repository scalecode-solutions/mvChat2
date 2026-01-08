package email

import "testing"

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
