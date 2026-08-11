package mirrorprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"lazycat.community/appstore/internal/lpkinspect"
	"lazycat.community/appstore/internal/mirror"
)

const (
	SampleBytes    int64 = 512 << 10
	RequestTimeout       = 8 * time.Second
	MaxConcurrency       = 4
)

type ProbeFunc func(context.Context, string) (int64, error)

type Measurement struct {
	Entry          mirror.Entry
	BytesPerSecond int64
	Successful     bool
	Index          int
}

func Probe(ctx context.Context, rawURL string) (int64, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" || parsed.User != nil {
		return 0, errors.New("mirror benchmark URL must use HTTPS without credentials")
	}
	if err := lpkinspect.ValidateURLHost(ctx, parsed, false); err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", SampleBytes-1))
	req.Header.Set("User-Agent", "MiaoMiao-Mirror-Benchmark/1")

	startedAt := time.Now()
	resp, err := benchmarkHTTPClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("mirror benchmark returned HTTP %d", resp.StatusCode)
	}
	sample := make([]byte, 512)
	sampleBytes, err := io.ReadFull(resp.Body, sample)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return 0, err
	}
	if err := validateProbeSample(sample[:sampleBytes]); err != nil {
		return 0, err
	}
	written, err := io.Copy(io.Discard, io.LimitReader(resp.Body, SampleBytes-int64(sampleBytes)))
	if err != nil {
		return 0, err
	}
	written += int64(sampleBytes)
	if written <= 0 {
		return 0, errors.New("mirror benchmark returned an empty response")
	}
	elapsed := time.Since(startedAt)
	if elapsed < time.Millisecond {
		elapsed = time.Millisecond
	}
	return int64(float64(written) / elapsed.Seconds()), nil
}

func validateProbeSample(sample []byte) error {
	if len(sample) == 0 {
		return errors.New("mirror benchmark returned an empty response")
	}
	mediaType, _, err := mime.ParseMediaType(http.DetectContentType(sample))
	if err != nil {
		return errors.New("mirror benchmark returned an unrecognized response")
	}
	if strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		return fmt.Errorf("mirror benchmark returned %s instead of a package", mediaType)
	}
	return nil
}

func Measure(ctx context.Context, entries []mirror.Entry, upstreamURL string, probe ProbeFunc) []Measurement {
	if probe == nil {
		probe = Probe
	}
	applicable := mirror.OrderedForURL(entries, upstreamURL, "")
	results := make([]Measurement, len(applicable))
	sem := make(chan struct{}, MaxConcurrency)
	var wg sync.WaitGroup
	for index, entry := range applicable {
		wg.Go(func() {
			result := Measurement{Entry: entry, Index: index}
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = result
				return
			}
			bps, err := probe(ctx, mirror.RewriteGitHub(upstreamURL, entry))
			if err == nil && bps > 0 {
				result.BytesPerSecond = bps
				result.Successful = true
			}
			results[index] = result
		})
	}
	wg.Wait()
	return results
}

func Fastest(measurements []Measurement) (mirror.Entry, bool) {
	ordered := slices.Clone(measurements)
	slices.SortStableFunc(ordered, func(left, right Measurement) int {
		if left.Successful != right.Successful {
			if left.Successful {
				return -1
			}
			return 1
		}
		if left.BytesPerSecond != right.BytesPerSecond {
			if left.BytesPerSecond > right.BytesPerSecond {
				return -1
			}
			return 1
		}
		if left.Index != right.Index {
			return left.Index - right.Index
		}
		return strings.Compare(left.Entry.ID, right.Entry.ID)
	})
	if len(ordered) == 0 || !ordered[0].Successful {
		return mirror.Entry{}, false
	}
	return ordered[0].Entry, true
}

func benchmarkHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeDialContext
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 5 * time.Second
	transport.ExpectContinueTimeout = time.Second
	transport.MaxIdleConns = MaxConcurrency
	transport.MaxIdleConnsPerHost = 1
	transport.IdleConnTimeout = 15 * time.Second
	return &http.Client{
		Transport: transport,
		Timeout:   RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return errors.New("too many mirror benchmark redirects")
			}
			if req.URL.Scheme != "https" || req.URL.User != nil {
				return errors.New("mirror benchmark redirect must use HTTPS without credentials")
			}
			return lpkinspect.ValidateURLHost(req.Context(), req.URL, false)
		},
	}
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("mirror benchmark host did not resolve")
	}
	for _, addr := range addrs {
		if lpkinspect.UnsafeIP(addr.IP) {
			return nil, errors.New("mirror benchmark host resolved to a private or local address")
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 15 * time.Second}
	var dialErr error
	for _, addr := range addrs {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = errors.Join(dialErr, err)
	}
	return nil, dialErr
}
