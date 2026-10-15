package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lazycat.community/appstore/ent/user"
)

func TestMCPAcceptsTunnelDomainHostWithBearerToken(t *testing.T) {
	app := newTestApp(t)
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(t.Context())
	const token = "lcmcp_tunnel_host_test"
	app.server.db.MCPToken.Create().
		SetUserID(admin.ID).
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(t.Context())

	httpServer := httptest.NewServer(app.handler)
	t.Cleanup(httpServer.Close)
	client := &http.Client{Timeout: 5 * time.Second}
	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"tunnel-test","version":"1"}}}`
	for _, tc := range []struct {
		name       string
		host       string
		token      string
		wantStatus int
	}{
		{name: "domain with token", host: "apps.pushcat.eu.org", token: token, wantStatus: http.StatusOK},
		{name: "domain without token", host: "apps.pushcat.eu.org", wantStatus: http.StatusUnauthorized},
		{name: "domain with invalid token", host: "apps.pushcat.eu.org", token: "invalid", wantStatus: http.StatusUnauthorized},
		{name: "loopback with token", host: "127.0.0.1", token: token, wantStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, httpServer.URL+"/mcp", strings.NewReader(initialize))
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tc.host
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, tc.wantStatus, body)
			}
		})
	}
}
