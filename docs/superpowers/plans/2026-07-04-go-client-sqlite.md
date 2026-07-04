# Go SQLite Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the standalone LazyCat client as a Go backend with embedded React UI, ent + SQLite source storage, Go-backed source sync, and LazyCat Go SDK LPK installation.

**Architecture:** Add a new `cmd/store-client` binary and `internal/clientserver` package beside the existing server. The client backend owns SQLite persistence and exposes `/api/client/v1/*`; the React UI switches from browser `localStorage` durable data to those APIs while retaining theme and language browser preferences.

**Tech Stack:** Go 1.26.4, ent v0.14.6, SQLite via the existing `github.com/mattn/go-sqlite3` driver, React 19/Vite 7, LazyCat Go SDK `gitee.com/linakesi/lzc-sdk@v0.1.0`.

## Global Constraints

- Keep `lazycat/server` and `lazycat/client` independently deployable.
- Client database path is `/lzcapp/var/data/client.db` in LazyCat and `./data/client.db` locally.
- Durable client data is limited to software sources and cached source apps.
- Resolve user id from `x-hc-user-id`, with local development fallback `local`.
- All source and cached app queries must be scoped by user id.
- Expose standalone client APIs under `/api/client/v1`.
- The Go SDK is the only intended installation entrypoint.
- Keep `lazycat.theme` and i18next language cache in browser storage.
- Do not create a generic settings table in the first implementation.
- Do not add speculative LazyCat manifest permissions.
- UI changes must keep the existing restrained operational style and respect `prefers-reduced-motion`.

---

## File Structure

- Create `cmd/store-client/main.go`: standalone client binary entrypoint.
- Create `internal/clientserver/config.go`: client-specific environment config.
- Create `internal/clientserver/server.go`: HTTP server construction, routing, embedded frontend handler.
- Create `internal/clientserver/db.go`: ent open/migrate helpers and SQLite directory creation.
- Create `internal/clientserver/types.go`: API DTOs, source sync error codes, install DTOs.
- Create `internal/clientserver/respond.go`: JSON and error response helpers copied in shape from `internal/server/respond.go`.
- Create `internal/clientserver/user.go`: LazyCat user id extraction.
- Create `internal/clientserver/sources.go`: source CRUD handlers.
- Create `internal/clientserver/sync.go`: source feed sync logic.
- Create `internal/clientserver/apps.go`: cached app and installed-app handlers.
- Create `internal/clientserver/install.go`: install handler and LazyCat package manager interface.
- Create `internal/clientserver/lazycat.go`: real Go SDK adapter.
- Create `internal/clientserver/server_test.go`: handler and sync tests.
- Create `ent/schema/client_source.go`: ent schema for client source subscriptions.
- Create `ent/schema/client_source_app.go`: ent schema for synced source app cache.
- Modify generated `ent/*`: generated code for the two schemas.
- Modify `go.mod` and `go.sum`: add LazyCat Go SDK module.
- Modify `client/src/App.tsx`: replace durable localStorage source/app state and JS SDK calls with client APIs.
- Modify `client/src/lazycatSdk.ts`: remove or stop using JS SDK install/query wrappers.
- Modify `client/package.json` and `client/package-lock.json`: remove `@lazycatcloud/sdk` if unused after migration.
- Modify `client/src/styles.css`: shared UI polish for server/client modes.
- Modify `lazycat/client/build.sh`: build frontend, copy assets into client embed directory, build `store-client`.
- Modify `lazycat/client/lzc-manifest.yml`: launch Go backend instead of static `file://` upstream.
- Modify `README.md`: document durable standalone client storage and local client backend command.

---

### Task 1: Client ent Schema And Database Opening

**Files:**
- Create: `ent/schema/client_source.go`
- Create: `ent/schema/client_source_app.go`
- Create: `internal/clientserver/config.go`
- Create: `internal/clientserver/db.go`
- Test: `internal/clientserver/server_test.go`
- Modify: generated `ent/*`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `type Config struct { Addr string; DBDSN string; DefaultSourceURL string; DefaultSourceName string; SyncTimeout time.Duration }`
- Produces: `func LoadConfig() Config`
- Produces: `func openDB(cfg Config) (*ent.Client, error)`
- Produces: `func sqliteDSN(path string) string`
- Produces ent clients: `client.ClientSource` and `client.ClientSourceApp`.

- [ ] **Step 1: Write failing schema/DB tests**

