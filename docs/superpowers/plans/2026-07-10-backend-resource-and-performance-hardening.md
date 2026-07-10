# Backend Resource and Performance Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bound untrusted input and remote responses, keep authentication and database resources finite, stream migration and backup archives through temporary files, and reduce app-list query cost only when repeatable benchmarks justify it.

**Architecture:** Build on the runtime-hardening plan after its lifecycle changes are green. Keep ordinary JSON and HTTP proxy traffic on bounded helpers, expose the configured `database/sql` pool alongside Ent, change migration imports to `io.ReaderAt`, and make every large archive operation file-backed. Performance work starts with query-count and allocation measurements and retains the one-query download aggregation only when it meets the stated acceptance threshold.

**Tech Stack:** Go 1.26.4, `net/http`, `database/sql`, Ent, `archive/zip`, `os.CreateTemp`, SQLite/PostgreSQL/MySQL, Go benchmarks and `benchstat`.

## Global Constraints

- Execute this plan only after `2026-07-10-backend-runtime-hardening.md` passes its focused race checkpoint.
- Preserve all existing routes, public migration format version, JSON field names, and software-source v1/v2 semantics.
- Do not modify generated Ent files manually and do not generate Ent code unless a schema file changes.
- Do not modify frontend source, locale files, or generated frontend distributions.
- Keep migration upload at 512 MiB compressed, 2 GiB total expanded data, 64 MiB per JSON or attachment entry, and 4096 ZIP entries.
- Keep software-source URLs compatible with HTTP, HTTPS, loopback, and private-network deployments.
- Use Go features up to and including 1.26.4; benchmarks use `b.Loop()` and tests use `t.Context()`.
- Do not add `sync.Pool`, a global result cache, a distributed lock, Redis, or a new runtime dependency.
- Preserve all pre-existing uncommitted work in owned files and stage only files listed by the current task.
- Before editing, capture `git status --short -- <owned files>` and `git diff -- <owned files>` in the execution transcript. For each pre-existing untracked owned file, also capture `git diff --no-index /dev/null <path> || true`. Use `git add -p` for every already-modified or pre-existing untracked file (run `git add -N <path>` first for untracked files); never stage pre-existing AppDownload/AppVote, migration-record, backup, API, or generated Ent WIP. Files first created by the current task may use ordinary `git add`.
- Before every commit run `git diff --cached --check` and inspect `git diff --cached --unified=0`.

---

## File Map

- Modify `internal/server/respond.go` and complete pre-existing untracked `respond_test.go` for bounded, single-value JSON.
- Modify `internal/clientserver/respond.go` and JSON handlers for the same request contract.
- Complete pre-existing untracked `internal/clientserver/http_clients.go` and create tests for bounded and streaming clients.
- Complete pre-existing untracked `internal/clientserver/source_policy.go`, create tests, and complete pre-existing untracked `docs/security/software-source-trust.md`.
- Modify `internal/server/server.go` and `handlers_auth.go` for bounded login-failure state.
- Create `internal/dbpool/open.go` and tests; wire both applications through it.
- Modify `internal/migration/importer.go`, `zip.go`, `files.go`, and `exporter.go` for file-backed archives.
- Create `internal/server/migration_files.go`; modify migration handlers and backup.
- Modify `internal/server/app_metrics.go` and benchmarks for evidence-based query consolidation.

### Task 1: Bound Local JSON Requests and Reject Trailing Values

**Files:**
- Modify: `internal/server/respond.go`
- Modify (pre-existing untracked): `internal/server/respond_test.go`
- Modify: `internal/clientserver/respond.go`
- Modify: `internal/clientserver/install.go`
- Modify: `internal/clientserver/comments.go`
- Modify: `internal/clientserver/settings.go`
- Modify: `internal/clientserver/sources.go`
- Modify: `internal/clientserver/server_test.go`

**Interfaces:**
- Preserves server: `func decodeJSON(r *http.Request, out any) error`.
- Produces client: `func decodeJSON(r *http.Request, out any) error`.
- Enforces a 1 MiB maximum, unknown-field rejection, and exactly one JSON value.

- [x] **Step 1: Add failing decoder tests**

Add server and client tests for a valid object, an unknown field, `{"name":"one"}{"name":"two"}`, and an object larger than 1 MiB. The trailing case must contain `single JSON value`; the oversized case must satisfy `errors.AsType[*http.MaxBytesError](err)`.

```go
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
        {name: "oversized", body: `{"name":"` + strings.Repeat("x", maxJSONBodyBytes) + `"}`, tooBig: true},
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
```

- [x] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/server ./internal/clientserver -run 'DecodeJSON|TrailingJSON|OversizedJSON' -count=1`

Expected: FAIL because the server accepts a second value and client handlers use direct decoders.

- [x] **Step 3: Implement the bounded decoder in both packages**

```go
const maxJSONBodyBytes int64 = 1 << 20

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
```

- [x] **Step 4: Replace direct client decoders**

In `install.go`, `comments.go`, `settings.go`, and `sources.go` replace `json.NewDecoder(r.Body).Decode(&input)` with `decodeJSON(r, &input)`. Preserve current status codes and user-facing messages.

- [x] **Step 5: Run tests and commit**

Run: `go test ./internal/server ./internal/clientserver -run 'DecodeJSON|TrailingJSON|OversizedJSON' -count=1`

Expected: PASS.

`internal/server/respond_test.go` is pre-existing untracked WIP in this worktree, so mark it intent-to-add and interactively stage it with the existing files:

```bash
git add -N internal/server/respond_test.go
git add -p internal/server/respond_test.go internal/server/respond.go internal/clientserver/respond.go internal/clientserver/install.go internal/clientserver/comments.go internal/clientserver/settings.go internal/clientserver/sources.go internal/clientserver/server_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: bound JSON request bodies"
```

### Task 2: Use Separate Bounded and Streaming HTTP Clients

**Files:**
- Modify (pre-existing untracked): `internal/clientserver/http_clients.go`
- Create: `internal/clientserver/http_clients_test.go`
- Modify (pre-existing untracked): `internal/clientserver/source_policy.go`
- Create: `internal/clientserver/source_policy_test.go`
- Modify (pre-existing untracked): `docs/security/software-source-trust.md`
- Modify: `internal/clientserver/server.go`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/comments.go`
- Modify: `internal/clientserver/chat.go`
- Modify: `internal/clientserver/assets.go`
- Modify: `internal/clientserver/sources.go`
- Modify: `internal/clientserver/user.go`

**Interfaces:**
- Produces: `func newHTTPClients() (*http.Client, *http.Client)`.
- Produces: `func decodeLimitedJSON(r io.Reader, maxBytes int64, out any) error`.
- Produces: `func readLimitedBody(r io.Reader, maxBytes int64) ([]byte, error)` and `responseTooLargeError`.
- Produces: `func writeBoundedSourceResponse(w http.ResponseWriter, resp *http.Response, maxBytes int64) error`; this function reads and validates the entire bounded response before copying headers or calling `WriteHeader`.
- Produces: `func noRedirectClient(base *http.Client) *http.Client` and changes `fetchSourceIcon` to accept that client.
- Produces the extension seam `sourceURLPolicy.Validate(context.Context, clientIdentity, *url.URL) error`; the initial `allowSourceURLPolicy` permits HTTP/HTTPS, loopback, and private-network targets.
- Adds `httpClient *http.Client` and `streamClient *http.Client` to `clientserver.Server`.
- Adds `sourcePolicy sourceURLPolicy` to `clientserver.Server`.
- Uses a 64 MiB feed limit, a 1 MiB non-stream proxy-response limit, and a 30-second total timeout only for non-stream traffic.

- [x] **Step 1: Add failing construction, redirect, and size-limit tests**

In `http_clients_test.go`, assert:

