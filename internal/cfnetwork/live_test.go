package cfnetwork

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"slices"
	"testing"
	"time"
)

func TestLivePreferredEndpoint(t *testing.T) {
	if os.Getenv("CF_NETWORK_TEST") != "1" {
		t.Skip("set CF_NETWORK_TEST=1 to compare real Cloudflare routes")
	}
	endpoint := DefaultEndpoint
	if value := os.Getenv("CF_TEST_ENDPOINT"); value != "" {
		endpoint = value
	}
	const target = "https://apps.pushcat.eu.org/source/v2/index.json"
	var expectedHash [sha256.Size]byte
	haveHash := false
	timings := map[string][]time.Duration{"direct": {}, "preferred": {}}
	for round := range 3 {
		// Alternate order to reduce warm-cache/order bias. Each request uses a fresh connection.
		modes := []string{"direct", "preferred"}
		if round%2 == 1 {
			slices.Reverse(modes)
		}
		for _, mode := range modes {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = nil
			if mode == "preferred" {
				transport = NewTransport(endpoint, false)
			}
			client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			remote := ""
			trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { remote = info.Conn.RemoteAddr().String() }}
			req, _ := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), trace), "GET", target, nil)
			start := time.Now()
			resp, err := client.Do(req)
			if err != nil {
				transport.CloseIdleConnections()
				t.Errorf("round=%d route=%s endpoint=%s error=%v", round+1, mode, endpoint, err)
				continue
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20+1))
			_ = resp.Body.Close()
			elapsed := time.Since(start)
			transport.CloseIdleConnections()
			var feed struct {
				Apps []json.RawMessage `json:"apps"`
			}
			if err != nil || resp.StatusCode != 200 || len(body) > 64<<20 || json.Unmarshal(body, &feed) != nil || len(feed.Apps) == 0 {
				t.Errorf("round=%d route=%s invalid feed status=%d bytes=%d read=%v", round+1, mode, resp.StatusCode, len(body), err)
				continue
			}
			hash := sha256.Sum256(body)
			if haveHash && hash != expectedHash {
				t.Errorf("response differs from comparison baseline; feed may have changed during test: route=%s", mode)
			}
			expectedHash = hash
			haveHash = true
			timings[mode] = append(timings[mode], elapsed)
			t.Logf("round=%d route=%s remote=%s total=%s bytes=%d apps=%d sha256=%x", round+1, mode, remote, elapsed, len(body), len(feed.Apps), sha256.Sum256(body))
		}
	}
	if len(timings["direct"]) != 3 || len(timings["preferred"]) != 3 {
		t.Fatal("comparison incomplete; preferred requests never use direct fallback")
	}
	for _, values := range timings {
		slices.Sort(values)
	}
	direct, preferred := timings["direct"][1], timings["preferred"][1]
	t.Logf("median direct=%s preferred=%s improvement=%.1f%% faster=%v (network-specific, not a universal guarantee)", direct, preferred, 100*(1-float64(preferred)/float64(direct)), preferred < direct)
	if os.Getenv("CF_REQUIRE_FASTER") == "1" && preferred >= direct {
		t.Fatal("preferred endpoint is not faster on this network")
	}
}
