package server

import (
	"net/mail"
	"testing"
)

func TestSMTPFromHeaderUsesConfiguredDisplayName(t *testing.T) {
	got := smtpFromHeader(smtpConfig{From: "store@example.com", FromName: "懒猫私有商店"})
	parsed, err := mail.ParseAddress(got)
	if err != nil {
		t.Fatalf("parse from header %q: %v", got, err)
	}
	if parsed.Name != "懒猫私有商店" || parsed.Address != "store@example.com" {
		t.Fatalf("from header = %#v", parsed)
	}
}

func TestSMTPFromHeaderFallsBackToAddress(t *testing.T) {
	if got := smtpFromHeader(smtpConfig{From: "store@example.com"}); got != "store@example.com" {
		t.Fatalf("from header = %q", got)
	}
}