- the ordinary client has a 30-second timeout;
- the stream client has no total timeout;
- both transports have 5-second dial/TLS, 10-second response-header, 1-second expect-continue, 90-second idle-connection, 100 global-idle, and 10 per-host-idle bounds;
- `decodeLimitedJSON` rejects a source feed larger than 64 MiB and rejects a second JSON value;
- `readLimitedBody` returns `*responseTooLargeError` for `maxBytes+1` bytes instead of returning a truncated body;
- `writeBoundedSourceResponse` leaves an `httptest.ResponseRecorder` at status 200 with no copied source headers and an empty body when the response is oversized, proving callers can still emit `SOURCE_RESPONSE_TOO_LARGE`;
- an icon endpoint that redirects to a second endpoint is not followed, while the ordinary source-feed client still follows redirects according to `net/http` defaults.

- [x] **Step 2: Implement the client factory**

```go
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
```

Initialize both in `clientserver.New`. Directly constructed test servers call the following package-private fallback before first use:

```go
func (s *Server) ensureHTTPClients() {
    if s.httpClient != nil && s.streamClient != nil {
        return
    }
    ordinary, stream := newHTTPClients()
    if s.httpClient == nil {
        s.httpClient = ordinary
    }
    if s.streamClient == nil {
        s.streamClient = stream
    }
}
```

- [x] **Step 3: Implement strict remote JSON and raw-body limits**

```go
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
```

- [x] **Step 4: Replace default clients and reject oversized requests and responses**

Call `s.ensureHTTPClients()` before each outbound path. Use `s.httpClient` for feed sync, comments, non-stream chat, and icons. Use `s.streamClient` only for `/api/v1/chat/events`. Decode source feeds with `decodeLimitedJSON(resp.Body, maxSourceFeedBytes, &root)`.

For the two chat request bodies, replace `io.ReadAll(io.LimitReader(r.Body, 1<<20))` with `readLimitedBody(r.Body, 1<<20)`. For the outdated-mark body, replace `io.ReadAll(io.LimitReader(r.Body, 4096))` with `readLimitedBody(r.Body, 4096)`. Map `*responseTooLargeError` to HTTP 413 with code `REQUEST_TOO_LARGE`, and map other read errors to the existing `INVALID_BODY` response. Do not forward a truncated request.

For `proxySourceChatRequest` and `proxySourceCommentRequest`, call `writeBoundedSourceResponse` before copying any source headers. Handle errors exactly as follows:

```go
err = writeBoundedSourceResponse(w, resp, maxSourceProxyResponseBytes)
if _, ok := errors.AsType[*responseTooLargeError](err); ok {
    writeError(w, http.StatusBadGateway, "SOURCE_RESPONSE_TOO_LARGE", "Source response is too large")
    return
}
if err != nil {
    return
}
```

Because `writeBoundedSourceResponse` does not mutate `w` until its bounded read succeeds, the oversized branch must be able to write the JSON error and must not preserve the source status or headers. The SSE path remains streaming and is not routed through this helper.

Change the icon function and its call site to preserve the existing no-redirect trust boundary while reusing the bounded transport:

```go
func fetchSourceIcon(ctx context.Context, client *http.Client, iconURL, sourcePassword string, maxBytes int64) (assetdata.Payload, error)

payload, err := fetchSourceIcon(
    iconCtx,
    noRedirectClient(s.httpClient),
    icon.String(),
    sourcePassword,
    clientAssetMaxImageSize,
)
```

- [x] **Step 5: Add the private-source trust policy seam and documentation**

Create `source_policy.go` with these exact initial interfaces:

```go
type sourceURLPolicy interface {
    Validate(context.Context, clientIdentity, *url.URL) error
}

type allowSourceURLPolicy struct{}

func (allowSourceURLPolicy) Validate(_ context.Context, _ clientIdentity, target *url.URL) error {
    if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
        return errors.New("source URL must use HTTP or HTTPS")
    }
    return nil
}
```

Initialize `sourcePolicy: allowSourceURLPolicy{}` in `New` and `newTestServer`; add `currentClientIdentity(r) (clientIdentity, bool)` in `user.go` to read the already-authenticated context value. After `readSourceInput` normalizes the URL and before create/update persistence, parse it, call `s.sourcePolicy.Validate(r.Context(), identity, parsed)`, and return HTTP 422 `SOURCE_URL_NOT_ALLOWED` on rejection. Tests inject a rejecting policy and prove create/update do not persist; separate tests prove the default permits `http://127.0.0.1`, `http://[::1]`, RFC1918 addresses, and ordinary HTTPS hosts.

Create `docs/security/software-source-trust.md` with all of the following statements in concrete prose:

- a user-added software source is fetched with the client service's network reach and can therefore contact loopback/private services reachable by that process;
- loopback and private-network sources are intentionally supported for current single-user/LazyCat deployments;
- the current policy is `allowSourceURLPolicy`, so this task does not claim administrator-only enforcement;
- a future multi-user/OIDC deployment can provide an `admin-only` implementation through `sourceURLPolicy` once identity roles exist;
- no configuration flag is exposed until a real role/authorization model can enforce it.

- [x] **Step 6: Verify streaming independence and pre-header rejection**

Use a test client with a 20 ms ordinary timeout. Make an SSE test server emit after 50 ms and assert the stream still succeeds through `streamClient`. Add proxy tests where a source returns `maxSourceProxyResponseBytes+1` bytes with status 201 and header `X-Upstream: copied`; assert the client response is HTTP 502 with code `SOURCE_RESPONSE_TOO_LARGE`, does not contain `X-Upstream`, and does not expose status 201. Add exact-limit success tests for both chat and comments.

Run: `go test -race ./internal/clientserver -run 'HTTPClient|SourceFeed|SSE|ResponseTooLarge|SourceURLPolicy|SourceIconRedirect' -count=1`

Expected: PASS and the following scoped scan returns no output:

```bash
rg -n 'http\.DefaultClient' internal/clientserver --glob '*.go'
```

- [x] **Step 7: Commit**

```bash
git add internal/clientserver/http_clients_test.go internal/clientserver/source_policy_test.go
git add -N internal/clientserver/http_clients.go internal/clientserver/source_policy.go docs/security/software-source-trust.md
git add -p internal/clientserver/http_clients.go internal/clientserver/source_policy.go docs/security/software-source-trust.md internal/clientserver/server.go internal/clientserver/sync.go internal/clientserver/comments.go internal/clientserver/chat.go internal/clientserver/assets.go internal/clientserver/sources.go internal/clientserver/user.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: bound client HTTP resources"
```

### Task 3: Bound and Throttle Administrator Login Failures

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/handlers_auth.go`
- Create: `internal/server/login_throttle_test.go`

**Interfaces:**
- Produces `adminLoginFailure{Attempts, ExpiresAt, BlockedUntil}`.
- Changes `recordAdminLoginFailure` to return `adminLoginFailure` and adds `adminLoginFailureForRequest` plus `writeAdminLoginRateLimit`.
- Keeps at most 4096 keys for 15 minutes.
- Requires CAPTCHA after 3 failures and returns HTTP 429 for 30 seconds after 6 failures.

- [x] **Step 1: Add failing tests**

Inject `authNow func() time.Time`. Prove the third failure requires CAPTCHA, the sixth blocks, `Retry-After` is returned, success clears state, entries expire after 15 minutes, and 5000 unique keys leave at most 4096 entries.

- [x] **Step 2: Replace the unbounded map**

```go
const (
    adminLoginFailureTTL     = 15 * time.Minute
    adminLoginBlockDuration  = 30 * time.Second
    adminLoginBlockThreshold = 6
    maxAdminLoginFailureKeys = 4096
)

