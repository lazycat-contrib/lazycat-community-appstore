package clientserver

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
)

func TestHTTPClientsUseBoundedAndStreamingTimeouts(t *testing.T) {
	ordinary, stream := newHTTPClients()
	if ordinary.Timeout != 30*time.Second {
		t.Fatalf("ordinary timeout = %v", ordinary.Timeout)
	}
	if stream.Timeout != 0 {
		t.Fatalf("stream timeout = %v, want zero", stream.Timeout)
	}
	for name, client := range map[string]*http.Client{"ordinary": ordinary, "stream": stream} {
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("%s transport = %T", name, client.Transport)
		}
		if transport.TLSHandshakeTimeout != 5*time.Second || transport.ResponseHeaderTimeout != 10*time.Second || transport.ExpectContinueTimeout != time.Second || transport.IdleConnTimeout != 90*time.Second || transport.MaxIdleConns != 100 || transport.MaxIdleConnsPerHost != 10 {
			t.Fatalf("%s transport bounds are incorrect: %+v", name, transport)
		}
	}
}

func TestLimitedRemoteResponseHelpers(t *testing.T) {
	var decoded map[string]string
	if err := decodeLimitedJSON(strings.NewReader(`{"name":"one"}{"name":"two"}`), 128, &decoded); err == nil || !strings.Contains(err.Error(), "one JSON value") {
		t.Fatalf("decodeLimitedJSON() error = %v", err)
	}
	if _, err := readLimitedBody(strings.NewReader("12345"), 4); err == nil {
		t.Fatal("readLimitedBody() error = nil")
	} else if tooLarge, ok := errors.AsType[*responseTooLargeError](err); !ok || tooLarge.Limit != 4 {
		t.Fatalf("readLimitedBody() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Header:     http.Header{"Content-Type": {"application/json"}, "X-Upstream": {"copied"}},
		Body:       io.NopCloser(strings.NewReader("12345")),
	}
	if err := writeBoundedSourceResponse(recorder, resp, 4); err == nil {
		t.Fatal("writeBoundedSourceResponse() error = nil")
	}
	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 || len(recorder.Header()) != 0 {
		t.Fatalf("recorder mutated before bounded read: code=%d headers=%v body=%q", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestNoRedirectClientPreservesIconBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "target")
	}))
	t.Cleanup(server.Close)
	ordinary, _ := newHTTPClients()
	resp, err := ordinary.Get(server.URL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ordinary status = %d", resp.StatusCode)
	}
	resp, err = noRedirectClient(ordinary).Get(server.URL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("no-redirect status = %d", resp.StatusCode)
	}
}

func TestStreamingClientHasNoTotalTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		<-timer.C
		_, _ = io.WriteString(w, "data: ready\n\n")
	}))
	t.Cleanup(server.Close)
	ordinary, stream := newHTTPClients()
	ordinary.Timeout = 20 * time.Millisecond
	if response, err := ordinary.Get(server.URL); err == nil {
		_ = response.Body.Close()
		t.Fatal("ordinary client unexpectedly outlived total timeout")
	}
	resp, err := stream.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "data: ready\n\n" {
		t.Fatalf("stream body = %q", raw)
	}
}

func TestSourceFeedRejectsActualSixtyFourMiBOverflow(t *testing.T) {
	var decoded any
	err := decodeLimitedJSON(io.LimitReader(zeroHTTPReader{}, maxSourceFeedBytes+1), maxSourceFeedBytes, &decoded)
	if tooLarge, ok := errors.AsType[*responseTooLargeError](err); !ok || tooLarge.Limit != maxSourceFeedBytes {
		t.Fatalf("decodeLimitedJSON() error = %v, want %d-byte responseTooLargeError", err, maxSourceFeedBytes)
	}
}

func TestSourceResponseBodyDecodesSupportedEncodings(t *testing.T) {
	const payload = `{"apps":[]}`
	var gzipBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipBuffer)
	_, _ = gzipWriter.Write([]byte(payload))
	_ = gzipWriter.Close()
	var brotliBuffer bytes.Buffer
	brotliWriter := brotli.NewWriter(&brotliBuffer)
	_, _ = brotliWriter.Write([]byte(payload))
	_ = brotliWriter.Close()

	for _, test := range []struct {
		name     string
		encoding string
		body     []byte
	}{
		{name: "identity", body: []byte(payload)},
		{name: "gzip", encoding: "gzip", body: gzipBuffer.Bytes()},
		{name: "brotli", encoding: "br", body: brotliBuffer.Bytes()},
	} {
		t.Run(test.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(test.body))}
			if test.encoding != "" {
				resp.Header.Set("Content-Encoding", test.encoding)
			}
			reader, err := sourceResponseBody(resp)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			raw, err := io.ReadAll(reader)
			if err != nil || string(raw) != payload {
				t.Fatalf("decoded = %q error=%v", raw, err)
			}
		})
	}

	for _, encoding := range []string{"zstd", "br, gzip"} {
		resp := &http.Response{Header: http.Header{"Content-Encoding": {encoding}}, Body: io.NopCloser(strings.NewReader(payload))}
		if _, err := sourceResponseBody(resp); err == nil {
			t.Fatalf("encoding %q error = nil", encoding)
		}
	}
}

func TestChatAndCommentRequestBodyBoundaries(t *testing.T) {
	for _, limit := range []int64{4096, 1 << 20} {
		raw, err := readLimitedBody(io.LimitReader(zeroHTTPReader{}, limit), limit)
		if err != nil || int64(len(raw)) != limit {
			t.Fatalf("readLimitedBody(exact %d) len=%d error=%v", limit, len(raw), err)
		}
		if _, err := readLimitedBody(io.LimitReader(zeroHTTPReader{}, limit+1), limit); err == nil {
			t.Fatalf("readLimitedBody(%d+1) error = nil", limit)
		} else if _, ok := errors.AsType[*responseTooLargeError](err); !ok {
			t.Fatalf("readLimitedBody(%d+1) error = %v", limit, err)
		}
	}
}

type zeroHTTPReader struct{}

func (zeroHTTPReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
