// Package mail sends the few transactional messages this service needs.
//
// Two senders, chosen by configuration: SMTP for anywhere real, and a logging
// sender for development. The logging one is what makes a password reset
// completable on a laptop with no mail server, and it is the reason
// `handlePasswordForgot` never had to put a token in an HTTP response.
package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Message is one email. Plain text only: these are short, transactional, and
// a password-reset link that renders in every client beats one that looks
// nice in some.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers a message. Declared here, next to its callers, so a service
// can be handed a fake without a mail server.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// LogSender writes messages to the log instead of sending them.
//
// For development. It exists so the password-reset flow can be exercised end
// to end locally: the reset link appears in the server's own output, which is
// reachable by the developer running it and by nobody else. `cmd/api` selects
// this only outside production, so it is structurally impossible to deploy a
// service that "sends" mail into a log file.
type LogSender struct {
	Log *slog.Logger
}

func (s LogSender) Send(ctx context.Context, msg Message) error {
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "email (not sent: development mail sender)",
		"to", msg.To, "subject", msg.Subject, "body", msg.Body)
	return nil
}

// SMTPSender delivers over SMTP with STARTTLS.
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// FromName is the display name on the From header.
	FromName string
}

func (s SMTPSender) addr() string { return net.JoinHostPort(s.Host, fmt.Sprint(s.Port)) }

// Send delivers one message.
//
// The context bounds the whole exchange. net/smtp has no context support of
// its own, so the deadline is applied by dialing with one and closing the
// connection when it expires -- without that, an unresponsive mail host holds
// the request that triggered it for as long as the TCP stack allows.
func (s SMTPSender) Send(ctx context.Context, msg Message) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.addr())
	if err != nil {
		return fmt.Errorf("dialing smtp %s: %w", s.addr(), err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return fmt.Errorf("smtp handshake with %s: %w", s.Host, err)
	}
	defer client.Close()

	// STARTTLS whenever the server offers it. The credentials below travel on
	// this connection, and PLAIN auth over a cleartext link hands them to
	// anyone on the path.
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("starttls with %s: %w", s.Host, err)
		}
	}

	if s.Username != "" {
		auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth as %s: %w", s.Username, err)
		}
	}

	if err := client.Mail(s.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write([]byte(s.render(msg))); err != nil {
		return fmt.Errorf("writing message body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("closing message body: %w", err)
	}

	return client.Quit()
}

// render builds the wire form of the message.
//
// Header values are stripped of CR and LF before they go in. A newline in a
// subject or address lets a caller append headers of their own -- a Bcc, say --
// which is header injection, and the only input here that a user influences is
// the address they typed.
func (s SMTPSender) render(msg Message) string {
	from := s.From
	if s.FromName != "" {
		from = fmt.Sprintf("%s <%s>", stripCRLF(s.FromName), s.From)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", stripCRLF(from))
	fmt.Fprintf(&b, "To: %s\r\n", stripCRLF(msg.To))
	fmt.Fprintf(&b, "Subject: %s\r\n", stripCRLF(msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	// Bare newlines become CRLF: SMTP's line ending is CRLF, and a lone LF in
	// a body is interpreted inconsistently across servers.
	b.WriteString(strings.ReplaceAll(strings.ReplaceAll(msg.Body, "\r\n", "\n"), "\n", "\r\n"))

	return b.String()
}

func stripCRLF(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}