type adminLoginFailure struct {
    Attempts     int
    ExpiresAt    time.Time
    BlockedUntil time.Time
}
```

Add `authNow func() time.Time` to `Server` and use this fallback:

```go
func (s *Server) adminLoginNow() time.Time {
    if s.authNow != nil {
        return s.authNow().UTC()
    }
    return time.Now().UTC()
}
```

Change the map to `map[string]adminLoginFailure`. While holding its mutex, remove every entry whose `ExpiresAt` is not after `now`. Before inserting a new key at capacity, find and delete the entry with the earliest `ExpiresAt`; ties are resolved by lexicographically smaller key so the test is deterministic. Refresh `ExpiresAt = now.Add(adminLoginFailureTTL)` on every failure.

- [x] **Step 3: Add pre-check and recording**

After the user record is loaded and confirmed as an administrator, but before checking its password, read the keyed state under the mutex. When `BlockedUntil` is after `now`, return status 429/code `LOGIN_RATE_LIMITED` and set `Retry-After` to the ceiling of the remaining seconds, with a minimum of one:

```go
func writeAdminLoginRateLimit(w http.ResponseWriter, remaining time.Duration, state adminLoginFailure) {
    retryAfter := max(1, int(math.Ceil(remaining.Seconds())))
    w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
    writeError(w, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "Too many administrator login attempts", loginFailureDetails{
        FailedAttempts:  state.Attempts,
        CaptchaRequired: true,
    })
}
```

On password failure, increment attempts and set `BlockedUntil = now.Add(adminLoginBlockDuration)` when the count reaches six. The sixth failure itself returns the same HTTP 429 response; failures three through five remain HTTP 401 with `CaptchaRequired: true`. Successful administrator login deletes the key. Disabled, unverified, and invalid-TOTP responses do not clear password-failure state; only a fully successful login clears it.

- [x] **Step 4: Run race tests and commit**

Run: `go test -race ./internal/server -run 'AdminLogin(RequiresCaptcha|Failures|Failure|Handler)' -count=20`

Expected: PASS with exact concurrent counts and bounded map size.

```bash
git add internal/server/login_throttle_test.go
git add -p internal/server/server.go internal/server/handlers_auth.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: bound admin login throttling"
```

### Task 4: Configure and Verify Database Connection Pools

**Files:**
- Create: `internal/dbpool/open.go`
- Create: `internal/dbpool/open_test.go`
- Create: `internal/dbpool/sqlite_benchmark_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/clientserver/config.go`
- Modify: `internal/clientserver/config_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/clientserver/db.go`
- Modify: `internal/clientserver/server.go`

**Interfaces:**
- Produces `dbpool.Config{Driver, DSN, MaxOpen, MaxIdle, MaxLifetime, MaxIdleTime}`.
- Produces `func dbpool.Open(cfg Config) (*sql.DB, dialect.Driver, error)`.
- Changes server `openEnt(config.Config)` to `(*ent.Client, *sql.DB, error)` and client `openDB(Config)` to the same return shape.
- Defaults PostgreSQL/MySQL to 20 open/10 idle. SQLite begins at the safe 1 open/1 idle candidate and changes to 4/4 only if the benchmark gate in Step 6 passes. All drivers default to a 30-minute lifetime and 5-minute idle time.
- Preserves both existing SQLite DSN normalizers, WAL, busy timeout, directory creation, and schema creation.

- [x] **Step 1: Add failing configuration and pool tests**

Add these exact fields to both configuration test expectations:

```go
DBMaxOpenConns     int
DBMaxIdleConns     int
DBConnMaxLifetime  time.Duration
DBConnMaxIdleTime  time.Duration
```

Cover server environment variables `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, and `DB_CONN_MAX_IDLE_TIME`, plus client variables with the `CLIENT_` prefix. Assert invalid integers/durations fall back to the driver default, `Validate` rejects `MaxOpen <= 0`, `MaxIdle < 0`, `MaxIdle > MaxOpen`, or negative lifetimes, and zero lifetime/idle time remains a valid explicit database/sql disable value.

In `open_test.go`, use SQLite and assert all of the following:

```go
if got := db.Stats().MaxOpenConnections; got != cfg.MaxOpen {
    t.Fatalf("MaxOpenConnections = %d, want %d", got, cfg.MaxOpen)
}
if err := ent.NewClient(ent.Driver(driver)).Schema.Create(t.Context()); err != nil {
    t.Fatalf("create schema: %v", err)
}
```

Both dbpool test files import `_ "github.com/lib-x/entsqlite"` so `sql.Open("sqlite3", ...)` is registered when the package is tested in isolation. Also prove unsupported drivers and failed pings close the raw pool and return an error.

- [x] **Step 2: Implement exact parsing and pool opening**

Refactor `config.Load` to compute the normalized server driver before the struct literal so defaults are driver-aware. Add the shared helpers below to the server and client config packages (the client always passes `sqlite3`):

```go
func defaultDBPool(driver string) (maxOpen, maxIdle int) {
    if driver == "sqlite3" {
        return 1, 1
    }
    return 20, 10
}

func envDuration(key string, fallback time.Duration) time.Duration {
    value := strings.TrimSpace(os.Getenv(key))
    if value == "" {
        return fallback
    }
    parsed, err := time.ParseDuration(value)
    if err != nil {
        return fallback
    }
    return parsed
}
```

Create `internal/dbpool/open.go` with:

```go
type Config struct {
    Driver      string
    DSN         string
    MaxOpen     int
    MaxIdle     int
    MaxLifetime time.Duration
    MaxIdleTime time.Duration
}

func Open(cfg Config) (*sql.DB, dialect.Driver, error) {
    entDialect, err := entDialectName(cfg.Driver)
    if err != nil {
        return nil, nil, err
    }
    db, err := sql.Open(cfg.Driver, cfg.DSN)
    if err != nil {
        return nil, nil, err
    }
    db.SetMaxOpenConns(cfg.MaxOpen)
    db.SetMaxIdleConns(min(cfg.MaxIdle, cfg.MaxOpen))
    db.SetConnMaxLifetime(cfg.MaxLifetime)
    db.SetConnMaxIdleTime(cfg.MaxIdleTime)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        _ = db.Close()
        return nil, nil, err
    }
    return db, entsql.OpenDB(entDialect, db), nil
}
```

`entDialectName` maps `sqlite3` to `dialect.SQLite`, `postgres` to `dialect.Postgres`, and `mysql` to `dialect.MySQL`. Validate `MaxOpen > 0`, `0 <= MaxIdle <= MaxOpen`, and non-negative durations before `sql.Open`.

- [x] **Step 3: Wire the server without changing its SQLite semantics**

Keep `ensureSQLiteDir(cfg)` and `sqliteDSN(cfg.DBDSN)` in `internal/server/server.go`. Pass the normalized DSN and config limits to `dbpool.Open`, construct Ent with `ent.NewClient(ent.Driver(driver))`, and return both values:

```go
func openEnt(cfg config.Config) (*ent.Client, *sql.DB, error) {
    dsn := cfg.DBDSN
    if cfg.DBDriver == "sqlite3" {
        dsn = sqliteDSN(dsn)
    }
    sqlDB, driver, err := dbpool.Open(dbpool.Config{
        Driver: cfg.DBDriver, DSN: dsn,
        MaxOpen: cfg.DBMaxOpenConns, MaxIdle: cfg.DBMaxIdleConns,
        MaxLifetime: cfg.DBConnMaxLifetime, MaxIdleTime: cfg.DBConnMaxIdleTime,
    })
    if err != nil {
        return nil, nil, err
    }
    return ent.NewClient(ent.Driver(driver)), sqlDB, nil
}
```

Add `sqlDB *sql.DB` to `server.Server`, set it in `New`, and continue running `client.Schema.Create` exactly where it runs now. Every error path closes the Ent client. Do not close `sqlDB` separately because `entsql.OpenDB` owns the same pool.

