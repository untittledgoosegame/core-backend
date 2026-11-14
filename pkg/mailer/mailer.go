package mailer

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/config"
	"github.com/untittledgoosegame/core-backend/templates"
)

const (
	dialTimeout    = 10 * time.Second
	sessionTimeout = 30 * time.Second
)

type Purpose string

const (
	PurposeRegister Purpose = "register"
	PurposeLogin    Purpose = "login"
	PurposeLink     Purpose = "link"
)

var codeIntros = map[Purpose]string{
	PurposeRegister: "Use this code to create your account.",
	PurposeLogin:    "Use this code to sign in.",
	PurposeLink:     "Use this code to add this email to your account.",
}

var pages = parsePages("code.html", "security_alert.html")

func parsePages(names ...string) map[string]*template.Template {
	out := make(map[string]*template.Template, len(names))
	for _, name := range names {
		out[name] = template.Must(template.ParseFS(templates.FS, "layout.html", name))
	}
	return out
}

type Mailer struct {
	cfg     config.SMTPConfig
	appName string
	dev     bool
}

func New(cfg config.SMTPConfig, appName string, dev bool) *Mailer {
	return &Mailer{cfg: cfg, appName: appName, dev: dev}
}

func (m *Mailer) SendCode(to, code string, purpose Purpose, ttl time.Duration) error {
	subject := fmt.Sprintf("Your %s code: %s", m.appName, code)
	if m.cfg.Host == "" && m.dev {
		log.Warn("smtp not configured, logging email code", "email", to, "purpose", purpose, "code", code)
		return nil
	}
	return m.send(to, subject, "code.html", map[string]any{
		"Intro":            codeIntros[purpose],
		"Code":             code,
		"ExpiresInMinutes": int(ttl.Minutes()),
	})
}

func (m *Mailer) SendSecurityAlert(to, title, message string) error {
	if m.cfg.Host == "" && m.dev {
		log.Warn("smtp not configured, skipping security alert", "email", to, "title", title)
		return nil
	}
	return m.send(to, fmt.Sprintf("%s security alert: %s", m.appName, title), "security_alert.html", map[string]any{
		"Title":   title,
		"Message": message,
		"When":    time.Now().UTC().Format("Mon, 2 Jan 2006 15:04 MST"),
	})
}

func (m *Mailer) send(to, subject, page string, data map[string]any) error {
	if m.cfg.Host == "" || m.cfg.From == "" {
		return apperr.Unavailable("email is not configured")
	}
	data["AppName"] = m.appName
	data["Subject"] = subject

	var body bytes.Buffer
	if err := pages[page].ExecuteTemplate(&body, page, data); err != nil {
		return fmt.Errorf("render %s: %w", page, err)
	}
	return m.deliver(to, subject, body.String())
}

func (m *Mailer) deliver(to, subject, htmlBody string) error {
	rcpt, err := mail.ParseAddress(to)
	if err != nil || rcpt.Address != to {
		return fmt.Errorf("invalid recipient %q", to)
	}
	from, err := mail.ParseAddress(m.cfg.From)
	if err != nil {
		return fmt.Errorf("invalid smtp.from: %w", err)
	}

	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	dialer := &net.Dialer{Timeout: dialTimeout}
	implicitTLS := m.cfg.Port == "465"
	var conn net.Conn
	if implicitTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: m.cfg.Host})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(sessionTimeout))

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok && !implicitTLS {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if m.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(rcpt.Address); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	headers := []string{
		"From: " + from.String(),
		"To: " + rcpt.Address,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="UTF-8"`,
	}
	if _, err := w.Write([]byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return c.Quit()
}