Add these tests to `internal/clientserver/server_test.go`:

```go
package clientserver

import (
	"context"
	"strings"
	"testing"
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
```

Add `testClient(t)` in the same file:

```go
func testClient(t *testing.T) *ent.Client {
	t.Helper()
	client, err := ent.Open("sqlite3", "file:ent-client?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	return client
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/clientserver -run 'TestSQLiteDSNAddsPragmas|TestClientSourceSchemaUserScopedUniqueness'
```

Expected: FAIL because `internal/clientserver` and `ClientSource` do not exist.

- [ ] **Step 3: Add ent schemas**

Create `ent/schema/client_source.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientSource struct {
	ent.Schema
}

func (ClientSource) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").NotEmpty(),
		field.String("name").NotEmpty(),
		field.String("url").NotEmpty(),
		field.String("password").Default(""),
		field.String("mirror").Default(""),
		field.Time("last_sync").Optional().Nillable(),
		field.String("last_error").Optional().Nillable(),
		field.Enum("last_error_code").Values("auth", "format", "http", "network").Optional().Nillable(),
		field.Int("last_app_count").Default(0),
		field.Int("last_installable_count").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "url").Unique(),
		index.Fields("user_id", "updated_at"),
	}
}
```

Create `ent/schema/client_source_app.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientSourceApp struct {
	ent.Schema
}

func (ClientSourceApp) Fields() []ent.Field {
	return []ent.Field{
		field.Int("source_id"),
		field.String("external_id").Default(""),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty(),
		field.String("summary").Default(""),
		field.String("category").Default(""),
		field.Bool("install_protected").Default(false),
		field.Text("latest_version_json").Default(""),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientSourceApp) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", ClientSource.Type).
			Ref("apps").
			Field("source_id").
			Required().
			Unique(),
	}
}

func (ClientSourceApp) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_id", "slug").Unique(),
		index.Fields("source_id", "updated_at"),
	}
}
```

Add the reverse edge to `ClientSource`:

```go
func (ClientSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("apps", ClientSourceApp.Type),
	}
}
```

Import `entgo.io/ent/schema/edge` in `client_source.go`.

- [ ] **Step 4: Add config and DB helpers**

Create `internal/clientserver/config.go`:

```go
package clientserver

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Addr              string
	DBDSN             string
	DefaultSourceURL  string
	DefaultSourceName string
	SyncTimeout      time.Duration
}

func LoadConfig() Config {
	return Config{
		Addr:              env("CLIENT_ADDR", "127.0.0.1:8090"),
		DBDSN:             env("CLIENT_DB_DSN", "./data/client.db"),
		DefaultSourceURL:  strings.TrimSpace(os.Getenv("CLIENT_DEFAULT_SOURCE_URL")),
		DefaultSourceName: env("CLIENT_DEFAULT_SOURCE_NAME", "Community Store"),
		SyncTimeout:      20 * time.Second,
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
```

Create `internal/clientserver/db.go`:

```go
package clientserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"lazycat.community/appstore/ent"
)

func openDB(cfg Config) (*ent.Client, error) {
	if err := ensureSQLiteDir(cfg.DBDSN); err != nil {
		return nil, err
	}
	client, err := ent.Open("sqlite3", sqliteDSN(cfg.DBDSN))
	if err != nil {
		return nil, err
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func sqliteDSN(dsn string) string {
	if strings.HasPrefix(dsn, "file:") || strings.Contains(dsn, "?") {
		return dsn
	}
	return "file:" + dsn + "?cache=shared&_fk=1&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"
}

func ensureSQLiteDir(dsn string) error {
	dsn = strings.TrimPrefix(dsn, "file:")
	if i := strings.IndexByte(dsn, '?'); i >= 0 {
		dsn = dsn[:i]
	}
	if dsn == "" || dsn == ":memory:" {
		return nil
	}
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
```

- [ ] **Step 5: Generate ent code and add SDK module**

