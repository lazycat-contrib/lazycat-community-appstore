package server

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func TestSMTPFromHeaderUsesConfiguredDisplayName(t *testing.T) {
	got := smtpFromHeader(smtpConfig{From: "store@example.com", FromName: "喵喵私有商店"})
	parsed, err := mail.ParseAddress(got)
	if err != nil {
		t.Fatalf("parse from header %q: %v", got, err)
	}
	if parsed.Name != "喵喵私有商店" || parsed.Address != "store@example.com" {
		t.Fatalf("from header = %#v", parsed)
	}
}

func TestSMTPFromHeaderFallsBackToAddress(t *testing.T) {
	if got := smtpFromHeader(smtpConfig{From: "store@example.com"}); got != "store@example.com" {
		t.Fatalf("from header = %q", got)
	}
}

func TestBuildSMTPMessageUsesMultipartAlternativeForHTML(t *testing.T) {
	raw, err := buildSMTPMessage(
		smtpConfig{From: "store@example.com", FromName: "MiaoMiao Store"},
		"user@example.com",
		"Reset password",
		"Plain reset body",
		"<strong>HTML reset body</strong>",
	)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse smtp message: %v\n%s", err, string(raw))
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("content type = %q params=%v", mediaType, params)
	}
	reader := multipart.NewReader(msg.Body, params["boundary"])
	parts := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		parts[part.Header.Get("Content-Type")] = string(body)
	}
	if !strings.Contains(parts["text/plain; charset=utf-8"], "Plain reset body") {
		t.Fatalf("plain part missing: %#v", parts)
	}
	if !strings.Contains(parts["text/html; charset=utf-8"], "HTML reset body") {
		t.Fatalf("html part missing: %#v", parts)
	}
}
