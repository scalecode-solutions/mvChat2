package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html"
	"html/template"
	"net/mail"
	"net/smtp"
	"regexp"
	"strings"
)

// Config holds email configuration.
type Config struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
}

// Service handles email sending.
type Service struct {
	cfg Config
}

// New creates a new email service.
func New(cfg Config) *Service {
	return &Service{cfg: cfg}
}

// IsEnabled returns whether email is enabled.
func (s *Service) IsEnabled() bool {
	return s.cfg.Enabled
}

// SendInvite sends an invite code email.
func (s *Service) SendInvite(toEmail, toName, code, inviterName string) error {
	if !s.cfg.Enabled {
		return nil
	}

	// Validate email address
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return fmt.Errorf("invalid email address: %w", err)
	}

	// Sanitize inputs to prevent header injection and HTML injection
	safeInviterName := sanitizeForHeader(inviterName)
	if safeInviterName == "" {
		safeInviterName = "Someone"
	}

	subject := fmt.Sprintf("%s invited you to chat on mvChat", safeInviterName)

	// HTML-escape all user-controlled data for the template
	data := map[string]string{
		"ToName":      html.EscapeString(sanitizeForDisplay(toName)),
		"InviterName": html.EscapeString(safeInviterName),
		"Code":        html.EscapeString(code),
		"Link":        "https://chat.mvchat.app",
	}

	body, err := s.renderTemplate(inviteTemplate, data)
	if err != nil {
		return err
	}

	return s.send(toEmail, subject, body)
}

func (s *Service) send(to, subject, body string) error {
	// Sanitize header values to prevent header injection
	safeTo := sanitizeForHeader(to)
	safeSubject := sanitizeForHeader(subject)

	from := s.cfg.From
	if s.cfg.FromName != "" {
		safeFromName := sanitizeForHeader(s.cfg.FromName)
		from = fmt.Sprintf("%s <%s>", safeFromName, s.cfg.From)
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"\r\n"+
		"%s", from, safeTo, safeSubject, body)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	// Use TLS for port 465, STARTTLS for 587
	if s.cfg.Port == 465 {
		return s.sendSSL(addr, to, msg)
	}

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}

func (s *Service) sendSSL(addr, to, msg string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.cfg.Host})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if s.cfg.Username != "" {
		auth := LoginAuth(s.cfg.Username, s.cfg.Password)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}

	return client.Quit()
}

// loginAuth implements smtp.Auth for LOGIN authentication.
type loginAuth struct {
	username, password string
}

// LoginAuth returns an Auth that implements the LOGIN authentication mechanism.
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.username), nil
	case "Password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unknown server challenge: %s", fromServer)
	}
}

func (s *Service) renderTemplate(tmpl string, data any) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// headerInjectionRegex matches characters that could be used for header injection
var headerInjectionRegex = regexp.MustCompile(`[\r\n\x00]`)

// sanitizeForHeader removes characters that could be used for email header injection.
// This prevents attackers from injecting additional headers like BCC.
func sanitizeForHeader(s string) string {
	// Remove CR, LF, and null bytes that could inject headers
	s = headerInjectionRegex.ReplaceAllString(s, "")
	// Trim whitespace
	s = strings.TrimSpace(s)
	// Limit length to prevent buffer issues
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// sanitizeForDisplay cleans user input for display purposes.
// Removes control characters but keeps the text readable.
func sanitizeForDisplay(s string) string {
	// Remove control characters except space
	var result strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\t' {
			result.WriteRune(r)
		}
	}
	s = strings.TrimSpace(result.String())
	// Limit length
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

const inviteTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; border-radius: 10px 10px 0 0; text-align: center; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .code { background: #fff; border: 2px dashed #667eea; padding: 20px; text-align: center; margin: 20px 0; border-radius: 8px; }
        .code-text { font-size: 32px; font-weight: bold; letter-spacing: 4px; color: #667eea; font-family: monospace; }
        .button { display: inline-block; background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin-top: 20px; }
        .footer { text-align: center; margin-top: 20px; color: #888; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>You're Invited!</h1>
    </div>
    <div class="content">
        <p>Hi{{if .ToName}} {{.ToName}}{{end}},</p>
        <p><strong>{{.InviterName}}</strong> has invited you to connect on mvChat, a secure messaging platform.</p>
        <p>Use this invite code to create your account:</p>
        <div class="code">
            <span class="code-text">{{.Code}}</span>
        </div>
        <p>This code expires in 7 days.</p>
        <p style="text-align: center;">
            <a href="{{.Link}}" class="button">Open mvChat</a>
        </p>
    </div>
    <div class="footer">
        <p>This invitation was sent by {{.InviterName}}. If you didn't expect this, you can safely ignore it.</p>
    </div>
</body>
</html>`
