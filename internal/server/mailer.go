package server

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"lazycat.community/appstore/internal/config"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type smtpMailer struct {
	cfg config.Config
}

func newSMTPMailer(cfg config.Config) Mailer {
	return smtpMailer{cfg: cfg}
}

func (m smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if strings.TrimSpace(m.cfg.SMTPHost) == "" || strings.TrimSpace(m.cfg.SMTPFrom) == "" {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	var auth smtp.Auth
	if m.cfg.SMTPUser != "" || m.cfg.SMTPPass != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}
	message := []byte(strings.Join([]string{
		"From: " + m.cfg.SMTPFrom,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n"))
	return smtp.SendMail(addr, auth, m.cfg.SMTPFrom, []string{to}, message)
}

func (s *Server) emailDeliveryConfigured() bool {
	return strings.TrimSpace(s.cfg.SMTPHost) != "" && strings.TrimSpace(s.cfg.SMTPFrom) != ""
}

func (s *Server) sendVerificationEmail(ctx context.Context, to, token string) error {
	if !s.emailDeliveryConfigured() || strings.TrimSpace(to) == "" {
		return nil
	}
	verifyURL := strings.TrimRight(s.cfg.SitePublicURL, "/") + "/verify?token=" + token
	body := fmt.Sprintf("Open this link to verify your LazyCat App Store account:\n\n%s\n\nIf the client does not handle this link yet, paste this token in the verification form:\n\n%s\n", verifyURL, token)
	return s.mailer.Send(ctx, to, "Verify your LazyCat App Store email", body)
}
