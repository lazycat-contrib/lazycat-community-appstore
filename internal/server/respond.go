package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 1 << 20

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message, Details: details}})
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

func badRequest(w http.ResponseWriter, err error) {
	message := "Invalid request"
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		message = err.Error()
	}
	writeError(w, http.StatusBadRequest, "BAD_REQUEST", message, nil)
}