- [x] **Step 4: Wire the client without changing its SQLite semantics**

Keep `ensureSQLiteDir(cfg.DBDSN)` and `sqliteDSN(cfg.DBDSN)` in `internal/clientserver/db.go`, keep `migrateSchema` in `clientserver.New`, and use:

```go
func openDB(cfg Config) (*ent.Client, *sql.DB, error) {
    if err := ensureSQLiteDir(cfg.DBDSN); err != nil {
        return nil, nil, err
    }
    sqlDB, driver, err := dbpool.Open(dbpool.Config{
        Driver: "sqlite3", DSN: sqliteDSN(cfg.DBDSN),
        MaxOpen: cfg.DBMaxOpenConns, MaxIdle: cfg.DBMaxIdleConns,
        MaxLifetime: cfg.DBConnMaxLifetime, MaxIdleTime: cfg.DBConnMaxIdleTime,
    })
    if err != nil {
        return nil, nil, err
    }
    return ent.NewClient(ent.Driver(driver)), sqlDB, nil
}
```

Add `sqlDB *sql.DB` to `clientserver.Server`, store it in `New`, and preserve every current Ent close path.

- [x] **Step 5: Add a deterministic SQLite pressure test and real 1-vs-4 benchmark**

The pressure test uses 16 goroutines and `sync.WaitGroup.Go`, each executing 50 short transactions against a WAL database under a 10-second context. Assert the final count is 800 and collect every error through a buffered channel; any `database is locked` error fails the test instead of being silently retried forever.

Create `sqlite_benchmark_test.go` with an actual contention benchmark, not a command that refers to a nonexistent benchmark. Run one stable benchmark name with an environment-selected candidate so `benchstat` can compare two files:

```go
func BenchmarkSQLitePoolContention(b *testing.B) {
    maxOpen, err := strconv.Atoi(os.Getenv("APPSTORE_BENCH_SQLITE_MAX_OPEN"))
    if err != nil || (maxOpen != 1 && maxOpen != 4) {
        b.Fatal("APPSTORE_BENCH_SQLITE_MAX_OPEN must be 1 or 4")
    }
    db := openBenchmarkSQLite(b, maxOpen)
    var lockErrors atomic.Int64
    b.ReportAllocs()
    b.ResetTimer()
    for b.Loop() {
        var wg sync.WaitGroup
        for range 16 {
            wg.Go(func() {
                ctx, cancel := context.WithTimeout(b.Context(), 2*time.Second)
                defer cancel()
                if err := incrementSQLiteCounter(ctx, db); err != nil {
                    if strings.Contains(strings.ToLower(err.Error()), "locked") {
                        lockErrors.Add(1)
                    }
                    b.Error(err)
                }
            })
        }
        wg.Wait()
    }
    b.ReportMetric(float64(lockErrors.Load()), "lock-errors")
}
```

`openBenchmarkSQLite` must use a fresh WAL file for each benchmark process with `_pragma=busy_timeout(10000)`, create `counters(id INTEGER PRIMARY KEY, value INTEGER NOT NULL)`, and set max open/idle to the parameter. `incrementSQLiteCounter` begins a transaction, runs `UPDATE counters SET value = value + 1 WHERE id = 1`, and commits; rollback is deferred.

- [x] **Step 6: Run the 1-vs-4 evidence gate and select the SQLite default**

Ensure `benchstat` exists before capturing results:

```bash
command -v benchstat >/dev/null || go install golang.org/x/perf/cmd/benchstat@latest
APPSTORE_BENCH_SQLITE_MAX_OPEN=1 go test ./internal/dbpool -run '^$' -bench '^BenchmarkSQLitePoolContention$' -benchmem -count=10 > /tmp/appstore-sqlite-pool-1.txt
APPSTORE_BENCH_SQLITE_MAX_OPEN=4 go test ./internal/dbpool -run '^$' -bench '^BenchmarkSQLitePoolContention$' -benchmem -count=10 > /tmp/appstore-sqlite-pool-4.txt
benchstat /tmp/appstore-sqlite-pool-1.txt /tmp/appstore-sqlite-pool-4.txt
```

Keep SQLite at 1 open/1 idle unless the 4-connection case has zero lock errors in every run, improves `sec/op` by at least 5% with `p < 0.05`, and does not regress `B/op` or `allocs/op` by more than 5%. If and only if all three checks pass, change both SQLite defaults to 4/4 and update the config tests before committing. PostgreSQL/MySQL remain 20/10 regardless of this result. Record the chosen default and the exact `benchstat` comparison in the commit body.

- [x] **Step 7: Run and commit**

Run: `go test -race ./internal/dbpool ./internal/config ./internal/server ./internal/clientserver -run 'Pool|SQLiteConcurrent|Config' -count=5`

Expected: PASS.

```bash
git add internal/dbpool/open.go internal/dbpool/open_test.go internal/dbpool/sqlite_benchmark_test.go
git add -p internal/config/config.go internal/config/config_test.go internal/clientserver/config.go internal/clientserver/config_test.go internal/server/server.go internal/clientserver/db.go internal/clientserver/server.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "feat: configure database connection pools" -m "SQLite default selected from the recorded 1-vs-4 benchstat gate."
```

### Task 5: Change Migration Imports to File-Backed ReaderAt

**Files:**
- Modify: `internal/migration/importer.go`
- Modify: `internal/migration/zip.go`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**
- Changes the complete public and private call chain to `io.ReaderAt` plus size:

```go
func PreviewPackage(ctx context.Context, r io.ReaderAt, size int64) (*Preview, error)
func readManifestFromReaderAt(r io.ReaderAt, size int64) (Manifest, error)
func zipReaderFromReaderAt(r io.ReaderAt, size int64) (*zip.Reader, error)
func (i *Importer) Preview(ctx context.Context, r io.ReaderAt, size int64) (*Preview, error)
func (i *Importer) Import(ctx context.Context, r io.ReaderAt, size int64, options ImportOptions) (*ImportResult, error)
func readImportPackage(r io.ReaderAt, size int64) (*zip.Reader, Manifest, SiteData, PeopleData, AppsData, error)
```

- All buffer-backed callers pass `*bytes.Reader`; file-backed callers introduced by Task 7 pass `*os.File`.

- [x] **Step 1: Add a failing ReaderAt spy test**

Create a valid archive in a temporary file, wrap it with the following spy, and assert both `PreviewPackage` and `Importer.Preview` call `ReadAt`; the spy fails if any single request is larger than 128 KiB:

```go
type readerAtSpy struct {
    next      io.ReaderAt
    calls     atomic.Int64
    maxReadAt atomic.Int64
}

func (s *readerAtSpy) ReadAt(p []byte, off int64) (int, error) {
    if len(p) > 128<<10 {
        return 0, fmt.Errorf("ReadAt requested %d bytes", len(p))
    }
    s.calls.Add(1)
    s.maxReadAt.CompareAndSwap(0, int64(len(p)))
    return s.next.ReadAt(p, off)
}
```

Also add nil, negative-size, and `maxCompressedBytes+1` tests. Existing test archives must keep a named `reader := bytes.NewReader(buf.Bytes())`; do not pass a one-shot `io.Reader` wrapper that lacks `ReadAt`.

- [x] **Step 2: Replace the copying ZIP constructor and every caller**

```go
func zipReaderFromReaderAt(r io.ReaderAt, size int64) (*zip.Reader, error) {
    if r == nil {
        return nil, fmt.Errorf("migration package reader is required")
    }
    if size < 0 || size > maxCompressedBytes {
        return nil, fmt.Errorf("migration package is too large")
    }
    zr, err := zip.NewReader(r, size)
    if err != nil {
        return nil, fmt.Errorf("open migration package: %w", err)
    }
    return zr, nil
}
```