Run:

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
go get gitee.com/linakesi/lzc-sdk@v0.1.0
go mod tidy
```

Expected: generated ent files include `client_source*` and `client_source_app*`; `go.mod` includes `gitee.com/linakesi/lzc-sdk v0.1.0`.

- [ ] **Step 6: Run task tests**

Run:

```bash
go test ./internal/clientserver -run 'TestSQLiteDSNAddsPragmas|TestClientSourceSchemaUserScopedUniqueness'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add ent go.mod go.sum internal/clientserver
git commit -m "feat: add client sqlite schema"
```

---

### Task 2: Source CRUD API

**Files:**
- Create: `internal/clientserver/respond.go`
- Create: `internal/clientserver/types.go`
- Create: `internal/clientserver/user.go`
- Create: `internal/clientserver/server.go`
- Create: `internal/clientserver/sources.go`
- Modify: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: `Config`, `openDB`, ent `ClientSource`.
- Produces: `func New(cfg Config) (*Server, error)`
- Produces: `func (s *Server) Handler() http.Handler`
- Produces: `func currentUserID(r *http.Request) string`
- Produces DTOs: `SourceDTO`, `SourceInput`, `ErrorResponse`.

- [ ] **Step 1: Write failing source CRUD handler tests**

Append tests:

```go
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
```

Add a test helper:

```go
type clientTestServer struct {
	t       *testing.T
	server  *Server
	handler http.Handler
}

func testServer(t *testing.T) *clientTestServer {
	t.Helper()
	client := testClient(t)
	s := newTestServer(client)
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
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/clientserver -run 'TestSourceCRUDIsUserScoped|TestSourceDuplicateURLForUserFails'
```

Expected: FAIL because server and handlers do not exist.

- [ ] **Step 3: Add DTOs and response helpers**

Create `internal/clientserver/types.go`:

```go
package clientserver

import "time"

type SourceDTO struct {
	ID                   int        `json:"id"`
	Name                 string     `json:"name"`
	URL                  string     `json:"url"`
	Password             string     `json:"password"`
	Mirror               string     `json:"mirror"`
	LastSync             *time.Time `json:"lastSync,omitempty"`
	LastError            string     `json:"lastError,omitempty"`
	LastErrorCode        string     `json:"lastErrorCode,omitempty"`
	LastAppCount         int        `json:"lastAppCount"`
	LastInstallableCount int        `json:"lastInstallableCount"`
}

type SourceInput struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Password string `json:"password"`
	Mirror   string `json:"mirror"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
```

Create `internal/clientserver/respond.go`:

```go
package clientserver

import (
	"encoding/json"
	"net/http"
)

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
```

Create `internal/clientserver/user.go`:

```go
package clientserver

import (
	"net/http"
	"strings"
)

func currentUserID(r *http.Request) string {
	userID := strings.TrimSpace(r.Header.Get("x-hc-user-id"))
	if userID == "" {
		return "local"
	}
	return userID
}
```

- [ ] **Step 4: Add server shell and source handlers**

Create `internal/clientserver/server.go`:

```go
package clientserver

import (
	"net/http"

	"lazycat.community/appstore/ent"
)

type Server struct {
	cfg Config
	db  *ent.Client
	mux *http.ServeMux
}

func New(cfg Config) (*Server, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, db: db, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func newTestServer(db *ent.Client) *Server {
	s := &Server{cfg: Config{DefaultSourceName: "Community Store"}, db: db, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Close() error { return s.db.Close() }

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/client/v1/sources", s.handleListSources)
	s.mux.HandleFunc("POST /api/client/v1/sources", s.handleCreateSource)
	s.mux.HandleFunc("PATCH /api/client/v1/sources/{id}", s.handleUpdateSource)
	s.mux.HandleFunc("DELETE /api/client/v1/sources/{id}", s.handleDeleteSource)
}
```

Create `internal/clientserver/sources.go` with handlers that trim input, validate non-empty name and absolute HTTP(S) URL, scope all queries by `currentUserID(r)`, and map ent unique constraint failures to HTTP 409.

- [ ] **Step 5: Run task tests**

Run:

```bash
go test ./internal/clientserver -run 'TestSourceCRUDIsUserScoped|TestSourceDuplicateURLForUserFails'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/clientserver
git commit -m "feat: add client source api"
```

---

### Task 3: Source Sync And Cached App API

**Files:**
- Create: `internal/clientserver/sync.go`
- Create: `internal/clientserver/apps.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/server.go`
- Modify: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: source CRUD API and ent source/app schema.
- Produces: `func (s *Server) syncSource(ctx context.Context, sourceID int, userID string) (SourceDTO, error)`
- Produces API DTOs: `SourceAppDTO`, `VersionDTO`, `SyncAllResult`.

- [ ] **Step 1: Write failing sync tests**

Add tests:

```go
func TestSyncSourceCachesAppsAndUpdatesSource(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Source-Password"); got != "pw" {
			t.Fatalf("password header = %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{"apps": []map[string]any{{
			"id": 7, "name": "Notes", "slug": "notes", "summary": "Write notes", "category": "tools",
			"latestVersion": map[string]any{
				"version": "1.2.3", "downloadUrl": "https://github.com/org/notes/releases/download/a/notes.lpk",
				"upstreamDownloadUrl": "https://github.com/org/notes/releases/download/a/notes.lpk",
				"sha256": strings.Repeat("a", 64), "size": 123,
			},
		}}})
	}))
	defer feed.Close()

	app := testServer(t)
	create := app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`","password":"pw","mirror":"https://ghproxy.example"}`, "alice")
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	apps := app.request("GET", "/api/client/v1/apps", ``, "alice")
	body := apps.Body.String()
	if !strings.Contains(body, `"slug":"notes"`) || !strings.Contains(body, "https://ghproxy.example/https://github.com/org/notes") {
		t.Fatalf("cached app missing mirror rewrite: %s", body)
	}
}

