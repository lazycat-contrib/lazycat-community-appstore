package clientserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
)

func (s *Server) handleListChatConversations(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Chat") {
		return
	}
	source, ok := s.sourceForClientChat(w, r)
	if !ok {
		return
	}
	endpoint, ok := sourceChatEndpoint(w, source.URL, "/api/v1/chat/conversations")
	if !ok {
		return
	}
	s.proxySourceChatRequest(w, r, source, http.MethodGet, endpoint, nil)
}

func (s *Server) handleCreateAppChatConversation(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Chat") {
		return
	}
	appRecord, source, ok := s.sourceAppForClientChat(w, r)
	if !ok {
		return
	}
	body, err := readLimitedBody(r.Body, 1<<20)
	if err != nil {
		if _, ok := errors.AsType[*responseTooLargeError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	endpoint, ok := sourceChatEndpoint(w, source.URL, "/api/v1/apps/"+url.PathEscape(appRecord.ExternalID)+"/chat")
	if !ok {
		return
	}
	s.proxySourceChatRequest(w, r, source, http.MethodPost, endpoint, bytes.NewReader(body))
}

func (s *Server) handleListChatMessages(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Chat") {
		return
	}
	source, ok := s.sourceForClientChat(w, r)
	if !ok {
		return
	}
	conversationID, ok := clientChatConversationID(w, r)
	if !ok {
		return
	}
	endpoint, ok := sourceChatEndpoint(w, source.URL, "/api/v1/chat/conversations/"+strconv.Itoa(conversationID)+"/messages")
	if !ok {
		return
	}
	s.proxySourceChatRequest(w, r, source, http.MethodGet, endpoint, nil)
}

func (s *Server) handleCreateChatMessage(w http.ResponseWriter, r *http.Request) {
	if !requireLazyCatClient(w, r, "Chat") {
		return
	}
	source, ok := s.sourceForClientChat(w, r)
	if !ok {
		return
	}
	conversationID, ok := clientChatConversationID(w, r)
	if !ok {
		return
	}
	body, err := readLimitedBody(r.Body, 1<<20)
	if err != nil {
		if _, ok := errors.AsType[*responseTooLargeError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}
	endpoint, ok := sourceChatEndpoint(w, source.URL, "/api/v1/chat/conversations/"+strconv.Itoa(conversationID)+"/messages")
	if !ok {
		return
	}
	s.proxySourceChatRequest(w, r, source, http.MethodPost, endpoint, bytes.NewReader(body))
}

func (s *Server) handleReadChatConversation(w http.ResponseWriter, r *http.Request) {
	s.proxyChatConversationAction(w, r, "read", http.MethodPost)
}

func (s *Server) handleDeleteChatConversation(w http.ResponseWriter, r *http.Request) {
	s.proxyChatConversationAction(w, r, "", http.MethodDelete)
}

func (s *Server) proxyChatConversationAction(w http.ResponseWriter, r *http.Request, suffix string, method string) {
	if !requireLazyCatClient(w, r, "Chat") {
		return
	}
	source, ok := s.sourceForClientChat(w, r)
	if !ok {
		return
	}
	conversationID, ok := clientChatConversationID(w, r)
	if !ok {
		return
	}
	path := "/api/v1/chat/conversations/" + strconv.Itoa(conversationID)
	if suffix != "" {
		path += "/" + suffix
	}
	endpoint, ok := sourceChatEndpoint(w, source.URL, path)
	if !ok {
		return
	}
	s.proxySourceChatRequest(w, r, source, method, endpoint, nil)
}

func (s *Server) handleChatEvents(w http.ResponseWriter, r *http.Request) {
	s.ensureHTTPClients()
	if !requireLazyCatClient(w, r, "Chat") {
		return
	}
	source, ok := s.sourceForClientChat(w, r)
	if !ok {
		return
	}
	endpoint, ok := sourceChatEndpoint(w, source.URL, "/api/v1/chat/events")
	if !ok {
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return
	}
	applySourceProxyHeaders(req, r, source, s.clientCommentDisplayName(r))
	resp, err := s.streamClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "SOURCE_CHAT_FAILED", "Could not reach source chat")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	for key, values := range resp.Header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	w.WriteHeader(resp.StatusCode)
	if err := copySSE(r.Context(), w, resp.Body, 30*time.Second); err != nil {
		return
	}
}

