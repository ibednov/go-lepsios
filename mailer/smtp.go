// Package mailer provides a small stdlib SMTP sender with STARTTLS or implicit TLS.
package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Message struct{ To, Subject, Body string }
type Sender interface {
	Send(context.Context, Message) error
}
type Config struct {
	Address, Username, Password, From string
	ImplicitTLS                       bool
	Timeout                           time.Duration
}
type SMTP struct{ cfg Config }

func NewSMTP(cfg Config) (*SMTP, error) {
	if cfg.Address == "" || cfg.From == "" {
		return nil, errors.New("mailer: SMTP address and from are required")
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.Address); err != nil {
		return nil, fmt.Errorf("mailer: invalid address: %w", err)
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		return nil, errors.New("mailer: invalid from address")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &SMTP{cfg: cfg}, nil
}
func (s *SMTP) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return errors.New("mailer: invalid from address")
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return errors.New("mailer: invalid recipient")
	}
	if strings.ContainsAny(m.Subject, "\r\n") || m.Subject == "" {
		return errors.New("mailer: invalid subject")
	}
	conn, err := (&net.Dialer{Timeout: s.cfg.Timeout}).DialContext(ctx, "tcp", s.cfg.Address)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(s.cfg.Timeout))
	}
	host, _, _ := net.SplitHostPort(s.cfg.Address)
	var client *smtp.Client
	if s.cfg.ImplicitTLS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return err
		}
		client, err = smtp.NewClient(tlsConn, host)
	} else {
		client, err = smtp.NewClient(conn, host)
	}
	if err != nil {
		return err
	}
	defer client.Close()
	if !s.cfg.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("mailer: SMTP server does not support STARTTLS")
		}
		if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("mailer: SMTP server does not support authentication")
		}
		if err = client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, host)); err != nil {
			return err
		}
	}
	if err = client.Mail(from.Address); err != nil {
		return err
	}
	if err = client.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", from.String(), to.String(), m.Subject, m.Body)
	closeErr := w.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return client.Quit()
}