func TestSyncSourceAuthFailureIsStored(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer feed.Close()
	app := testServer(t)
	_ = app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`"}`, "alice")
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	sources := app.request("GET", "/api/client/v1/sources", ``, "alice")
	if !strings.Contains(sources.Body.String(), `"lastErrorCode":"auth"`) {
		t.Fatalf("auth code not stored: %s", sources.Body.String())
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/clientserver -run 'TestSyncSourceCachesAppsAndUpdatesSource|TestSyncSourceAuthFailureIsStored'
```

Expected: FAIL because sync routes do not exist.

- [ ] **Step 3: Add app DTOs**

Extend `types.go`:

```go
type VersionDTO struct {
	Version             string `json:"version"`
	DownloadURL         string `json:"downloadUrl"`
	UpstreamDownloadURL string `json:"upstreamDownloadUrl,omitempty"`
	SourceType          string `json:"sourceType,omitempty"`
	SHA256              string `json:"sha256"`
	Size                int64  `json:"size"`
}

type SourceAppDTO struct {
	ID               int         `json:"id"`
	SourceID         int         `json:"sourceId"`
	SourceName       string      `json:"sourceName"`
	Name             string      `json:"name"`
	Slug             string      `json:"slug"`
	Summary          string      `json:"summary"`
	Category         string      `json:"category,omitempty"`
	InstallProtected bool        `json:"installProtected"`
	LatestVersion    *VersionDTO `json:"latestVersion,omitempty"`
}

type SyncAllResult struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
}
```

- [ ] **Step 4: Implement sync and cached app handlers**

Add routes:

```go
s.mux.HandleFunc("POST /api/client/v1/sources/{id}/sync", s.handleSyncSource)
s.mux.HandleFunc("POST /api/client/v1/sources/sync", s.handleSyncAllSources)
s.mux.HandleFunc("GET /api/client/v1/apps", s.handleListApps)
s.mux.HandleFunc("GET /api/client/v1/apps/{id}", s.handleGetApp)
```

Implement `sync.go` with these exact rules:

- HTTP client timeout uses `s.cfg.SyncTimeout`, defaulting to 20 seconds if zero.
- Password is sent as query `password` and `X-Source-Password`.
- Replace all cached apps for that source inside an ent transaction.
- For GitHub URLs, mirror rewrite returns `strings.TrimRight(source.Mirror, "/") + "/" + githubURL`.
- Store `latest_version_json` as JSON encoded `VersionDTO`.

- [ ] **Step 5: Run task tests**

Run:

```bash
go test ./internal/clientserver -run 'TestSyncSourceCachesAppsAndUpdatesSource|TestSyncSourceAuthFailureIsStored'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/clientserver
git commit -m "feat: add client source sync"
```

---

### Task 4: LazyCat Go SDK Installed And Install APIs

**Files:**
- Create: `internal/clientserver/lazycat.go`
- Create: `internal/clientserver/install.go`
- Modify: `internal/clientserver/apps.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/server.go`
- Modify: `internal/clientserver/server_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: cached `SourceAppDTO` and `VersionDTO`.
- Produces: `type PackageManager interface { QueryInstalled(context.Context, string) ([]InstalledApplicationDTO, error); InstallLPK(context.Context, string, InstallRequestDTO) (InstallResultDTO, error) }`
- Produces real adapter using `gohelper.NewAPIGateway(ctx)`, `gw.PkgManager.QueryApplication(ctx, &sys.QueryApplicationRequest{})`, and `gw.PkgManager.InstallLPK(ctx, &sys.InstallLPKRequest{...})`.

