package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	KindDownload       = "download"
	KindRaw            = "raw"
	PreferredSelection = "__preferred__"
)

type Entry struct {
	ID                  string     `json:"id"`
	Kind                string     `json:"kind"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	BenchmarkStatus     string     `json:"benchmarkStatus,omitempty"`
	SpeedBytesPerSecond int64      `json:"speedBytesPerSecond,omitempty"`
	StabilityPercent    int        `json:"stabilityPercent,omitempty"`
	BenchmarkScore      int64      `json:"benchmarkScore,omitempty"`
	LastBenchmarkAt     *time.Time `json:"lastBenchmarkAt,omitempty"`
}

func Parse(value string, kind string) ([]Entry, error) {
	kind = CleanKind(kind)
	if kind == "" {
		return nil, fmt.Errorf("mirror kind is required")
	}
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]Entry, 0, len(lines))
	seenNames := map[string]struct{}{}
	seenURLs := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, rawURL, ok := strings.Cut(line, "=>")
		if !ok {
			return nil, fmt.Errorf("mirror entries must use name=>url")
		}
		name = strings.TrimSpace(name)
		clean := Clean(rawURL)
		if name == "" {
			return nil, fmt.Errorf("mirror name is required")
		}
		if clean == "" {
			return nil, fmt.Errorf("mirror %q must use an http or https URL", name)
		}
		nameKey := strings.ToLower(name)
		if _, ok := seenNames[nameKey]; ok {
			return nil, fmt.Errorf("mirror name %q is duplicated", name)
		}
		if _, ok := seenURLs[clean]; ok {
			return nil, fmt.Errorf("mirror URL %q is duplicated", clean)
		}
		seenNames[nameKey] = struct{}{}
		seenURLs[clean] = struct{}{}
		out = append(out, Entry{ID: ID(kind, clean), Kind: kind, Name: name, URL: clean})
	}
	return out, nil
}

func Normalize(value string, kind string) (string, error) {
	entries, err := Parse(value, kind)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, entry.Name+"=>"+entry.URL)
	}
	return strings.Join(lines, "\n"), nil
}

func CleanKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindDownload, "release", "zip":
		return KindDownload
	case KindRaw:
		return KindRaw
	default:
		return ""
	}
}

func Clean(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return value
}

func Find(entries []Entry, id string) (Entry, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Entry{}, false
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

func FindApplicable(entries []Entry, id string, rawURL string) (Entry, bool) {
	entry, ok := Find(entries, id)
	if !ok {
		return Entry{}, false
	}
	if entry.Kind != KindForURL(rawURL) {
		return Entry{}, false
	}
	return entry, true
}

// OrderedForURL returns mirrors applicable to rawURL in their configured
// order, with a valid preferred mirror pinned to the front. The input slice is
// never mutated, so callers can safely reuse the feed/configuration order.
func OrderedForURL(entries []Entry, rawURL, preferredID string) []Entry {
	kind := KindForURL(rawURL)
	if kind == "" {
		return nil
	}
	ordered := make([]Entry, 0, len(entries))
	preferredIndex := -1
	for _, entry := range entries {
		if entry.Kind != kind {
			continue
		}
		if entry.ID == strings.TrimSpace(preferredID) {
			preferredIndex = len(ordered)
		}
		ordered = append(ordered, entry)
	}
	if preferredIndex > 0 {
		preferred := ordered[preferredIndex]
		copy(ordered[1:preferredIndex+1], ordered[:preferredIndex])
		ordered[0] = preferred
	}
	return ordered
}

// ResolveSelection resolves either a concrete mirror ID or the stable
// preferred selection. Preferred selection falls back to the upstream URL
// when no applicable mirror is configured.
func ResolveSelection(entries []Entry, rawURL, selectionID, preferredID string) (string, bool) {
	selectionID = strings.TrimSpace(selectionID)
	if selectionID == "" {
		return rawURL, true
	}
	if KindForURL(rawURL) == "" {
		return "", false
	}
	if selectionID == PreferredSelection {
		ordered := OrderedForURL(entries, rawURL, preferredID)
		if len(ordered) == 0 {
			return rawURL, true
		}
		return RewriteGitHub(rawURL, ordered[0]), true
	}
	entry, ok := FindApplicable(entries, selectionID, rawURL)
	if !ok {
		return "", false
	}
	return RewriteGitHub(rawURL, entry), true
}

func RewriteGitHub(rawURL string, entry Entry) string {
	if !IsGitHubURL(rawURL) {
		return rawURL
	}
	if entry.URL == "" {
		return rawURL
	}
	base := strings.TrimRight(entry.URL, "/")
	if rewritten := rewriteWithEmbeddedTarget(rawURL, base, "https://github.com"); rewritten != "" {
		return rewritten
	}
	if rewritten := rewriteWithEmbeddedTarget(rawURL, base, "https://raw.githubusercontent.com"); rewritten != "" {
		return rewritten
	}
	return base + "/" + rawURL
}

func IsGitHubURL(rawURL string) bool {
	return KindForURL(rawURL) != ""
}

func KindForURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "raw.githubusercontent.com":
		return KindRaw
	case "github.com":
		return KindDownload
	default:
		return ""
	}
}

func rewriteWithEmbeddedTarget(rawURL, base, target string) string {
	if !strings.HasPrefix(rawURL, target) {
		return ""
	}
	if strings.Contains(base, target) {
		return base + strings.TrimPrefix(rawURL, target)
	}
	targetWithoutScheme := strings.TrimPrefix(target, "https://")
	if strings.Contains(base, targetWithoutScheme) {
		return base + strings.TrimPrefix(rawURL, target)
	}
	return ""
}

func ID(kind, rawURL string) string {
	sum := sha256.Sum256([]byte(CleanKind(kind) + ":" + Clean(rawURL)))
	return "ghm_" + hex.EncodeToString(sum[:])[:12]
}
