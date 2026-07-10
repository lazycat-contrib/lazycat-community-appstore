package clientserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 1 << 20

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var out ErrorResponse
	out.Error.Code = code
	out.Error.Message = message
	writeJSON(w, status, out)
}

func decodeJSON(r *http.Request, out any) error {
	limited := &io.LimitedReader{R: r.Body, N: maxJSONBodyBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		if limited.N == 0 {
			return &http.MaxBytesError{Limit: maxJSONBodyBytes}
		}
		return err
	}
	var extra any
	err := decoder.Decode(&extra)
	if limited.N == 0 {
		return &http.MaxBytesError{Limit: maxJSONBodyBytes}
	}
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return fmt.Errorf("request body must contain a single JSON value: %w", err)
	}
	return nil
}
