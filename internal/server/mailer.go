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

type smtpConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func newSMTPMailer(cfg config.Config) Mailer {
	return smtpMailer{cfg: cfg}
}

func (m smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	return m.SendWithConfig(ctx, smtpConfig{
		Host: m.cfg.SMTPHost,
		Port: m.cfg.SMTPPort,
		User: m.cfg.SMTPUser,
		Pass: m.cfg.SMTPPass,
		From: m.cfg.SMTPFrom,
	}, to, subject, body)
}

func (m smtpMailer) SendWithConfig(ctx context.Context, cfg smtpConfig, to, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.From) == "" || strings.TrimSpace(to) == "" {
		return nil
	}
	if cfg.Port <= 0 {
		cfg.Port = 25
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var auth smtp.Auth
	if cfg.User != "" || cfg.Pass != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}
	message := []byte(strings.Join([]string{
		"From: " + cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n"))
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, message)
}

func (s *Server) smtpSettings(ctx context.Context) smtpConfig {
	return smtpConfig{
		Host: strings.TrimSpace(s.setting(ctx, settingSMTPHost, s.cfg.SMTPHost)),
		Port: s.settingInt(ctx, settingSMTPPort, s.cfg.SMTPPort),
		User: strings.TrimSpace(s.setting(ctx, settingSMTPUser, s.cfg.SMTPUser)),
		Pass: s.setting(ctx, settingSMTPPass, s.cfg.SMTPPass),
		From: strings.TrimSpace(s.setting(ctx, settingSMTPFrom, s.cfg.SMTPFrom)),
	}
}

func (s *Server) emailDeliveryConfigured(ctx context.Context) bool {
	cfg := s.smtpSettings(ctx)
	return strings.TrimSpace(cfg.Host) != "" && strings.TrimSpace(cfg.From) != ""
}

func (s *Server) sendEmail(ctx context.Context, to, subject, body string) error {
	if !s.emailDeliveryConfigured(ctx) || strings.TrimSpace(to) == "" {
		return nil
	}
	if mailer, ok := s.mailer.(smtpMailer); ok {
		return mailer.SendWithConfig(ctx, s.smtpSettings(ctx), to, subject, body)
	}
	return s.mailer.Send(ctx, to, subject, body)
}

func (s *Server) sendVerificationEmail(ctx context.Context, to, token string) error {
	if !s.emailDeliveryConfigured(ctx) || strings.TrimSpace(to) == "" {
		return nil
	}
	verifyURL := strings.TrimRight(s.sitePublicURL(ctx), "/") + "/login?mode=verify&token=" + token
	body := fmt.Sprintf("Open this link to verify your LazyCat Private Store account:\n\n%s\n\nIf the client does not handle this link yet, paste this token in the verification form:\n\n%s\n", verifyURL, token)
	return s.sendEmail(ctx, to, "Verify your LazyCat Private Store email", body)
}
