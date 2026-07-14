package server

import (
	"net/http"
	"strconv"
	"strings"
)

func serveSourceFeedSnapshot(w http.ResponseWriter, r *http.Request, snapshot sourceFeedSnapshot) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("ETag", snapshot.ETag)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Vary", "Accept-Encoding, X-Group-Codes, X-Source-Password")
	if sourceFeedETagMatches(r.Header.Get("If-None-Match"), snapshot.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	encoding, ok := preferredSourceFeedEncoding(r.Header.Get("Accept-Encoding"))
	if !ok {
		writeError(w, http.StatusNotAcceptable, "ENCODING_NOT_ACCEPTABLE", "No supported response encoding is acceptable", nil)
		return
	}
	body := snapshot.Identity
	switch encoding {
	case "br":
		body = snapshot.Brotli
		w.Header().Set("Content-Encoding", "br")
	case "gzip":
		body = snapshot.Gzip
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func sourceFeedETagMatches(header, etag string) bool {
	etag = weakSourceFeedETag(etag)
	for value := range strings.SplitSeq(header, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || weakSourceFeedETag(value) == etag {
			return true
		}
	}
	return false
}

func weakSourceFeedETag(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "W/")
	return value
}

func preferredSourceFeedEncoding(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "identity", true
	}
	qualities := map[string]float64{}
	wildcard := -1.0
	for item := range strings.SplitSeq(header, ",") {
		parts := strings.Split(item, ";")
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		if name == "" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = 0
			} else {
				quality = parsed
			}
		}
		if name == "*" {
			wildcard = quality
		} else {
			qualities[name] = quality
		}
	}

	qualityFor := func(name string) float64 {
		if quality, ok := qualities[name]; ok {
			return quality
		}
		if wildcard >= 0 {
			return wildcard
		}
		if name == "identity" {
			return 0.001
		}
		return 0
	}
	bestName := ""
	bestQuality := 0.0
	for _, name := range []string{"br", "gzip", "identity"} {
		quality := qualityFor(name)
		if quality > bestQuality {
			bestName = name
			bestQuality = quality
		}
	}
	return bestName, bestName != ""
}