- [ ] **Step 1: Write failing SDK handler tests with fake package manager**

Add tests:

```go
type fakePackageManager struct {
	installed []InstalledApplicationDTO
	install   InstallResultDTO
	req       InstallRequestDTO
}

func (f *fakePackageManager) QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error) {
	return f.installed, nil
}

func (f *fakePackageManager) InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error) {
	f.req = req
	return f.install, nil
}

func TestInstalledAppsUsesPackageManager(t *testing.T) {
	app := testServer(t)
	app.server.pkg = &fakePackageManager{installed: []InstalledApplicationDTO{{AppID: "notes", Title: "Notes", Version: "1.2.3", Status: "Installed"}}}
	rec := app.request("GET", "/api/client/v1/installed", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"appid":"notes"`) {
		t.Fatalf("installed = %d %s", rec.Code, rec.Body.String())
	}
}

func TestInstallUsesCachedAppVersion(t *testing.T) {
	app := testServer(t)
	pm := &fakePackageManager{install: InstallResultDTO{Mode: "lazycat-go-sdk", TaskID: "task-1"}}
	app.server.pkg = pm
	seedCachedApp(t, app.server.db, "alice")
	rec := app.request("POST", "/api/client/v1/install", `{"appId":1}`, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("install = %d %s", rec.Code, rec.Body.String())
	}
	if pm.req.DownloadURL == "" || pm.req.SHA256 == "" || pm.req.PackageID != "notes" {
		t.Fatalf("bad install request: %#v", pm.req)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/clientserver -run 'TestInstalledAppsUsesPackageManager|TestInstallUsesCachedAppVersion'
```

Expected: FAIL because package manager and routes do not exist.

- [ ] **Step 3: Add DTOs and interface**

Extend `types.go`:

```go
type InstalledApplicationDTO struct {
	AppID          string `json:"appid"`
	Title          string `json:"title,omitempty"`
	Version        string `json:"version,omitempty"`
	Status         string `json:"status,omitempty"`
	InstanceStatus string `json:"instanceStatus,omitempty"`
	Icon           string `json:"icon,omitempty"`
}

type InstallRequestDTO struct {
	AppID           int    `json:"appId"`
	InstallPassword string `json:"installPassword,omitempty"`
	Name            string `json:"name,omitempty"`
	PackageID       string `json:"pkgId,omitempty"`
	DownloadURL     string `json:"downloadUrl,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
}

type InstallResultDTO struct {
	Mode   string `json:"mode"`
	TaskID string `json:"taskId,omitempty"`
	Status string `json:"status,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type PackageManager interface {
	QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error)
	InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error)
}
```

- [ ] **Step 4: Implement real LazyCat adapter**

Create `lazycat.go`:

```go
package clientserver

import (
	"context"
	"time"

	gohelper "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
	"google.golang.org/grpc/metadata"
)

type lazyCatPackageManager struct{}

func newLazyCatPackageManager() PackageManager {
	return lazyCatPackageManager{}
}

func lazycatContext(ctx context.Context, userID string) context.Context {
	if userID == "" || userID == "local" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-hc-user-id", userID)
}

func (lazyCatPackageManager) QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error) {
	ctx, cancel := context.WithTimeout(lazycatContext(ctx, userID), 10*time.Second)
	defer cancel()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return nil, err
	}
	defer gw.Close()
	resp, err := gw.PkgManager.QueryApplication(ctx, &sys.QueryApplicationRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]InstalledApplicationDTO, 0, len(resp.GetInfoList()))
	for _, item := range resp.GetInfoList() {
		out = append(out, InstalledApplicationDTO{
			AppID: item.GetAppid(), Title: item.GetTitle(), Version: item.GetVersion(),
			Status: item.GetStatus().String(), InstanceStatus: item.GetInstanceStatus().String(), Icon: item.GetIcon(),
		})
	}
	return out, nil
}

