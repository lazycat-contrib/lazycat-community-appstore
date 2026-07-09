package clientserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
)

func (s *Server) handleListSourceAppComments(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Comments") {
		return
	}
	appRecord, source, ok := s.sourceAppForComment(w, r)
	if !ok {
		return
	}
	endpoint, ok := sourceCommentEndpoint(w, source.URL, appRecord.ExternalID, "")
	if !ok {
		return
	}
	s.proxySourceCommentRequest(w, r, source, http.MethodGet, endpoint, nil)
}

func (s *Server) handleCreateSourceAppComment(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Comments") {
		return
	}
	appRecord, source, ok := s.sourceAppForComment(w, r)
	if !ok {
		return
	}
	if !appRecord.CommentsEnabled {
		writeError(w, http.StatusForbidden, "COMMENTS_DISABLED", "Comments are disabled for this app")
		return
	}
	var input CommentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	payload, err := json.Marshal(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_CREATE_FAILED", "Could not encode comment")
		return
	}
	endpoint, ok := sourceCommentEndpoint(w, source.URL, appRecord.ExternalID, "")
	if !ok {
		return
	}
	s.proxySourceCommentRequest(w, r, source, http.MethodPost, endpoint, bytes.NewReader(payload))
}

func (s *Server) handleDeleteSourceAppComment(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Comments") {
		return
	}
	_, source, ok := s.sourceAppForComment(w, r)
	if !ok {
		return
	}
	commentID, err := strconv.Atoi(r.PathValue("commentId"))
	if err != nil || commentID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment id")
		return
	}
	endpoint, ok := sourceCommentEndpoint(w, source.URL, strconv.Itoa(commentID), "delete")
	if !ok {
		return
	}
	s.proxySourceCommentRequest(w, r, source, http.MethodDelete, endpoint, nil)
}

func (s *Server) handleMarkSourceAppOutdated(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Outdated marks") {
		return
	}
	appRecord, source, ok := s.sourceAppForComment(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	endpoint, ok := sourceOutdatedEndpoint(w, source.URL, appRecord.ExternalID)
	if !ok {
		return
	}
	s.proxySourceCommentRequest(w, r, source, http.MethodPost, endpoint, bytes.NewReader(body))
}

func (s *Server) handleClearSourceAppOutdated(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Outdated marks") {
		return
	}
	appRecord, source, ok := s.sourceAppForComment(w, r)
	if !ok {
		return
	}
	endpoint, ok := sourceOutdatedEndpoint(w, source.URL, appRecord.ExternalID)
	if !ok {
		return
	}
	s.proxySourceCommentRequest(w, r, source, http.MethodDelete, endpoint, nil)
}

func (s *Server) sourceAppForComment(w http.ResponseWriter, r *http.Request) (*ent.ClientSourceApp, *ent.ClientSource, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid app id")
		return nil, nil, false
	}
	appRecord, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.IDEQ(id), clientsourceapp.HasSourceWith(clientsource.UserIDEQ(currentUserID(r)))).
		WithSource().
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found")
			return nil, nil, false
		}
		writeError(w, http.StatusInternalServerError, "APP_LOAD_FAILED", "Could not load app")
		return nil, nil, false
	}
	source, err := appRecord.Edges.SourceOrErr()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_LOAD_FAILED", "Could not load source")
		return nil, nil, false
	}
	return appRecord, source, true
}

func sourceCommentEndpoint(w http.ResponseWriter, sourceURL, id, kind string) (string, bool) {
	base, err := sourceAPIBase(sourceURL)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return "", false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_APP_ID_MISSING", "Source app id is missing")
		return "", false
	}
	if kind == "delete" {
		return base + "/api/v1/comments/" + url.PathEscape(id), true
	}
	return base + "/api/v1/apps/" + url.PathEscape(id) + "/comments", true
}

func sourceOutdatedEndpoint(w http.ResponseWriter, sourceURL, id string) (string, bool) {
	base, err := sourceAPIBase(sourceURL)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return "", false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_APP_ID_MISSING", "Source app id is missing")
		return "", false
	}
	return base + "/api/v1/apps/" + url.PathEscape(id) + "/outdated-marks", true
}

func sourceAPIBase(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/source/v2/index.json")
	parsed.Path = strings.TrimSuffix(parsed.Path, "/source/v1/index.json")
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (s *Server) proxySourceCommentRequest(w http.ResponseWriter, r *http.Request, source *ent.ClientSource, method, endpoint string, body io.Reader) {
	req, err := http.NewRequestWithContext(r.Context(), method, endpoint, body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-LazyCat-Client-User-ID", pseudonymousClientUserID(source.URL, currentUserID(r)))
	req.Header.Set("X-LazyCat-Client-Display-Name", s.clientCommentDisplayName(r))
	req.Header.Set("X-LazyCat-Client-Device-ID", strings.TrimSpace(r.Header.Get("x-hc-device-id")))
	req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
	if source.Password != "" {
		req.Header.Set("X-Source-Password", source.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "SOURCE_COMMENT_FAILED", "Could not reach source comments")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func pseudonymousClientUserID(sourceURL, userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceURL) + "\x00" + strings.TrimSpace(userID)))
	return "lc_" + hex.EncodeToString(sum[:])[:24]
}

func requireLazyCatClient(w http.ResponseWriter, r *http.Request, action string) bool {
	if currentUserID(r) == "local" || strings.TrimSpace(r.Header.Get("x-hc-device-id")) == "" {
		writeError(w, http.StatusForbidden, "LAZYCAT_CLIENT_REQUIRED", action+" require the app store client")
		return false
	}
	return true
}