Rename `readManifestFromReader` to `readManifestFromReaderAt`, `zipReaderFromReader` to `zipReaderFromReaderAt`, and update `PreviewPackage`, both `Importer` methods, and `readImportPackage` to the signatures in the Interfaces block. Update all production and test call sites in the same commit so it compiles; there must be no transitional adapter that copies an `io.Reader` into memory.

- [x] **Step 3: Run and commit**

Run: `go test ./internal/migration -count=1`

Expected: PASS and this constructor-specific scan returns no output:

```bash
rg -n 'io\.ReadAll\(io\.LimitReader\(r, maxCompressedBytes' internal/migration/zip.go
```

```bash
git add -p internal/migration/importer.go internal/migration/zip.go internal/migration/migration_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "refactor: read migration archives from files"
```

### Task 6: Stream Migration Attachments

**Files:**
- Modify: `internal/migration/files.go`
- Modify: `internal/migration/exporter.go`
- Modify: `internal/migration/migration_test.go`
- Create: `internal/migration/migration_benchmark_test.go`

**Interfaces:**
- Removes `filePayload.Data []byte`.
- Produces `writeFileEntries(ctx, zw, resolver, refs) ([]FileManifest, []string, error)`.
- Preserves paths, sizes, SHA256 values, and warnings.
- Uses `storage.SaveFile(ctx, backend, reader, filename, maxJSONEntryBytes)` during import so size/hash are calculated while streaming and an oversized object is deleted by the storage helper.

- [x] **Step 1: Add streaming export/import tests**

Use a 32 MiB deterministic backend reader whose `Read` fails if asked for more than 128 KiB. Export, inspect size/hash, import through an incremental backend, and assert the stored data and hash. Add focused cases for:

- an attachment unavailable before its ZIP entry is created becomes a warning and export continues;
- a declared size above 64 MiB becomes a warning before entry creation;
- a mid-entry read failure aborts the whole export;
- a ZIP writer failure aborts the whole export;
- an import size mismatch and hash mismatch each delete the newly stored object before returning an error;
- every backend reader and ZIP entry reader is closed, including failures.

- [x] **Step 2: Stream export entries**

Delete `filePayload` and `collectFilePayloads`. Keep `collectFileRefs`, deduplicate by `StorageKey + "\x00" + StoragePath`, and process one backend object at a time. Before calling `zw.CreateHeader`, treat an unavailable backend/object, unsafe path, or `reader.Size > maxJSONEntryBytes` as the same warning class used today and continue.

Once `CreateHeader` succeeds, any read, write, context, or close error aborts the export. A partial ZIP entry cannot be removed safely, so never append a warning and continue after entry creation. Use a retained hasher and a bounded reader:

```go
hasher := sha256.New()
limited := &io.LimitedReader{R: reader.Body, N: maxJSONEntryBytes + 1}
n, copyErr := io.CopyBuffer(
    io.MultiWriter(entry, hasher),
    limited,
    make([]byte, 64<<10),
)
closeErr := reader.Body.Close()
if err := errors.Join(copyErr, closeErr); err != nil {
    return nil, nil, fmt.Errorf("stream %s attachment: %w", ref.Kind, err)
}
if n > maxJSONEntryBytes {
    return nil, nil, fmt.Errorf("stream %s attachment: file is too large", ref.Kind)
}
manifestFile := FileManifest{
    Path:        zipPath,
    StorageKey:  ref.StorageKey,
    StoragePath: ref.StoragePath,
    Size:        n,
    SHA256:      hex.EncodeToString(hasher.Sum(nil)),
}
```

Return the completed manifest entries and warnings. In `Exporter.Export`, write data JSON first, call `writeFileEntries`, populate `manifest.Files`, `manifest.TotalFileBytes`, `manifest.Counts["files"]`, and `manifest.Warnings`, then append `manifest.json` last. ZIP readers already locate it by name, so the public archive format remains compatible. On every error, join the `zip.Writer.Close` error with the primary error instead of discarding it.

- [x] **Step 3: Stream import entries**

Locate each manifest path without using `readZipEntry`, open the ZIP entry, resolve the backend, and call:

```go
obj, saveErr := storage.SaveFile(
    ctx,
    backend,
    rc,
    path.Base(file.StoragePath),
    maxJSONEntryBytes,
)
closeErr := rc.Close()
if err := errors.Join(saveErr, closeErr); err != nil {
    var deleteErr error
    if obj.Path != "" {
        deleteErr = backend.Delete(ctx, obj.Path)
    }
    return nil, nil, errors.Join(fmt.Errorf("save attachment: %w", err), deleteErr)
}
if obj.Size != file.Size || !strings.EqualFold(obj.SHA256, file.SHA256) {
    deleteErr := backend.Delete(ctx, obj.Path)
    if obj.Size != file.Size {
        return nil, nil, errors.Join(errors.New("attachment size mismatch"), deleteErr)
    }
    return nil, nil, errors.Join(errors.New("attachment hash mismatch"), deleteErr)
}
```

Close the entry before continuing and only add to `pathMap` after size/hash verification. If the manifest path is missing, duplicated, or resolves to a directory, return the existing attachment read error. Do not call `readZipEntry` for attachments.

- [x] **Step 4: Create and run the attachment benchmark**

Create `BenchmarkMigrationAttachmentStreaming` in `migration_benchmark_test.go`. Each case exports exactly one deterministic attachment through a backend whose `Open` returns an `io.NopCloser(io.LimitReader(zeroReader{}, size))`; use 1 MiB and 32 MiB sub-benchmarks, `io.Discard` as the archive destination, `b.ReportAllocs()`, and `b.Loop()`:

```go
func BenchmarkMigrationAttachmentStreaming(b *testing.B) {
    for _, size := range []int64{1 << 20, 32 << 20} {
        b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
            exporter := newAttachmentBenchmarkExporter(b, size)
            b.ReportAllocs()
            for b.Loop() {
                if _, err := exporter.Export(b.Context(), io.Discard, Options{IncludeApps: true, IncludeFiles: true}); err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}
```

Change the existing test helpers to accept `testing.TB` so the benchmark can reuse the exact migration fixture:

```go
func newMigrationTestDB(t testing.TB) *ent.Client
func seedMigrationData(t testing.TB, db *ent.Client)
```

Use this complete streaming backend in the benchmark file; every `Open` returns a fresh reader:

```go
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
    clear(p)
    return len(p), nil
}

type attachmentBenchmarkBackend struct {
    size int64
}

func (b attachmentBenchmarkBackend) Open(context.Context, string) (storage.Reader, error) {
    return storage.Reader{
        Body: io.NopCloser(io.LimitReader(zeroReader{}, b.size)),
        Size: b.size,
    }, nil
}

func (attachmentBenchmarkBackend) Save(context.Context, string, io.Reader) (storage.Object, error) {
    return storage.Object{}, errors.New("benchmark backend is read-only")
}

func (attachmentBenchmarkBackend) Delete(context.Context, string) error { return nil }
func (attachmentBenchmarkBackend) PublicURL(string) string              { return "" }

func newAttachmentBenchmarkExporter(b *testing.B, size int64) *Exporter {
    b.Helper()
    db := newMigrationTestDB(b)
    seedMigrationData(b, db)
    backend := attachmentBenchmarkBackend{size: size}
    resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) {
        return backend, nil
    })
    return NewExporter(db, resolver, "benchmark")
}
```

Setup is outside the timed loop. This benchmark must exist before running the command below.