func (lazyCatPackageManager) InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error) {
	ctx, cancel := context.WithTimeout(lazycatContext(ctx, userID), 60*time.Second)
	defer cancel()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return InstallResultDTO{}, err
	}
	defer gw.Close()
	wait := true
	in := &sys.InstallLPKRequest{LpkUrl: req.DownloadURL, WaitUnitDone: &wait}
	if req.SHA256 != "" {
		in.Sha256 = &req.SHA256
	}
	if req.PackageID != "" {
		in.PkgId = &req.PackageID
	}
	if req.Name != "" {
		in.TmpTitle = &req.Name
	}
	resp, err := gw.PkgManager.InstallLPK(ctx, in)
	if err != nil {
		return InstallResultDTO{}, err
	}
	result := InstallResultDTO{Mode: "lazycat-go-sdk"}
	if task := resp.GetTaskInfo(); task != nil {
		result.TaskID = task.GetTaskId()
		result.Status = task.GetStatus().String()
		result.Detail = task.GetDetail()
	}
	return result, nil
}
```

- [ ] **Step 5: Add install and installed routes**

Add `pkg PackageManager` to `Server`, initialize it with `newLazyCatPackageManager()` in `New`, and add routes:

```go
s.mux.HandleFunc("GET /api/client/v1/installed", s.handleInstalled)
s.mux.HandleFunc("POST /api/client/v1/install", s.handleInstall)
```

`handleInstall` must:

- Load cached app by id and source owner user id.
- Reject missing latest version with HTTP 422.
- Append `installPassword` to protected download URLs using the existing frontend behavior: `installPassword=<urlencoded password>`.
- Use slug as `PackageID` and app name as temporary title.

- [ ] **Step 6: Run task tests**

Run:

```bash
go test ./internal/clientserver -run 'TestInstalledAppsUsesPackageManager|TestInstallUsesCachedAppVersion'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/clientserver go.mod go.sum
git commit -m "feat: install apps through lazycat go sdk"
```

---

### Task 5: Store Client Binary And LazyCat Packaging

**Files:**
- Create: `cmd/store-client/main.go`
- Create: `clientembed/embed.go`
- Create: `clientembed/dist/README.md`
- Modify: `internal/clientserver/server.go`
- Modify: `lazycat/client/build.sh`
- Modify: `lazycat/client/lzc-manifest.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes: `clientserver.New`, `clientserver.LoadConfig`.
- Produces: `go run ./cmd/store-client`.
- Produces embedded frontend fallback for SPA routes.

- [ ] **Step 1: Write smoke test for health endpoint**

Add to `server_test.go`:

```go
func TestHealthz(t *testing.T) {
	app := testServer(t)
	rec := app.request("GET", "/healthz", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("healthz = %d %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Add frontend embed package**

Create `clientembed/embed.go`:

```go
package clientembed

import "embed"

//go:embed dist
var Dist embed.FS
```

Create `clientembed/dist/README.md`:

```markdown
This directory is populated by `lazycat/client/build.sh` before compiling the
standalone client binary. The Go `clientembed` package embeds the generated
Vite assets from here.
```

- [ ] **Step 3: Add static frontend route and health route**

In `internal/clientserver/server.go`, add:

```go
s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "lazycat-appstore-client"})
})
s.mux.Handle("/", embeddedClientHandler())
```

Implement `embeddedClientHandler()` using `fs.Sub(clientembed.Dist, "dist")`, `http.FileServer`, and an SPA fallback to `index.html`.

- [ ] **Step 4: Add binary entrypoint**

Create `cmd/store-client/main.go`:

```go
package main

import (
	"errors"
	"log"
	"net/http"

	"lazycat.community/appstore/internal/clientserver"
)

