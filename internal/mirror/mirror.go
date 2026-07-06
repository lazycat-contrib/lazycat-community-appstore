package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

const (
	KindDownload = "download"
	KindRaw      = "raw"
)

type Entry struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	URL  string `json:"url"`
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
	return strings.Contains(rawURL, "github.com/") || strings.Contains(rawURL, "githubusercontent.com/")
}

func KindForURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.Contains(rawURL, "raw.githubusercontent.com/") {
		return KindRaw
	}
	if strings.Contains(rawURL, "github.com/") {
		return KindDownload
	}
	return ""
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
