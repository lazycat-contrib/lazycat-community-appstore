package mirrorprobe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lazycat.community/appstore/internal/mirror"
)

func TestMeasureRanksRealThroughputAndKeepsStableTies(t *testing.T) {
	entries := []mirror.Entry{
		{ID: "slow", Kind: mirror.KindDownload, URL: "https://slow.example/https://github.com"},
		{ID: "fast", Kind: mirror.KindDownload, URL: "https://fast.example/https://github.com"},
		{ID: "failed", Kind: mirror.KindDownload, URL: "https://failed.example/https://github.com"},
	}
	probe := func(_ context.Context, rawURL string) (int64, error) {
		switch {
		case strings.Contains(rawURL, "fast.example"):
			return 8_000_000, nil
		case strings.Contains(rawURL, "slow.example"):
			return 2_000_000, nil
		default:
			return 0, errors.New("unavailable")
		}
	}

	measurements := Measure(t.Context(), entries, "https://github.com/acme/app/releases/download/v1/app.lpk", probe)
	fastest, ok := Fastest(measurements)
	if !ok || fastest.ID != "fast" {
		t.Fatalf("fastest = %#v, %t", fastest, ok)
	}
}

func TestProbeRejectsUnsafeTargetsBeforeRequest(t *testing.T) {
	for _, rawURL := range []string{
		"http://example.com/file.lpk",
		"https://user:pass@example.com/file.lpk",
		"https://127.0.0.1/file.lpk",
		"https://[::1]/file.lpk",
	} {
		if _, err := Probe(t.Context(), rawURL); err == nil {
			t.Fatalf("Probe(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestValidateProbeSampleRejectsErrorDocuments(t *testing.T) {
	for _, sample := range [][]byte{
		[]byte("<!doctype html><title>rate limited</title>"),
		[]byte(`{"message":"rate limited"}`),
	} {
		if err := validateProbeSample(sample); err == nil {
			t.Fatalf("validateProbeSample(%q) unexpectedly succeeded", sample)
		}
	}
	if err := validateProbeSample([]byte("PK\x03\x04binary-lpk")); err != nil {
		t.Fatalf("validateProbeSample rejected an archive: %v", err)
	}
}