func main() {
	cfg := clientserver.LoadConfig()
	app, err := clientserver.New(cfg)
	if err != nil {
		log.Fatalf("start appstore client: %v", err)
	}
	defer app.Close()
	server := &http.Server{Addr: cfg.Addr, Handler: app.Handler()}
	log.Printf("LazyCat Community App Store client listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Update LazyCat client build and manifest**

Change `lazycat/client/build.sh` to:

- Build React with `VITE_API_BASE_URL=""`.
- Copy `client/dist/.` to `clientembed/dist/`.
- Write `app-config.js` with default source env values.
- Build `go build -trimpath -ldflags="-s -w" -o "$CONTENT_DIR/store-client" ./cmd/store-client`.
- Keep lazycat inject download.

Change `lazycat/client/lzc-manifest.yml` upstream to:

```yaml
application:
  subdomain: lazycat-appstore
  background_task: true
  public_path:
    - /
  upstreams:
    - location: /
      backend: http://127.0.0.1:8090/
      backend_launch_command: /lzcapp/pkg/content/store-client
  environment:
    - CLIENT_ADDR=127.0.0.1:8090
    - CLIENT_DB_DSN=/lzcapp/var/data/client.db
    - CLIENT_DEFAULT_SOURCE_URL={{ .U.default_source_url }}
    - CLIENT_DEFAULT_SOURCE_NAME={{ .U.default_source_name }}
```

If deploy params do not currently define `default_source_url` and `default_source_name`, use shell build-time app-config defaults instead of manifest template variables.

- [ ] **Step 6: Run smoke checks**

Run:

```bash
go test ./internal/clientserver -run TestHealthz
go build ./cmd/store-client
npx --yes js-yaml lazycat/client/lzc-manifest.yml
```

Expected: all commands pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/store-client clientembed internal/clientserver lazycat/client README.md
git commit -m "feat: package standalone go client"
```

---

### Task 6: Frontend API Migration

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/lazycatSdk.ts`
- Modify: `client/package.json`
- Modify: `client/package-lock.json`

**Interfaces:**
- Consumes: `/api/client/v1/sources`, `/sources/{id}/sync`, `/sources/sync`, `/apps`, `/installed`, `/install`.
- Produces: frontend no longer reads or writes `lazycat.sources` or `lazycat.sourceApps`.

- [ ] **Step 1: Add client API helper**

In `App.tsx`, add:

```ts
const CLIENT_API_BASE = '/api/client/v1';

async function clientApi<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${CLIENT_API_BASE}${path}`, {
    credentials: 'include',
    headers: options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data?.error?.message || `HTTP ${response.status}`);
  }
  return data as T;
}
```

- [ ] **Step 2: Replace source/app initialization**

Change:

```ts
const [sourceApps, setSourceApps] = useState<SourceApp[]>(() => {
  const saved = localStorage.getItem('lazycat.sourceApps');
  ...
});
const [sources, setSources] = useState<SourceSubscription[]>(() => {
  const saved = localStorage.getItem('lazycat.sources');
  ...
});
```

to:

```ts
const [sourceApps, setSourceApps] = useState<SourceApp[]>([]);
const [sources, setSources] = useState<SourceSubscription[]>([]);
```

Delete the two effects that call:

```ts
localStorage.setItem('lazycat.sources', JSON.stringify(sources));
localStorage.setItem('lazycat.sourceApps', JSON.stringify(sourceApps));
```

- [ ] **Step 3: Add client data loaders**

Add:

```ts
async function loadClientSources() {
  const data = await clientApi<{ sources: SourceSubscription[] }>('/sources');
  setSources(data.sources);
}

async function loadClientApps() {
  const data = await clientApi<{ apps: SourceApp[] }>('/apps');
  setSourceApps(data.apps);
}

async function refreshClientData() {
  await Promise.all([loadClientSources(), loadClientApps()]);
}
```

In the initial `useEffect`, call `refreshAll()` for `HAS_API` server mode and `refreshClientData()` for standalone mode.

- [ ] **Step 4: Replace source CRUD and sync calls**

In source add/update/delete handlers, replace local array mutation as the durable action with `clientApi` calls:

```ts
await clientApi('/sources', { method: 'POST', body: JSON.stringify({ name, url, password: draft.password, mirror: draft.mirror }) });
await refreshClientData();
```

For sync:

```ts
await clientApi(`/sources/${source.id}/sync`, { method: 'POST' });
await refreshClientData();
```

For sync all:

```ts
await clientApi('/sources/sync', { method: 'POST' });
await refreshClientData();
```

- [ ] **Step 5: Replace installed and install calls**

Replace `queryInstalledApplications()` with:

```ts
const result = await clientApi<{ apps: InstalledApplication[] }>('/installed');
setInstalledApps(result.apps || []);
```

Replace `installWithLazyCat(...)` with:

```ts
const result = await clientApi<{ mode: string; taskId?: string; status?: string; detail?: string }>('/install', {
  method: 'POST',
  body: JSON.stringify({
    appId: app.id,
    installPassword: options.installPassword,
  }),
});
```

Map `result.mode === 'lazycat-go-sdk'` to the existing success UI.

- [ ] **Step 6: Remove JS SDK dependency if unused**

If `client/src/lazycatSdk.ts` is no longer imported, delete it and run:

```bash
cd client
npm uninstall @lazycatcloud/sdk
```

- [ ] **Step 7: Verify no durable localStorage remains**

Run:

```bash
rg -n "lazycat\\.sources|lazycat\\.sourceApps|queryInstalledApplications|installWithLazyCat|@lazycatcloud/sdk" client/src client/package.json
cd client && npm run build
```

Expected: `rg` returns no matches for removed durable storage and SDK calls; build passes.

- [ ] **Step 8: Commit**

```bash
git add client
git commit -m "feat: load standalone client data from api"
```

---

### Task 7: UI Polish And Full Verification

**Files:**
- Modify: `client/src/styles.css`
- Modify: `client/src/App.tsx`
- Modify: `README.md`

**Interfaces:**
- Consumes: API-backed frontend state from Task 6.
- Produces: polished shared UI for standalone client and server-backed mode.

- [ ] **Step 1: Add motion and interaction tokens**

In `styles.css`, add root variables:

```css
--ease-out: cubic-bezier(0.23, 1, 0.32, 1);
--ease-in-out: cubic-bezier(0.77, 0, 0.175, 1);
--shadow-soft: 0 18px 48px color-mix(in srgb, var(--ink) 12%, transparent);
```

- [ ] **Step 2: Add precise interactive transitions**

Update button/card selectors:

```css
.icon-button,
.user-pill,
.primary-button,
.secondary-button,
.install-button,
.nav-item,
.app-card,
.source-app-card,
.source-row,
.review-row,
.version-row {
  transition:
    transform 140ms var(--ease-out),
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease,
    box-shadow 160ms ease;
}

