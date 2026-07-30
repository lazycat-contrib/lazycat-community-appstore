package clientserver

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
)

func (s *Server) handleListSourceWishes(w http.ResponseWriter, r *http.Request) {
	s.proxySourceWish(w, r, http.MethodGet, "", nil)
}

func (s *Server) handleCreateSourceWish(w http.ResponseWriter, r *http.Request) {
	body, ok := sourceWishBody(w, r)
	if !ok {
		return
	}
	s.proxySourceWish(w, r, http.MethodPost, "", bytes.NewReader(body))
}

func (s *Server) handleGetSourceWish(w http.ResponseWriter, r *http.Request) {
	s.proxySourceWish(w, r, http.MethodGet, r.PathValue("wishId"), nil)
}

func (s *Server) handleUpdateSourceWish(w http.ResponseWriter, r *http.Request) {
	body, ok := sourceWishBody(w, r)
	if !ok {
		return
	}
	s.proxySourceWish(w, r, http.MethodPatch, r.PathValue("wishId"), bytes.NewReader(body))
}

func (s *Server) handleDeleteSourceWish(w http.ResponseWriter, r *http.Request) {
	s.proxySourceWish(w, r, http.MethodDelete, r.PathValue("wishId"), nil)
}

func sourceWishBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := readLimitedBody(r.Body, 16<<10)
	if err != nil {
		if _, tooLarge := errors.AsType[*responseTooLargeError](err); tooLarge {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Wish request is too large")
		} else {
			writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid wish request")
		}
		return nil, false
	}
	return body, true
}

func (s *Server) proxySourceWish(w http.ResponseWriter, r *http.Request, method, wishID string, body io.Reader) {
	if !requireLazyCatClient(w, r, "Wish wall") {
		return
	}
	sourceID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || sourceID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_SOURCE_ID", "Invalid source id")
		return
	}
	source, err := s.db.ClientSource.Query().Where(clientsource.IDEQ(sourceID), clientsource.UserIDEQ(currentUserID(r))).Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "SOURCE_NOT_FOUND", "Source not found")
		} else {
			writeError(w, 500, "SOURCE_LOAD_FAILED", "Could not load source")
		}
		return
	}
	if !source.WishWallAvailable {
		writeError(w, http.StatusForbidden, "WISH_WALL_UNAVAILABLE", "This source does not support the wish wall")
		return
	}
	base, err := sourceAPIBase(source.URL)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return
	}
	endpoint := base + "/api/v1/wishes"
	if wishID != "" {
		parsedID, parseErr := strconv.Atoi(wishID)
		if parseErr != nil || parsedID <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_WISH_ID", "Invalid wish id")
			return
		}
		endpoint += "/" + url.PathEscape(strconv.Itoa(parsedID))
	} else if method == http.MethodGet {
		query := url.Values{}
		for _, key := range []string{"kind", "status", "page", "pageSize"} {
			if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
				query.Set(key, value)
			}
		}
		if encoded := query.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	}
	s.ensureHTTPClients()
	req, err := http.NewRequestWithContext(r.Context(), method, endpoint, body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SOURCE_URL_INVALID", "Source URL is invalid")
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-LazyCat-Client-User-ID", pseudonymousClientUserID(source.URL, currentUserID(r)))
	req.Header.Set("X-LazyCat-Client-Display-Name", s.clientCommentDisplayName(r))
	req.Header.Set("X-LazyCat-Client-Device-ID", strings.TrimSpace(r.Header.Get("x-hc-device-id")))
	req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
	if source.Password != "" {
		req.Header.Set("X-Source-Password", source.Password)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "SOURCE_WISH_FAILED", "Could not reach source wish wall")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if err := writeBoundedSourceResponse(w, resp, maxSourceProxyResponseBytes); err != nil {
		if _, tooLarge := errors.AsType[*responseTooLargeError](err); tooLarge {
			writeError(w, http.StatusBadGateway, "SOURCE_RESPONSE_TOO_LARGE", "Source response is too large")
		}
	}
}
