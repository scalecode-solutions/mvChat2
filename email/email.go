package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
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

	subject := fmt.Sprintf("%s invited you to chat on mvChat", inviterName)

	data := map[string]string{
		"ToName":      toName,
		"InviterName": inviterName,
		"Code":        code,
		"Link":        "https://chat.mvchat.app",
	}

	body, err := s.renderTemplate(inviteTemplate, data)
	if err != nil {
		return err
	}

	return s.send(toEmail, subject, body)
}

func (s *Service) send(to, subject, body string) error {
	from := s.cfg.From
	if s.cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.cfg.FromName, s.cfg.From)
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"\r\n"+
		"%s", from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
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