.icon-button:active,
.user-pill:active,
.primary-button:active,
.secondary-button:active,
.install-button:active,
.nav-item:active,
.app-card:active,
.source-app-card:active {
  transform: scale(0.97);
}

@media (hover: hover) and (pointer: fine) {
  .app-card:hover,
  .source-app-card:hover,
  .source-row:hover,
  .review-row:hover,
  .version-row:hover {
    border-color: color-mix(in srgb, var(--green) 45%, var(--line));
    box-shadow: var(--shadow-soft);
  }
}
```

- [ ] **Step 3: Add skeleton loading states**

Add CSS:

```css
.skeleton-list {
  display: grid;
  gap: 12px;
}

.skeleton-line {
  min-height: 74px;
  border-radius: 8px;
  background: linear-gradient(90deg, var(--field), color-mix(in srgb, var(--field) 78%, var(--line)), var(--field));
  background-size: 220% 100%;
  animation: skeleton-shimmer 1.1s linear infinite;
}

@keyframes skeleton-shimmer {
  from { background-position: 120% 0; }
  to { background-position: -120% 0; }
}
```

Under `prefers-reduced-motion`, disable the shimmer animation.

In `App.tsx`, replace the top-level generic loading block with three `.skeleton-line` rows.

- [ ] **Step 4: Add drawer/toast/install panel entry polish**

Use `@starting-style` where supported:

```css
.drawer {
  transition: transform 220ms var(--ease-drawer, var(--ease-out)), opacity 180ms var(--ease-out);
  @starting-style {
    opacity: 0;
    transform: translateX(24px);
  }
}

.toast,
.install-panel,
.install-password-dialog {
  transition: transform 180ms var(--ease-out), opacity 180ms var(--ease-out);
  @starting-style {
    opacity: 0;
    transform: translateY(12px);
  }
}
```

- [ ] **Step 5: Verify build and tests**

Run:

```bash
go test ./...
(cd client && npm run build)
npx --yes js-yaml lazycat/client/package.yml
npx --yes js-yaml lazycat/client/lzc-manifest.yml
npx --yes js-yaml lazycat/client/lzc-build.yml
go build ./cmd/store-client
```

Expected: all commands pass.

- [ ] **Step 6: Commit**

```bash
git add client README.md
git commit -m "style: polish appstore client ui"
```

---

## Self-Review

- Spec coverage: Tasks 1-4 cover ent/SQLite, user scoping, source sync, cached apps, installed apps, and Go SDK install. Task 5 covers Go binary and LazyCat packaging. Task 6 covers frontend data migration away from durable `localStorage`. Task 7 covers Emil-style UI polish and verification.
- Placeholder scan: The plan avoids unresolved markers and unspecified task steps. Each task has concrete files, interfaces, commands, and expected results.
- Type consistency: API DTO names are introduced before use. The LazyCat adapter uses confirmed SDK symbols: `gohelper.NewAPIGateway`, `gw.PkgManager.QueryApplication`, `sys.QueryApplicationRequest`, `gw.PkgManager.InstallLPK`, and `sys.InstallLPKRequest`.
