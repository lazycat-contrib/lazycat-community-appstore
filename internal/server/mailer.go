package server

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/lib-x/mailingo"

	"lazycat.community/appstore/internal/config"
)

//go:embed mail_locales/*.json
var mailLocalesFS embed.FS

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type smtpMailer struct {
	cfg config.Config
}

type smtpConfig struct {
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	FromName string
}

func newSMTPMailer(cfg config.Config) Mailer {
	return smtpMailer{cfg: cfg}
}

func (m smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	return m.SendWithConfig(ctx, smtpConfig{
		Host:     m.cfg.SMTPHost,
		Port:     m.cfg.SMTPPort,
		User:     m.cfg.SMTPUser,
		Pass:     m.cfg.SMTPPass,
		From:     m.cfg.SMTPFrom,
		FromName: m.cfg.SMTPFromName,
	}, to, subject, body)
}

func (m smtpMailer) SendWithConfig(ctx context.Context, cfg smtpConfig, to, subject, body string) error {
	return m.SendMessageWithConfig(ctx, cfg, to, subject, body, "")
}

func (m smtpMailer) SendMessageWithConfig(ctx context.Context, cfg smtpConfig, to, subject, textBody, htmlBody string) error {
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
	message, err := buildSMTPMessage(cfg, to, subject, textBody, htmlBody)
	if err != nil {
		return err
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, message)
}

