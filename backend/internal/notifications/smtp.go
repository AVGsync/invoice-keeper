package notifications

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type SMTPMailer struct {
	config SMTPConfig
	logger *slog.Logger
}

func NewSMTPMailer(config SMTPConfig, logger *slog.Logger) (*SMTPMailer, error) {
	config.Host = strings.TrimSpace(config.Host)
	config.Port = strings.TrimSpace(config.Port)
	config.Username = strings.TrimSpace(config.Username)
	config.From = strings.TrimSpace(config.From)

	if config.Host == "" {
		return nil, errors.New("SMTP host is required")
	}
	if config.Port == "" {
		return nil, errors.New("SMTP port is required")
	}
	if config.Username == "" {
		return nil, errors.New("SMTP username is required")
	}
	if config.Password == "" {
		return nil, errors.New("SMTP password is required")
	}
	if config.From == "" {
		return nil, errors.New("SMTP sender address is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &SMTPMailer{
		config: config,
		logger: logger,
	}, nil
}

func (m *SMTPMailer) Send(
	ctx context.Context,
	to string,
	subject string,
	body string,
) error {
	to = strings.TrimSpace(to)

	if to == "" {
		return errors.New("recipient email is required")
	}
	if strings.ContainsAny(to, "\r\n") {
		return errors.New("invalid recipient email")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("invalid email subject")
	}

	address := net.JoinHostPort(m.config.Host, m.config.Port)

	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	} else {
		if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	}

	client, err := smtp.NewClient(conn, m.config.Host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("SMTP server does not support STARTTLS")
	}

	if err := client.StartTLS(&tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("start SMTP TLS: %w", err)
	}

	auth := smtp.PlainAuth(
		"",
		m.config.Username,
		m.config.Password,
		m.config.Host,
	)

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication: %w", err)
	}

	if err := client.Mail(m.config.From); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}

	message := buildMessage(m.config.From, to, subject, body)

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("send SMTP message: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}

	m.logger.Info("email sent", "recipient", to)

	return nil
}

func buildMessage(from, to, subject, body string) []byte {
	var builder strings.Builder

	builder.WriteString("From: " + from + "\r\n")
	builder.WriteString("To: " + to + "\r\n")
	// Тема с кириллицей кодируется по RFC 2047, иначе её читают криво.
	builder.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	builder.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	if at := strings.LastIndex(from, "@"); at >= 0 {
		builder.WriteString("Message-ID: <" + randomID() + "@" + strings.Trim(from[at+1:], "<> ") + ">\r\n")
	}
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	builder.WriteString("\r\n")

	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")

	builder.WriteString(body)

	return []byte(builder.String())
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

// compile-time check: SMTPMailer implements Mailer.
var _ Mailer = (*SMTPMailer)(nil)

// Keep SMTP protocol errors available for errors.As when needed.
var _ *textproto.Error