- [x] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/migration -count=1
go test ./internal/migration -run '^$' -bench '^BenchmarkMigrationAttachmentStreaming$' -benchmem -count=5
```

Expected: PASS; `B/op` may include ZIP compression state but must not increase by approximately 31 MiB between the 1 MiB and 32 MiB cases, proving there is no archive-sized attachment slice.

```bash
git add internal/migration/migration_benchmark_test.go
git add -p internal/migration/files.go internal/migration/exporter.go internal/migration/migration_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "perf: stream migration attachments"
```

### Task 7: Move Migration HTTP and Backups to Temporary Files

**Files:**
- Create: `internal/server/migration_files.go`
- Modify: `internal/server/handlers_migration.go`
- Modify: `internal/server/backup.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/server/backup_test.go`

**Interfaces:**
- Produces `migrationUpload{File *os.File, Size int64, Values url.Values}` with idempotent `Cleanup() error`.
- Produces `readMigrationUpload(w, r) (*migrationUpload, bool)`.
- Produces `func migrationImportOptions(values url.Values, actorUserID int) migration.ImportOptions`.
- Produces `func (s *Server) exportMigrationFile(ctx context.Context, options migration.Options) (path string, manifest *migration.Manifest, err error)`.
- The request body cap is `maxMigrationUploadBytes + 64<<10`; the file itself remains capped at exactly 512 MiB and all text fields together at exactly 64 KiB.

- [x] **Step 1: Add cleanup and pre-header failure tests**

Set `TMPDIR` to `t.TempDir()`. For preview, import, export, and backup success/failure, assert no `appstore-migration-*.zip` remains. Make export fail and assert HTTP 200 was not sent. Add multipart tests for:

- exactly 512 MiB accepted without allocating it in the test process (use a generated reader);
- 512 MiB + 1 byte rejected;
- request overhead up to 64 KiB accepted by `MaxBytesReader`;
- cumulative text fields above 64 KiB rejected;
- duplicate `file` parts, a missing `file` part, and a second file-like field rejected;
- a cleanup call repeated twice returns the same joined result and leaves no temp file;
- import options come only from `migrationUpload.Values`, not `r.FormValue` after streaming multipart parsing.

- [x] **Step 2: Stream multipart upload to a 0600 file**

Define the exact upload type and cleanup behavior in `migration_files.go`:

```go
const maxMigrationTextBytes int64 = 64 << 10

type migrationUpload struct {
    File       *os.File
    Size       int64
    Values     url.Values
    cleanupOnce sync.Once
    cleanupErr  error
}

func (u *migrationUpload) Cleanup() error {
    if u == nil || u.File == nil {
        return nil
    }
    u.cleanupOnce.Do(func() {
        name := u.File.Name()
        closeErr := u.File.Close()
        removeErr := os.Remove(name)
        if errors.Is(removeErr, fs.ErrNotExist) {
            removeErr = nil
        }
        u.cleanupErr = errors.Join(closeErr, removeErr)
    })
    return u.cleanupErr
}
```

`readMigrationUpload` first wraps the body, then iterates `r.MultipartReader()` parts:

```go
r.Body = http.MaxBytesReader(w, r.Body, maxMigrationUploadBytes+maxMigrationTextBytes)
reader, err := r.MultipartReader()
```

Accept exactly one part with `FormName() == "file"` and non-empty `FileName()`. Create it with `os.CreateTemp("", "appstore-migration-upload-*.zip")` (0600 by contract), copy with an `io.LimitedReader{N: maxMigrationUploadBytes + 1}`, reject `n > maxMigrationUploadBytes`, join every part close error, and remove/close on every failure. Reject any other part with a non-empty filename. For text parts, read at most `remaining+1`, increment a cumulative byte count, and call `values.Add(part.FormName(), string(raw))`; reject when cumulative bytes exceed `maxMigrationTextBytes`. After EOF, require one file, seek it to offset zero, and return its actual copied size.

Keep the `MaxBytesReader` overflow branch mapped to HTTP 413 `VALIDATION_ERROR`; malformed multipart remains HTTP 400. File-size overflow is HTTP 422. Do not call `ParseMultipartForm`, `FormFile`, `FormValue`, or `io.ReadAll` on the archive.

- [x] **Step 3: Update preview and import**

Defer cleanup immediately after a successful read, and pass the `*os.File` directly to the ReaderAt APIs:

```go
upload, ok := readMigrationUpload(w, r)
if !ok {
    return
}
defer func() { _ = upload.Cleanup() }()

preview, err := importer.Preview(r.Context(), upload.File, upload.Size)
```

For import, parse values exactly once:

```go
func migrationImportOptions(values url.Values, actorUserID int) migration.ImportOptions {
    return migration.ImportOptions{
        Options: migration.Options{
            IncludeSite:   valueBool(values.Get("includeSite")),
            IncludePeople: valueBool(values.Get("includePeople")),
            IncludeApps:   valueBool(values.Get("includeApps")),
            IncludeFiles:  valueBool(values.Get("includeFiles")),
        },
        Mode:           migration.ImportMode(values.Get("mode")),
        ConfirmReplace: values.Get("confirmReplace"),
        ActorUserID:    actorUserID,
    }
}

func valueBool(value string) bool {
    value = strings.TrimSpace(strings.ToLower(value))
    return value == "1" || value == "true" || value == "yes" || value == "on"
}
```

Call `Importer.Import(r.Context(), upload.File, upload.Size, options)`. Preserve Task 4 of the runtime-hardening plan: the checked/flushed migration result must be written successfully before `requestRestart()` closes the restart channel.

- [x] **Step 4: Export before sending headers**

Implement the exact receiver from the Interfaces block:

```go
func (s *Server) exportMigrationFile(ctx context.Context, options migration.Options) (string, *migration.Manifest, error) {
    file, err := os.CreateTemp("", "appstore-migration-export-*.zip")
    if err != nil {
        return "", nil, err
    }
    filePath := file.Name()
    exporter := migration.NewExporter(s.db, s.migrationStorageResolver(), appVersion())
    manifest, exportErr := exporter.Export(ctx, file, options)
    closeErr := file.Close()
    if err := errors.Join(exportErr, closeErr); err != nil {
        removeErr := os.Remove(filePath)
        if errors.Is(removeErr, fs.ErrNotExist) {
            removeErr = nil
        }
        return "", nil, errors.Join(err, removeErr)
    }
    return filePath, manifest, nil
}
```

The HTTP handler calls `exportMigrationFile`, defers removal, opens the completed file, stats it, defers a checked close, and only then sets download headers and calls `http.ServeContent(w, r, filename, info.ModTime(), file)`. Open/stat/export errors occur before headers. A read/close failure after `ServeContent` is logged or joined into the handler's existing error path but cannot be converted into a second JSON response.

- [x] **Step 5: Reuse the file for backup**

Call `s.exportMigrationFile(ctx, migration.DefaultOptions())`, defer removal, open once to hash through `io.Copy(sha256.New(), file)`, stat for size, and join the read/close errors before any target upload. Reopen separately for each target:

```go
source, openErr := os.Open(filePath)
if openErr != nil {
    targetResult.Error = openErr.Error()
    result.Targets = append(result.Targets, targetResult)
    continue
}
obj, saveErr := target.writer.SaveObject(ctx, objectPath, source)
closeErr := source.Close()
saveErr = errors.Join(saveErr, closeErr)
```

Record the per-target open/save/close failure and continue to the remaining targets. Never reuse a consumed file offset. Use the manifest returned by `exportMigrationFile` for counts/warnings and the streamed hash/stat for result SHA256/size.

- [x] **Step 6: Verify and commit**

Run: `go test -race ./internal/migration ./internal/server -run 'Migration|Backup' -count=1`

Expected: PASS and both scoped scans return no output:

```bash
rg -n 'bytes\.Buffer|io\.ReadAll' internal/server/handlers_migration.go internal/server/migration_files.go
rg -n 'bytes\.Buffer|bytes\.NewReader\(buf\.Bytes\(\)\)' internal/server/backup.go
```

```bash
git add internal/server/migration_files.go
git add -p internal/server/handlers_migration.go internal/server/backup.go internal/server/server_test.go internal/server/backup_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "perf: stream migration and backup archives"
```

### Task 8: Measure and Consolidate App Download Statistics

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/app_metrics.go`
- Create: `internal/server/app_metrics_test.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- Benchmarks 50/100 apps with 0/10,000/20,000 download events.
- Reports `queries/op`, `ns/op`, `B/op`, and `allocs/op`.
- Produces `func (s *Server) nowUTC() time.Time` and injects `Server.now func() time.Time` for deterministic period boundaries.
- Produces `downloadPeriodStarts{Day, Week, Month, Year time.Time}` and `downloadPeriodStartsAt(now, loc)`.
- Produces `func (s *Server) downloadCountsByPeriod(ctx context.Context, appIDs []int, starts downloadPeriodStarts) (map[int]downloadStats, error)` only when the evidence gate passes.
- Produces the test seam `queryContextExecutor`/`Server.metricsSQL`, initialized to the Task 4 raw `sqlDB`, so the benchmark counts the raw aggregation query as well as Ent queries.

- [x] **Step 1: Add a fixed clock and boundary tests before changing query shape**

Add `now func() time.Time` and `metricsSQL queryContextExecutor` to `Server`; initialize them in `New` as `now: time.Now` and `metricsSQL: sqlDB`. Use these exact helpers:

```go
type queryContextExecutor interface {
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Server) nowUTC() time.Time {
    if s != nil && s.now != nil {
        return s.now().UTC()
    }
    return time.Now().UTC()
}

