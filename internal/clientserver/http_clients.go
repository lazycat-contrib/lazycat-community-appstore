package clientserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	maxSourceFeedBytes          int64 = 64 << 20
	maxSourceProxyResponseBytes int64 = 1 << 20
)

type responseTooLargeError struct {
	Limit int64
}

func (e *responseTooLargeError) Error() string {
	return fmt.Sprintf("response exceeds %d bytes", e.Limit)
}

func newHTTPTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.ExpectContinueTimeout = time.Second
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	return transport
}

func newHTTPClients() (*http.Client, *http.Client) {
	return &http.Client{Transport: newHTTPTransport(), Timeout: 30 * time.Second},
		&http.Client{Transport: newHTTPTransport()}
}

func noRedirectClient(base *http.Client) *http.Client {
	clone := *base
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

func readLimitedBody(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, &responseTooLargeError{Limit: maxBytes}
	}
	return raw, nil
}

func decodeLimitedJSON(r io.Reader, maxBytes int64, out any) error {
	raw, err := readLimitedBody(r, maxBytes)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("response must contain one JSON value")
		}
		return fmt.Errorf("response must contain one JSON value: %w", err)
	}
	return nil
}

func writeBoundedSourceResponse(w http.ResponseWriter, resp *http.Response, maxBytes int64) error {
	raw, err := readLimitedBody(resp.Body, maxBytes)
	if err != nil {
		return err
	}
	if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(raw)
	return err
}
