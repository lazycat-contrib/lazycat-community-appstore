package cfnetwork

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

// Pool bounds connection pools across user-selected endpoints. No user credentials
// or selection are stored here; each request independently chooses its transport.
type Pool struct {
	mu         sync.Mutex
	transports map[string]*http.Transport
}

func (p *Pool) Transport(endpoint string) *http.Transport {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.transports == nil {
		p.transports = make(map[string]*http.Transport)
	}
	if transport := p.transports[endpoint]; transport != nil {
		return transport
	}
	if len(p.transports) >= 32 {
		for key, transport := range p.transports {
			transport.CloseIdleConnections()
			delete(p.transports, key)
			break
		}
	}
	transport := NewTransport(endpoint, true)
	p.transports[endpoint] = transport
	return transport
}

func (p *Pool) CloseIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, transport := range p.transports {
		transport.CloseIdleConnections()
	}
}

// NewTransport bypasses environment proxies for the preferred connection, as a
// proxy would choose the destination itself. Fallback happens before sending HTTP
// bytes, so POST requests are never replayed. TLS always authenticates the origin.
func NewTransport(endpoint string, fallback bool) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.MaxIdleConnsPerHost = 10
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		tlsConfig := &tls.Config{}
		if transport.TLSClientConfig != nil {
			tlsConfig = transport.TLSClientConfig.Clone()
		}
		tlsConfig.ServerName = host
		tlsConfig.MinVersion = tls.VersionTLS12
		tlsConfig.InsecureSkipVerify = false
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		preferredCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		conn, preferredErr := dialPreferred(preferredCtx, network, port, endpoint, tlsConfig)
		cancel()
		if preferredErr == nil {
			return conn, nil
		}
		if !fallback || ctx.Err() != nil {
			return nil, preferredErr
		}
		dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}, Config: tlsConfig}
		return dialer.DialContext(ctx, network, addr)
	}
	return transport
}

func dialPreferred(ctx context.Context, network, port, endpoint string, tlsConfig *tls.Config) (net.Conn, error) {
	endpoint, err := Normalize(endpoint)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", endpoint)
	if err != nil {
		return nil, err
	}
	// Validate the resolved addresses and dial the checked IP, preventing DNS rebinding.
	return dialResolved(ctx, network, port, ips, tlsConfig, publicIP)
}

func dialResolved(ctx context.Context, network, port string, ips []netip.Addr, config *tls.Config, allowed func(netip.Addr) bool) (net.Conn, error) {
	lastErr := errors.New("preferred domain has no public addresses")
	for _, ip := range ips {
		if !allowed(ip) {
			continue
		}
		attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
		dialer := tls.Dialer{NetDialer: &net.Dialer{KeepAlive: 30 * time.Second}, Config: config}
		conn, err := dialer.DialContext(attemptCtx, network, net.JoinHostPort(ip.String(), port))
		cancel()
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

// OriginTransport constrains preferred routing, including redirects, to one HTTPS origin.
type OriginTransport struct {
	Origin    string
	Preferred http.RoundTripper
	Direct    http.RoundTripper
}

func (t OriginTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "https" && req.URL.Host == t.Origin {
		return t.Preferred.RoundTrip(req)
	}
	return t.Direct.RoundTrip(req)
}
