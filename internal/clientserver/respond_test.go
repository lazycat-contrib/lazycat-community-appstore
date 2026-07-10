package clientserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsTrailingAndOversizedBodies(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	tests := []struct {
		name    string
		body    string
		wantErr string
		tooBig  bool
	}{
		{name: "valid", body: `{"name":"one"}`},
		{name: "unknown", body: `{"name":"one","extra":true}`, wantErr: "unknown field"},
		{name: "trailing", body: `{"name":"one"}{"name":"two"}`, wantErr: "single JSON value"},
		{name: "oversized", body: `{"name":"` + strings.Repeat("x", int(maxJSONBodyBytes)) + `"}`, tooBig: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			var got payload
			err := decodeJSON(req, &got)
			if tt.tooBig {
				if _, ok := errors.AsType[*http.MaxBytesError](err); !ok {
					t.Fatalf("decodeJSON() error = %v, want MaxBytesError", err)
				}
				return
			}
			if tt.wantErr == "" && err != nil {
				t.Fatalf("decodeJSON() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("decodeJSON() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
