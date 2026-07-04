package clientserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lazycat.community/appstore/ent"
)

func TestSQLiteDSNAddsPragmas(t *testing.T) {
	dsn := sqliteDSN("./tmp/client.db")
	for _, part := range []string{
		"cache=shared",
		"_fk=1",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=busy_timeout(10000)",
	} {
		if !strings.Contains(dsn, part) {
			t.Fatalf("sqliteDSN missing %s in %q", part, dsn)
		}
	}
}

func TestClientSourceSchemaUserScopedUniqueness(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	defer client.Close()

	_, err := client.ClientSource.Create().
		SetUserID("alice").
		SetName("Community").
		SetURL("https://store.example/source/v1/index.json").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ClientSource.Create().
		SetUserID("alice").
		SetName("Duplicate").
		SetURL("https://store.example/source/v1/index.json").
		Save(ctx); err == nil {
		t.Fatal("expected duplicate url for same user to fail")
	}
	if _, err := client.ClientSource.Create().
		SetUserID("bob").
		SetName("Community").
		SetURL("https://store.example/source/v1/index.json").
		Save(ctx); err != nil {
		t.Fatalf("same url for different user failed: %v", err)
	}
}

func TestSourceCRUDIsUserScoped(t *testing.T) {
	app := testServer(t)

	alice := app.request("POST", "/api/client/v1/sources", `{"name":"A","url":"https://a.example/source/v1/index.json","password":"secret","mirror":"https://mirror.example"}`, "alice")
	if alice.Code != http.StatusCreated {
		t.Fatalf("create alice = %d %s", alice.Code, alice.Body.String())
	}
	bobList := app.request("GET", "/api/client/v1/sources", ``, "bob")
	if strings.Contains(bobList.Body.String(), "a.example") {
		t.Fatalf("bob saw alice source: %s", bobList.Body.String())
	}
	aliceList := app.request("GET", "/api/client/v1/sources", ``, "alice")
	if !strings.Contains(aliceList.Body.String(), "a.example") {
		t.Fatalf("alice source missing: %s", aliceList.Body.String())
	}
}

func TestSourceDuplicateURLForUserFails(t *testing.T) {
	app := testServer(t)
	body := `{"name":"A","url":"https://a.example/source/v1/index.json"}`
	if rec := app.request("POST", "/api/client/v1/sources", body, "alice"); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d", rec.Code)
	}
	rec := app.request("POST", "/api/client/v1/sources", body, "alice")
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate = %d %s", rec.Code, rec.Body.String())
	}
}

type clientTestServer struct {
	t       *testing.T
	server  *Server
	handler http.Handler
}

func testServer(t *testing.T) *clientTestServer {
	t.Helper()
	client := testClient(t)
	s := newTestServer(client)
	t.Cleanup(func() { _ = s.Close() })
	return &clientTestServer{t: t, server: s, handler: s.Handler()}
}

func (a *clientTestServer) request(method, target, body, userID string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("x-hc-user-id", userID)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func testClient(t *testing.T) *ent.Client {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client, err := ent.Open("sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	return client
}