type downloadPeriodStarts struct {
    Day   time.Time
    Week  time.Time
    Month time.Time
    Year  time.Time
}

func downloadPeriodStartsAt(now time.Time, loc *time.Location) downloadPeriodStarts {
    return downloadPeriodStarts{
        Day:   downloadPeriodStart(now, loc, downloadPeriodDay).UTC(),
        Week:  downloadPeriodStart(now, loc, downloadPeriodWeek).UTC(),
        Month: downloadPeriodStart(now, loc, downloadPeriodMonth).UTC(),
        Year:  downloadPeriodStart(now, loc, downloadPeriodYear).UTC(),
    }
}
```

Replace `time.Now()` with `s.nowUTC()` in `applyAppListSort`, `loadAppSummaryDownloadStats`, and `recordAppDownload`. Do not replace unrelated scheduler/backup clocks in this task.

In `app_metrics_test.go`, load `Asia/Shanghai` and set:

```go
fixedNow := time.Date(2026, time.July, 8, 12, 0, 0, 0, location)
srv.now = func() time.Time { return fixedNow }
starts := downloadPeriodStartsAt(fixedNow, location)
```

For app A, seed events at one second before and exactly at each of year/month/week/day starts; expect `{Day: 1, Week: 3, Month: 5, Year: 7}`. For app B, seed one event an hour before `fixedNow` and one second before the year start; expect `{Day: 1, Week: 1, Month: 1, Year: 1}`. At this step assert the existing `loadAppSummaryDownloadStats` four-query implementation returns these values; Step 4 applies the same fixture directly to the one-query candidate. Also test `applyAppListSort` and `recordAppDownload` use the fixed instant.

- [x] **Step 2: Expand the benchmark and add a real query counter**

Move `BenchmarkPreloadAppSummaries` from `server_test.go` into `app_metrics_test.go` and replace it with the complete matrix `apps={50,100}` × `events={0,10000,20000}`. Each sub-benchmark uses a separate database and a fixed `Asia/Shanghai` clock. Distribute events deterministically across app IDs and across the four boundaries so every aggregate branch is exercised.

Construct the benchmark Ent client with `ent.Debug()` and an `ent.Log` callback. Count raw SQL through:

```go
type countingQueryer struct {
    next  queryContextExecutor
    count *atomic.Int64
}

func (q countingQueryer) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
    q.count.Add(1)
    return q.next.QueryContext(ctx, query, args...)
}
```

The benchmark setup must use Task 4's `dbpool.Open`, construct the Ent client with `ent.NewClient(ent.Driver(driver), ent.Debug(), ent.Log(func(...any) { queries.Add(1) }))`, and set `srv.metricsSQL = countingQueryer{next: sqlDB, count: &queries}`. Reset `queries` after all schema/fixture setup and immediately before `b.ResetTimer()`. Count loop iterations explicitly:

```go
iterations := int64(0)
queries.Store(0)
b.ResetTimer()
for b.Loop() {
    preload, err := srv.preloadAppSummaries(b.Context(), records, nil)
    if err != nil {
        b.Fatal(err)
    }
    benchmarkAppSummaries = materializeBenchmarkSummaries(srv, records, preload)
    iterations++
}
b.StopTimer()
b.ReportMetric(float64(queries.Load())/float64(iterations), "queries/op")
```

`materializeBenchmarkSummaries` contains the existing DTO loop moved out of the timed function body; it must return the same `[]appSummary` currently assigned to `benchmarkAppSummaries`. Use `b.ReportAllocs()`.

- [x] **Step 3: Capture the ten-run baseline**

```bash
command -v benchstat >/dev/null || go install golang.org/x/perf/cmd/benchstat@latest
go test ./internal/server -run 'DownloadPeriodBoundary|AppMetricsClock' -count=1
go test ./internal/server -run '^$' -bench '^BenchmarkPreloadAppSummaries$' -benchmem -count=10 > /tmp/appstore-preload-old.txt
```

Expected: tests pass and every matrix case reports the same baseline `queries/op`; the download-stat portion accounts for four of those queries. Preserve `/tmp/appstore-preload-old.txt` until the gate is complete.

- [x] **Step 4: Write the failing single-query test**

Add `TestDownloadCountsByPeriodUsesOneQuery`. Set `srv.metricsSQL = countingQueryer{next: srv.sqlDB, count: &queries}`, call `downloadCountsByPeriod` for both app IDs and the fixed starts, assert the exact app A/app B counts from Step 1, and assert `queries.Load() == 1`. Run:

```bash
go test ./internal/server -run '^TestDownloadCountsByPeriodUsesOneQuery$' -count=1
```

Expected: FAIL because `downloadCountsByPeriod` does not exist.

- [x] **Step 5: Implement one bound conditional-aggregation query**

Use `s.metricsSQL.QueryContext` with driver-aware placeholders. Fall back to `s.sqlDB` only for older directly constructed tests, and return `errors.New("app metrics database is not configured")` if both are nil. Generate the `IN` clause starting after the four CASE timestamps:

```go
func bindVars(driver string, start, count int) string {
    vars := make([]string, count)
    for i := range count {
        if driver == "postgres" {
            vars[i] = "$" + strconv.Itoa(start+i)
        } else {
            vars[i] = "?"
        }
    }
    return strings.Join(vars, ",")
}

dayVar := bindVars(s.cfg.DBDriver, 1, 1)
weekVar := bindVars(s.cfg.DBDriver, 2, 1)
monthVar := bindVars(s.cfg.DBDriver, 3, 1)
yearVar := bindVars(s.cfg.DBDriver, 4, 1)
inVars := bindVars(s.cfg.DBDriver, 5, len(appIDs))
yearFloorVar := bindVars(s.cfg.DBDriver, 5+len(appIDs), 1)
query := fmt.Sprintf(`
SELECT app_id,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS day_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS week_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS month_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS year_count
FROM app_downloads
WHERE app_id IN (%s)
  AND created_at >= %s
GROUP BY app_id`, dayVar, weekVar, monthVar, yearVar, inVars, yearFloorVar)
```

Build and scan arguments without interpolating data:

```go
if len(appIDs) == 0 {
    return map[int]downloadStats{}, nil
}
queryer := s.metricsSQL
if queryer == nil {
    queryer = s.sqlDB
}
if queryer == nil {
    return nil, errors.New("app metrics database is not configured")
}