func copySSE(ctx context.Context, w http.ResponseWriter, src io.Reader, deadline time.Duration) error {
	controller := http.NewResponseController(w)
	buf := make([]byte, 32<<10)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if deadline > 0 {
				if err := controller.SetWriteDeadline(time.Now().Add(deadline)); err != nil && !errors.Is(err, http.ErrNotSupported) {
					return err
				}
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
			if err := controller.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (s *Server) sourceForClientChat(w http.ResponseWriter, r *http.Request) (*ent.ClientSource, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("sourceId")))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_SOURCE_ID", "Source id is required")
		return nil, false
	}
	source, err := s.db.ClientSource.Query().
		Where(clientsource.IDEQ(id), clientsource.UserIDEQ(currentUserID(r))).
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "SOURCE_NOT_FOUND", "Source not found")
			return nil, false
		}
		writeError(w, http.StatusInternalServerError, "SOURCE_LOAD_FAILED", "Could not load source")
		return nil, false
	}
	if !source.ChatAvailable || !source.ChatEnabled {
		writeError(w, http.StatusForbidden, "CHAT_DISABLED", "Chat is disabled for this source")
		return nil, false
	}
	return source, true
}

func (s *Server) sourceAppForClientChat(w http.ResponseWriter, r *http.Request) (*ent.ClientSourceApp, *ent.ClientSource, bool) {
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
	if !source.ChatAvailable || !source.ChatEnabled {
		writeError(w, http.StatusForbidden, "CHAT_DISABLED", "Chat is disabled for this source")
		return nil, nil, false
	}
	return appRecord, source, true
}

func clientChatConversationID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid conversation id")
		return 0, false
	}
	return id, true
}

func sourceChatEndpoint(w http.ResponseWriter, sourceURL, apiPath string) (string, bool) {
	base, err := sourceAPIBase(sourceURL)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return "", false
	}
	return base + apiPath, true
}

func (s *Server) proxySourceChatRequest(w http.ResponseWriter, r *http.Request, source *ent.ClientSource, method, endpoint string, body io.Reader) {
	s.ensureHTTPClients()
	req, err := http.NewRequestWithContext(r.Context(), method, endpoint, body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	applySourceProxyHeaders(req, r, source, s.clientCommentDisplayName(r))
	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "SOURCE_CHAT_FAILED", "Could not reach source chat")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	err = writeBoundedSourceResponse(w, resp, maxSourceProxyResponseBytes)
	if _, ok := errors.AsType[*responseTooLargeError](err); ok {
		writeError(w, http.StatusBadGateway, "SOURCE_RESPONSE_TOO_LARGE", "Source response is too large")
		return
	}
	if err != nil {
		return
	}
}

func applySourceProxyHeaders(req *http.Request, r *http.Request, source *ent.ClientSource, displayName string) {
	req.Header.Set("X-LazyCat-Client-User-ID", pseudonymousClientUserID(source.URL, currentUserID(r)))
	req.Header.Set("X-LazyCat-Client-Display-Name", displayName)
	req.Header.Set("X-LazyCat-Client-Device-ID", strings.TrimSpace(r.Header.Get("x-hc-device-id")))
	req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
	if source.Password != "" {
		req.Header.Set("X-Source-Password", source.Password)
	}
	if codes := decodeStringSlice(source.GroupCodesJSON); len(codes) > 0 {
		req.Header.Set("X-Group-Codes", strings.Join(codes, ","))
	}
}