func buildSMTPMessage(cfg smtpConfig, to, subject, textBody, htmlBody string) ([]byte, error) {
	var message bytes.Buffer
	headers := []string{
		"From: " + smtpFromHeader(cfg),
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version: 1.0",
	}
	for _, header := range headers {
		message.WriteString(header + "\r\n")
	}
	if strings.TrimSpace(htmlBody) == "" {
		message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		message.WriteString("\r\n")
		message.WriteString(textBody)
		return message.Bytes(), nil
	}
	writer := multipart.NewWriter(&message)
	message.WriteString("Content-Type: multipart/alternative; boundary=" + writer.Boundary() + "\r\n")
	message.WriteString("\r\n")
	textPart, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := textPart.Write([]byte(textBody)); err != nil {
		return nil, err
	}
	htmlPart, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/html; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := htmlPart.Write([]byte(htmlBody)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return message.Bytes(), nil
}

func (s *Server) smtpSettings(ctx context.Context) smtpConfig {
	return smtpConfig{
		Host:     strings.TrimSpace(s.setting(ctx, settingSMTPHost, s.cfg.SMTPHost)),
		Port:     s.settingInt(ctx, settingSMTPPort, s.cfg.SMTPPort),
		User:     strings.TrimSpace(s.setting(ctx, settingSMTPUser, s.cfg.SMTPUser)),
		Pass:     s.setting(ctx, settingSMTPPass, s.cfg.SMTPPass),
		From:     strings.TrimSpace(s.setting(ctx, settingSMTPFrom, s.cfg.SMTPFrom)),
		FromName: strings.TrimSpace(s.setting(ctx, settingSMTPFromName, s.cfg.SMTPFromName)),
	}
}

func smtpFromHeader(cfg smtpConfig) string {
	address := strings.TrimSpace(cfg.From)
	name := strings.TrimSpace(cfg.FromName)
	if name == "" {
		return address
	}
	return (&mail.Address{Name: name, Address: address}).String()
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

func (s *Server) sendRenderedEmail(ctx context.Context, to, subject, textBody, htmlBody string) error {
	if !s.emailDeliveryConfigured(ctx) || strings.TrimSpace(to) == "" {
		return nil
	}
	if mailer, ok := s.mailer.(smtpMailer); ok {
		return mailer.SendMessageWithConfig(ctx, s.smtpSettings(ctx), to, subject, textBody, htmlBody)
	}
	return s.mailer.Send(ctx, to, subject, textBody)
}

func (s *Server) sendVerificationEmail(ctx context.Context, to, recipientName, token, lang string) error {
	verifyURL := strings.TrimRight(s.sitePublicURL(ctx), "/") + "/login?mode=verify&token=" + token
	subject, textBody, htmlBody, err := s.renderMail(ctx, mailKindVerification, mailRenderData{
		RecipientName: recipientName,
		ActionURL:     verifyURL,
		Token:         token,
		Language:      lang,
	})
	if err != nil {
		return err
	}
	return s.sendRenderedEmail(ctx, to, subject, textBody, htmlBody)
}

type mailKind string

const (
	mailKindVerification  mailKind = "verification"
	mailKindPasswordReset mailKind = "password_reset"
	mailKindOutdated      mailKind = "outdated"
	mailKindTest          mailKind = "test"
)

type mailRenderData struct {
	RecipientName string
	ActionURL     string
	Token         string
	Language      string
	AppName       string
	ActorName     string
	Message       string
}

func (s *Server) renderMail(ctx context.Context, kind mailKind, data mailRenderData) (string, string, string, error) {
	lang := normalizeMailLanguage(data.Language)
	profile := s.siteProfile(ctx)
	product := mailingo.Product{
		Name:      profile.Title,
		Link:      s.sitePublicURL(ctx),
		Logo:      profile.IconURL,
		Copyright: fmt.Sprintf("© %d %s", time.Now().Year(), profile.Title),
	}
	mailer := mailingo.New(product, mailingo.Theme{
		PrimaryColor:    "#2F6F4E",
		BackgroundColor: "#F5F7F4",
		TextColor:       "#2E3430",
		ButtonColor:     "#2F6F4E",
		ButtonTextColor: "#FFFFFF",
	})
	for _, path := range []string{"mail_locales/en-US.json", "mail_locales/zh-CN.json"} {
		if err := mailer.LoadMessageFileFS(mailLocalesFS, path); err != nil {
			return "", "", "", err
		}
	}
	email := mailingo.Email{Body: mailingo.Body{
		Name:      strings.TrimSpace(data.RecipientName),
		Greeting:  "greeting",
		Signature: "signature",
	}}
	if email.Body.Name == "" {
		email.Body.Name = product.Name
	}
	switch kind {
	case mailKindVerification:
		email.Body.Title = "email.verify.title"
		email.Body.Intros = []string{"email.verify.intro"}
		email.Body.Actions = []mailingo.Action{{
			Instructions: "email.verify.instructions",
			Button:       mailingo.Button{Text: "email.verify.button", Link: data.ActionURL},
		}}
		email.Body.Dictionary = []mailingo.Entry{{Key: "email.token", Value: data.Token}}
		email.Body.Outros = []string{"email.verify.outro", "email.verify.warning"}
	case mailKindPasswordReset:
		email.Body.Title = "email.password_reset.title"
		email.Body.Intros = []string{"email.password_reset.intro"}
		email.Body.Actions = []mailingo.Action{{
			Instructions: "email.password_reset.instructions",
			Button:       mailingo.Button{Text: "email.password_reset.button", Link: data.ActionURL},
		}}
		email.Body.Outros = []string{"email.password_reset.outro", "email.password_reset.warning"}
	case mailKindOutdated:
		email.Body.Title = "email.outdated.title"
		email.Body.Intros = []string{"email.outdated.intro"}
		if strings.TrimSpace(data.Message) != "" {
			email.Body.Intros = append(email.Body.Intros, splitMailParagraphs(data.Message)...)
		}
		email.Body.Dictionary = []mailingo.Entry{
			{Key: "email.app", Value: data.AppName},
			{Key: "email.from", Value: data.ActorName},
		}
		email.Body.Actions = []mailingo.Action{{
			Instructions: "email.outdated.instructions",
			Button:       mailingo.Button{Text: "email.outdated.button", Link: product.Link},
		}}
		email.Body.Outros = []string{"email.outdated.outro"}
	case mailKindTest:
		email.Body.Title = "email.test.title"
		email.Body.Intros = []string{"email.test.intro"}
		email.Body.Outros = []string{"email.test.outro"}
	default:
		return "", "", "", fmt.Errorf("unknown mail kind %q", kind)
	}
	htmlBody, err := mailer.GenerateHTML(email, lang)
	if err != nil {
		return "", "", "", err
	}
	textBody, err := mailer.GeneratePlainText(email, lang)
	if err != nil {
		return "", "", "", err
	}
	return mailSubject(kind, lang, product.Name, data), textBody, htmlBody, nil
}

func splitMailParagraphs(value string) []string {
	chunks := strings.Split(strings.TrimSpace(value), "\n\n")
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk = strings.TrimSpace(chunk); chunk != "" {
			out = append(out, chunk)
		}
	}
	return out
}

func mailSubject(kind mailKind, lang, productName string, data mailRenderData) string {
	zh := strings.HasPrefix(normalizeMailLanguage(lang), "zh")
	switch kind {
	case mailKindVerification:
		if zh {
			return "验证你的" + productName + "邮箱"
		}
		return "Verify your " + productName + " email"
	case mailKindPasswordReset:
		if zh {
			return "重置你的" + productName + "密码"
		}
		return "Reset your " + productName + " password"
	case mailKindOutdated:
		if zh {
			return "应用更新提醒：" + data.AppName
		}
		return "Update requested for " + data.AppName
	case mailKindTest:
		if zh {
			return productName + "测试邮件"
		}
		return productName + " test email"
	default:
		return productName
	}
}

func normalizeMailLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lang, "zh") {
		return "zh-CN"
	}
	return "en-US"
}