args := []any{starts.Day, starts.Week, starts.Month, starts.Year}
for _, appID := range appIDs {
    args = append(args, appID)
}
args = append(args, starts.Year)

rows, err := queryer.QueryContext(ctx, query, args...)
if err != nil {
    return nil, err
}
defer rows.Close()

out := make(map[int]downloadStats, len(appIDs))
for rows.Next() {
    var appID int
    var stats downloadStats
    if err := rows.Scan(&appID, &stats.Day, &stats.Week, &stats.Month, &stats.Year); err != nil {
        return nil, err
    }
    out[appID] = stats
}
if err := rows.Err(); err != nil {
    return nil, err
}
return out, nil
```

PostgreSQL uses `$1` through `$N`; SQLite/MySQL use `?`. Bind every timestamp and ID; never interpolate values. `loadAppSummaryDownloadStats` computes `starts := downloadPeriodStartsAt(s.nowUTC(), s.siteLocation(ctx))`, calls this method once, and copies all four counters into `out` while preserving each existing `Total` value.

- [x] **Step 6: Run correctness tests and compare ten runs**

```bash
go test ./internal/server -run 'DownloadPeriodBoundary|AppMetricsClock|DownloadCountsByPeriod|DownloadStats' -count=1
go test ./internal/server -run '^$' -bench '^BenchmarkPreloadAppSummaries$' -benchmem -count=10 > /tmp/appstore-preload-new.txt
benchstat /tmp/appstore-preload-old.txt /tmp/appstore-preload-new.txt
```

Acceptance: `queries/op` drops by exactly 3, no case regresses `ns/op` or `B/op` by more than 5%, and a download-heavy case improves or is statistically neutral.

- [x] **Step 7: Apply the evidence gate without an unconditional commit**

If accepted, retain the conditional query and use the performance commit below. If rejected, reverse only the uncommitted `downloadCountsByPeriod`/single-query wiring with `apply_patch`, restore the existing four-period `downloadCountsSince` loop exactly, delete the now-inapplicable one-query-count test, and retain the fixed clock, boundary tests, matrix benchmark, and query-counter test support. Record the exact `benchstat` decision in the chosen commit body. Do not make a second commit for the branch that was not selected.

- [x] **Step 8: Commit exactly one branch**

`internal/server/app_metrics.go` is pre-existing untracked WIP. Mark it intent-to-add and stage only this task's hunks; never stage the unrelated AppDownload/AppVote content wholesale.

Accepted branch:

```bash
git add internal/server/app_metrics_test.go
git add -N internal/server/app_metrics.go
git add -p internal/server/app_metrics.go internal/server/server.go internal/server/server_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "perf: reduce app metrics query overhead" -m "Accepted after the recorded ten-run benchstat gate."
```

Rejected branch:

```bash
git add internal/server/app_metrics_test.go
git add -N internal/server/app_metrics.go
git add -p internal/server/app_metrics.go internal/server/server.go internal/server/server_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "test: expand app metrics benchmarks" -m "Kept the four-query implementation because the recorded ten-run benchstat gate failed."
```

### Task 9: Verify Resource and Performance Hardening

**Files:**
- Modify only files already owned by Tasks 1–8 when verification exposes a defect.

- [x] **Step 1: Run unit and race tests**

```bash
go test ./internal/dbpool ./internal/migration ./internal/server ./internal/clientserver -count=1
go test -race ./internal/dbpool ./internal/migration ./internal/server ./internal/clientserver -count=1
```

Expected: PASS without a race report.

- [x] **Step 2: Run static checks**

```bash
go vet ./internal/dbpool ./internal/migration ./internal/server ./internal/clientserver
GOLANGCI_LINT="$(command -v golangci-lint || true)"
if [ -z "$GOLANGCI_LINT" ]; then
  GOLANGCI_LINT="$(find "$HOME/.local/share/mise/installs/golangci-lint" -type f -name 'golangci-lint*' -perm -111 2>/dev/null | sort -V | tail -n 1)"
fi
test -n "$GOLANGCI_LINT"
"$GOLANGCI_LINT" run --timeout=5m ./internal/dbpool/... ./internal/migration/... ./internal/server/... ./internal/clientserver/...
```

Expected: no finding introduced by this plan.

- [x] **Step 3: Verify resource invariants**

```bash
go mod verify
rg -n 'context\.WithoutCancel' internal/server/singleflight.go
rg -n 'http\.DefaultClient' internal/clientserver --glob '*.go'
rg -n 'io\.ReadAll\(io\.LimitReader\(r, maxCompressedBytes' internal/migration/zip.go
rg -n 'bytes\.Buffer|io\.ReadAll' internal/server/handlers_migration.go internal/server/migration_files.go
rg -n 'bytes\.Buffer|bytes\.NewReader\(buf\.Bytes\(\)\)' internal/server/backup.go
```

Expected: dependency verification passes and every scoped `rg` command returns no output. Do not broaden these scans to test fixtures, authentication token buffers, or unrelated small bounded payloads.

- [x] **Step 4: Re-run the Task 8 decision benchmark**

Repeat the ten-run benchmark. For the accepted branch, compare it to `/tmp/appstore-preload-old.txt` and require the same gate result; for the rejected branch, require the retained four-query benchmark to remain within the original baseline's 5% noise band. Expected: the Task 8 decision remains reproducible.

- [x] **Step 5: Commit verification-only fixes if required**

Do not create an empty commit. If a fix was required, use `git add -p` for every existing file, ordinary `git add` only for a new file created by the relevant task, inspect the cached diff, and commit:

```bash
git add -N internal/server/app_metrics.go
git add -p \
  internal/dbpool/open.go internal/dbpool/open_test.go internal/dbpool/sqlite_benchmark_test.go \
  internal/config/config.go internal/config/config_test.go \
  internal/clientserver/config.go internal/clientserver/config_test.go internal/clientserver/server.go internal/clientserver/db.go \
  internal/clientserver/respond.go internal/clientserver/install.go internal/clientserver/comments.go internal/clientserver/settings.go \
  internal/clientserver/sources.go internal/clientserver/sync.go internal/clientserver/chat.go internal/clientserver/assets.go internal/clientserver/user.go \
  internal/clientserver/http_clients.go internal/clientserver/http_clients_test.go internal/clientserver/source_policy.go internal/clientserver/source_policy_test.go \
  internal/server/respond.go internal/server/respond_test.go internal/server/server.go internal/server/handlers_auth.go internal/server/login_throttle_test.go \
  internal/server/handlers_migration.go internal/server/migration_files.go internal/server/backup.go internal/server/server_test.go internal/server/backup_test.go \
  internal/server/app_metrics.go internal/server/app_metrics_test.go \
  internal/migration/importer.go internal/migration/zip.go internal/migration/files.go internal/migration/exporter.go internal/migration/migration_test.go internal/migration/migration_benchmark_test.go \
  docs/security/software-source-trust.md
git diff --cached --check
git diff --cached --unified=0
git commit -m "test: complete backend resource hardening"
```

- Backend resource/performance hardening Tasks 1–9: complete; unit/race suites, vet, lint, module verification, resource scans, migration streaming benchmark, and both performance decision gates passed.
- SQLite pool decision: retain 1 open / 1 idle. The 4-connection candidate had zero lock errors but no statistically significant throughput improvement (`~`, p=0.912), so it failed the required 5% gate.
- App metrics decision: retain the production four-query aggregation. The one-query candidate reduced `queries/op` from 13 to 10 but was about 1.6–2.5× slower with 10k/20k download events; it remains test-only evidence rather than production wiring.
- The migration resource scan's only `io.ReadAll` match is the intentionally bounded multipart text-field read (`remaining + 1`, cumulative 64 KiB), not archive buffering.
