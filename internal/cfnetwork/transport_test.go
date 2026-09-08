package cfnetwork

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	for _, value := range []string{"saas.sin.fan", "1.1.1.1", "2606:4700:4700::1111", "  SAAS.SIN.FAN. "} {
		if _, err := Normalize(value); err != nil {
			t.Errorf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"", "https://saas.sin.fan", "saas.sin.fan:443", "saas.sin.fan/a", "user@saas.sin.fan", "127.0.0.1", "::1", "10.0.0.1", "100.64.0.1", "::ffff:127.0.0.1", "169.254.169.254", "1.2.3.999", "-bad.example", "localhost", "a..b", "224.0.0.1", "2001:db8::1"} {
		if _, err := Normalize(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	endpoints, err := ParseList("SAAS.SIN.FAN\n1.1.1.1,saas.sin.fan\n")
	if err != nil || strings.Join(endpoints, ",") != "saas.sin.fan,1.1.1.1" {
		t.Fatalf("list=%v err=%v", endpoints, err)
	}
	if _, err := ParseList(strings.Repeat("a", 8193)); err == nil {
		t.Fatal("accepted oversized list")
	}
}

// The real TLS handshake uses an independently trusted origin certificate, with
// an unresolvable origin name routed to the test edge. A URL rewrite would fail.
func TestPreferredConnectionPreservesHostSNIAndCertificate(t *testing.T) {
	var calls atomic.Int32
	edge := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Host != "example.com" || r.TLS.ServerName != "example.com" || r.URL.RequestURI() != "/source/v2/index.json?keep=1" || r.Header.Get("X-Source-Password") != "secret" {
			t.Errorf("origin identity lost: Host=%s SNI=%s URI=%s", r.Host, r.TLS.ServerName, r.URL.RequestURI())
		}
		_, _ = io.WriteString(w, `{"apps":[]}`)
	}))
	defer edge.Close()
	_, port, _ := net.SplitHostPort(edge.Listener.Addr().String())
	roots := x509.NewCertPool()
	roots.AddCert(edge.Certificate())
	transport := NewTransport(DefaultEndpoint, false)
	defer transport.CloseIdleConnections()
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(addr)
		return dialResolved(ctx, network, port, []netip.Addr{netip.MustParseAddr("127.0.0.1")}, &tls.Config{RootCAs: roots, ServerName: host, MinVersion: tls.VersionTLS12}, func(netip.Addr) bool { return true })
	}
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequestWithContext(t.Context(), "GET", "https://example.com/source/v2/index.json?keep=1", nil)
	req.Header.Set("X-Source-Password", "secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || string(body) != `{"apps":[]}` || calls.Load() != 1 {
		t.Fatalf("body=%s err=%v calls=%d", body, err, calls.Load())
	}
	req, _ = http.NewRequestWithContext(t.Context(), "GET", "https://wrong-origin.example/", nil)
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
		t.Fatal("accepted wrong origin certificate")
	}
	if calls.Load() != 1 {
		t.Fatal("sent HTTP before validating certificate")
	}
}

func TestResolvedPrivateAddressesAreNeverDialed(t *testing.T) {
	_, err := dialResolved(t.Context(), "tcp", "443", []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}, &tls.Config{MinVersion: tls.VersionTLS12}, publicIP)
	if err == nil || !strings.Contains(err.Error(), "no public") {
		t.Fatalf("err=%v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOriginBoundary(t *testing.T) {
	for _, target := range []string{"https://origin.example/api/v1/chat/events", "https://other.example/api", "http://origin.example/api", "https://origin.example:8443/api"} {
		preferred := false
		rt := OriginTransport{Origin: "origin.example", Preferred: roundTripFunc(func(*http.Request) (*http.Response, error) { preferred = true; return nil, nil }), Direct: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })}
		req, _ := http.NewRequestWithContext(t.Context(), "GET", target, nil)
		resp, _ := rt.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		if preferred != (target == "https://origin.example/api/v1/chat/events") {
			t.Fatalf("incorrect routing for %s", target)
		}
	}
}

func TestConnectionFallbackAndStrictMode(t *testing.T) {
	// The origin uses the normal trusted system certificate chain in production.
	// An HTTP test server cannot stand in for TLS: a strict attempt must fail and a
	// fallback must reach TLS verification at the direct destination without HTTP.
	var hello atomic.Int32
	edge := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("untrusted HTTP delivered") }))
	edge.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) { hello.Add(1); return nil, nil }}
	edge.StartTLS()
	defer edge.Close()
	for _, fallback := range []bool{false, true} {
		transport := NewTransport("invalid", fallback)
		client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
		req, _ := http.NewRequestWithContext(t.Context(), "POST", edge.URL, strings.NewReader("write once"))
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			t.Fatal("accepted untrusted certificate")
		}
		transport.CloseIdleConnections()
		if (hello.Load() > 0) != fallback {
			t.Fatalf("fallback=%v hello=%d", fallback, hello.Load())
		}
	}
}

func TestFallbackPreservesHTTP2AndSendsPostOnce(t *testing.T) {
	var calls atomic.Int32
	edge := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(body) != "write once" || r.ProtoMajor != 2 {
			t.Errorf("method=%s body=%s protocol=%s", r.Method, body, r.Proto)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	edge.EnableHTTP2 = true
	edge.StartTLS()
	defer edge.Close()
	roots := x509.NewCertPool()
	roots.AddCert(edge.Certificate())
	transport := NewTransport("invalid", true)
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequestWithContext(t.Context(), "POST", edge.URL, strings.NewReader("write once"))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.ProtoMajor != 2 || calls.Load() != 1 {
		t.Fatalf("proto=%s calls=%d", resp.Proto, calls.Load())
	}
}

func TestNoReplayAfterHTTPDelivery(t *testing.T) {
	var calls atomic.Int32
	edge := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		// The origin has consumed the POST but loses the response. Retrying it could
		// duplicate comments or chat messages, even if the other route is healthy.
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	edge.StartTLS()
	defer edge.Close()
	roots := x509.NewCertPool()
	roots.AddCert(edge.Certificate())
	transport := NewTransport("invalid", true)
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequestWithContext(t.Context(), "POST", edge.URL, strings.NewReader("write once"))
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected dropped response error")
	}
	if calls.Load() != 1 {
		t.Fatalf("POST delivered %d times", calls.Load())
	}
}
