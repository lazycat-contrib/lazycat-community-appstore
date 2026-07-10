# Backend Runtime and Concurrency Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make both Go binaries shut down predictably, keep SSE connections healthy, serialize source-password rotation, and ensure every background goroutine and shared load has a bounded lifecycle.

**Architecture:** Keep the server and client applications separate, but give each binary one testable command path and use a small shared HTTP runner for signal-aware startup and ordered shutdown. The server owns a lifecycle context and restart event; goroutines are tracked with Go 1.26 `WaitGroup.Go`, request waiters retain cancellation, and long-lived SSE writes refresh their own deadlines.

**Tech Stack:** Go 1.26.4, `net/http`, `context`, `os/signal`, `sync`, `testing/synctest`, Ent, SQLite/PostgreSQL/MySQL.

## Global Constraints

- Preserve all existing API routes and public JSON behavior.
- Do not modify generated Ent files manually.
- Do not modify frontend source, locale files, or generated frontend distributions.
- Keep ordinary API operations bounded while allowing SSE to remain connected indefinitely.
- Use Go features up to and including 1.26.4.
- Do not introduce a goroutine pool, `sync.Pool`, distributed lock, or new production runtime dependency. `go.uber.org/goleak` is allowed only as a test dependency for the required leak regression.
- Every concurrency fix needs a deterministic regression test and must pass `go test -race`.
- Preserve all pre-existing uncommitted work in the target files.
- Before the first edit, save `git status --short -- <owned files>` and `git diff -- <owned files>` in the execution transcript. For each pre-existing untracked owned file, also save `git diff --no-index /dev/null <path> || true`. For every already-modified or pre-existing untracked file use `git add -p` (run `git add -N <path>` first for untracked files); never stage an existing WIP hunk. Files first created by the current task may use ordinary `git add`.
- Before every commit run `git diff --cached --check` and inspect `git diff --cached --unified=0`; the staged patch must contain only the current task's additions.

---

## File Map

- Complete pre-existing untracked `internal/httpserve/run.go`: shared HTTP serving and ordered shutdown.
- Complete pre-existing untracked `internal/httpserve/run_test.go`: startup, cancellation, shutdown timeout, and cleanup tests.
- Complete pre-existing untracked `internal/servercmd/execute.go`: server binary bootstrap and restart selection.
- Complete pre-existing untracked `internal/clientcmd/execute.go`: client binary bootstrap.
- Modify `cmd/store-server/main.go`: minimal entrypoint calling `servercmd.Execute`.
- Modify `cmd/store-client/main.go`: minimal entrypoint calling `clientcmd.Execute`.
- Modify `internal/config/config.go`: runtime timeout settings and validation.
- Modify `internal/config/config_test.go`: runtime and session-secret validation tests.
- Modify `internal/clientserver/config.go`: client runtime timeout settings and validation.
- Complete pre-existing untracked `internal/clientserver/config_test.go`: client configuration validation tests.
- Modify `internal/server/server.go`: lifecycle context, restart channel, source-password mutex, and ordered close.
- Modify `internal/server/chat_hub.go`: document and count non-blocking slow-subscriber drops.
- Complete pre-existing untracked `internal/server/chat_hub_test.go`: drop-policy regression.
- Modify `internal/server/handlers_migration.go`: event-driven restart after response flush.
- Complete pre-existing untracked `internal/server/lifecycle_test.go`: restart and close ordering tests.
- Complete pre-existing untracked `internal/server/sse.go`: checked SSE writes with renewable deadlines.
- Modify `internal/server/handlers_chat.go`: use checked SSE writer.
- Modify `internal/clientserver/chat.go`: use checked streaming proxy copy.
- Complete pre-existing untracked `internal/server/sse_test.go`: long-lived heartbeat and write-failure tests.
- Complete pre-existing untracked `internal/clientserver/chat_sse_test.go`: proxy cancellation, EOF, and write-failure tests.
- Modify `internal/server/settings.go`: atomic source-password rotation.
- Complete pre-existing untracked `internal/server/settings_concurrency_test.go`: concurrent rotation tests.
- Modify `internal/server/singleflight.go`: cancellable `DoChan` waiting.
- Complete pre-existing untracked `internal/server/singleflight_test.go`: waiter cancellation and shared load tests.
- Modify `internal/clientserver/scheduler.go`: track startup sync and close in order.
- Complete pre-existing untracked `internal/clientserver/scheduler_test.go`: scheduler cancellation and leak regression tests.
- Modify `go.mod` and `go.sum`: add the test-only `go.uber.org/goleak` dependency in the scheduler task.

### Task 1: Validate Runtime Configuration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/clientserver/config.go`
- Modify (pre-existing untracked): `internal/clientserver/config_test.go`

**Interfaces:**
- Produces: `func (c config.Config) Validate() error`
- Produces: `func (c clientserver.Config) Validate() error`
- Produces server fields: `ReadHeaderTimeout`, `IdleTimeout`, `ShutdownTimeout`, `MaxHeaderBytes`.
- Produces client fields: `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ShutdownTimeout`, `MaxHeaderBytes`.

- [x] **Step 1: Add failing server configuration tests**

Add tests that keep loopback development valid and reject a public deployment using the built-in session secret:

```go
func TestConfigValidateRejectsDefaultSecretOnPublicURL(t *testing.T) {
    cfg := Load()
    cfg.BaseURL = "https://store.example.com"
    cfg.SitePublicURL = "https://store.example.com"
    cfg.SessionSecret = defaultSessionSecret

    if err := cfg.Validate(); err == nil {
        t.Fatal("Validate() error = nil, want insecure session secret error")
    }
}

func TestConfigValidateAllowsDefaultSecretForLoopbackDevelopment(t *testing.T) {
    cfg := Load()
    cfg.BaseURL = "http://localhost:8080"
    cfg.SitePublicURL = "http://127.0.0.1:8080"
    cfg.SessionSecret = defaultSessionSecret

    if err := cfg.Validate(); err != nil {
        t.Fatalf("Validate() error = %v", err)
    }
}
```

- [x] **Step 2: Run the server configuration tests and confirm failure**

Run: `go test ./internal/config -run 'TestConfigValidate' -count=1`

Expected: FAIL because `Validate` and `defaultSessionSecret` do not exist.

- [x] **Step 3: Add explicit server runtime defaults and validation**

Add the constant and append the listed fields to the existing `Config` struct:

```go
const defaultSessionSecret = "dev-session-secret-change-me"

ReadHeaderTimeout time.Duration
ReadTimeout       time.Duration
WriteTimeout      time.Duration
IdleTimeout       time.Duration
ShutdownTimeout   time.Duration
MaxHeaderBytes    int
```

Set exact defaults in `Load`:

```go
SessionSecret:      env("SESSION_SECRET", defaultSessionSecret),
ReadHeaderTimeout:  5 * time.Second,
ReadTimeout:        10 * time.Second,
WriteTimeout:       60 * time.Second,
IdleTimeout:        2 * time.Minute,
ShutdownTimeout:    10 * time.Second,
MaxHeaderBytes:     1 << 20,
```

Add validation using parsed URL hostnames rather than string prefix checks:

```go
func (c Config) Validate() error {
    if strings.TrimSpace(c.SessionSecret) == "" {
        return errors.New("SESSION_SECRET is required")
    }
    if c.SessionSecret == defaultSessionSecret && (!loopbackURL(c.BaseURL) || !loopbackURL(c.SitePublicURL)) {
        return errors.New("SESSION_SECRET must be changed for non-loopback deployments")
    }
    if c.ReadHeaderTimeout <= 0 || c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 {
        return errors.New("HTTP timeout values must be positive")
    }
    if c.MaxHeaderBytes < 64<<10 {
        return errors.New("MaxHeaderBytes must be at least 65536")
    }
    return nil
}
```

`loopbackURL` must accept only `localhost` and literal loopback IPs; missing or public hostnames return false:

```go
func loopbackURL(raw string) bool {
    parsed, err := url.Parse(strings.TrimSpace(raw))
    if err != nil {
        return false
    }
    host := parsed.Hostname()
    if strings.EqualFold(host, "localhost") {
        return true
    }
    ip := net.ParseIP(host)
    return ip != nil && ip.IsLoopback()
}
```

- [x] **Step 4: Add client configuration tests and implementation**

Use the same policy for `CLIENT_SESSION_SECRET`, based on the configured listen address:

```go
const defaultClientSessionSecret = "dev-client-session-secret-change-me"

func (c Config) Validate() error {
    if strings.TrimSpace(c.SessionSecret) == "" {
        return errors.New("CLIENT_SESSION_SECRET is required")
    }
    host, _, err := net.SplitHostPort(c.Addr)
    if err != nil {
        return fmt.Errorf("parse CLIENT_ADDR: %w", err)
    }
    ip := net.ParseIP(host)
    loopback := strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
    if c.SessionSecret == defaultClientSessionSecret && !loopback {
        return errors.New("CLIENT_SESSION_SECRET must be changed for non-loopback deployments")
    }
    if c.ReadHeaderTimeout <= 0 || c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 {
        return errors.New("HTTP timeout values must be positive")
    }
    if c.MaxHeaderBytes < 64<<10 {
        return errors.New("MaxHeaderBytes must be at least 65536")
    }
    return nil
}
```

Add client runtime defaults:

```go
ReadHeaderTimeout: 5 * time.Second,
ReadTimeout:       30 * time.Second,
WriteTimeout:      60 * time.Second,
IdleTimeout:       2 * time.Minute,
ShutdownTimeout:   10 * time.Second,
MaxHeaderBytes:    1 << 20,
```

- [x] **Step 5: Run configuration tests**

Run: `go test ./internal/config ./internal/clientserver -run 'Config|Load' -count=1`

Expected: PASS.

- [x] **Step 6: Commit runtime configuration validation**

```bash
git add -N internal/clientserver/config_test.go
git add -p internal/clientserver/config_test.go internal/config/config.go internal/config/config_test.go internal/clientserver/config.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: validate runtime configuration"
```

### Task 2: Add a Shared Ordered HTTP Runner

**Files:**
- Modify (pre-existing untracked): `internal/httpserve/run.go`
- Modify (pre-existing untracked): `internal/httpserve/run_test.go`

**Interfaces:**
- Produces: `type Options struct { ShutdownTimeout time.Duration; Restart <-chan struct{}; Stop func(context.Context) error; Close func(context.Context) error }`
- Produces: `func Run(ctx context.Context, server *http.Server, options Options) error`
- Produces: `func RunListener(ctx context.Context, server *http.Server, listener net.Listener, options Options) error` for deterministic tests.
- Shutdown order is fixed: close listener → stop/cancel application work → graceful HTTP shutdown → force `Server.Close` on timeout → close dependencies. `Stop`, HTTP shutdown, and dependency close share one timeout context.

- [x] **Step 1: Write failing cancellation and cleanup tests**

Use a real loopback listener so the test never binds a fixed port. Add these complete helpers to `run_test.go`:

```go
func newTestListener(t *testing.T) net.Listener {
    t.Helper()
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = listener.Close() })
    return listener
}

func requireHTTP200(t *testing.T, addr net.Addr) {
    t.Helper()
    resp, err := http.Get("http://" + addr.String())
    if err != nil {
        t.Fatal(err)
    }
    defer func() { _ = resp.Body.Close() }()
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("status = %d, want 200", resp.StatusCode)
    }
}

type failingListener struct {
    err error
}

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (l failingListener) Close() error              { return nil }
func (l failingListener) Addr() net.Addr            { return &net.TCPAddr{} }
```

The first test records phase order:

```go
func TestRunShutsDownThenClosesDependencies(t *testing.T) {
    ctx, cancel := context.WithCancel(t.Context())
    listener := newTestListener(t)
    var mu sync.Mutex
    phases := []string{}
    record := func(phase string) {
        mu.Lock()
        phases = append(phases, phase)
        mu.Unlock()
    }
    server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        _, _ = io.WriteString(w, "ok")
    })}

    done := make(chan error, 1)
    go func() {
        done <- RunListener(ctx, server, listener, Options{
            ShutdownTimeout: time.Second,
            Stop: func(ctx context.Context) error {
                if _, ok := ctx.Deadline(); !ok {
                    t.Error("Stop context has no deadline")
                }
                record("stop")
                return nil
            },
            Close: func(ctx context.Context) error {
                if _, ok := ctx.Deadline(); !ok {
                    t.Error("Close context has no deadline")
                }
                record("close")
                return nil
            },
        })
    }()

    requireHTTP200(t, listener.Addr())
    cancel()

    if err := <-done; err != nil {
        t.Fatalf("RunListener() error = %v", err)
    }
    mu.Lock()
    defer mu.Unlock()
    if diff := cmp.Diff([]string{"stop", "close"}, phases); diff != "" {
        t.Fatalf("phase order mismatch (-want +got):\n%s", diff)
    }
}
```

- [x] **Step 2: Run the runner tests and confirm failure**

Run: `go test ./internal/httpserve -count=1`

Expected: FAIL because the package does not exist.

- [x] **Step 3: Implement ordered serve, shutdown, wait, and close**

Implement both production and injectable-listener entrypoints. `normalizeServeError` treats `http.ErrServerClosed` and `net.ErrClosed` as normal termination:

```go
type Options struct {
    ShutdownTimeout time.Duration
    Restart         <-chan struct{}
    Stop            func(context.Context) error
    Close           func(context.Context) error
}

func Run(ctx context.Context, server *http.Server, options Options) error {
    if options.ShutdownTimeout <= 0 {
        return errors.New("ShutdownTimeout must be positive")
    }
    listener, err := net.Listen("tcp", server.Addr)
    if err != nil {
        return err
    }
    return RunListener(ctx, server, listener, options)
}

func RunListener(ctx context.Context, server *http.Server, listener net.Listener, options Options) error {
    if options.ShutdownTimeout <= 0 {
        _ = listener.Close()
        return errors.New("ShutdownTimeout must be positive")
    }
    serveErr := make(chan error, 1)
    go func() {
        serveErr <- normalizeServeError(server.Serve(listener))
    }()

    var triggerErr error
    serveDone := false
    select {
    case triggerErr = <-serveErr:
        serveDone = true
    case <-ctx.Done():
        // Signal or parent cancellation is a normal termination trigger.
    case <-options.Restart:
        // A nil Restart channel disables this case.
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), options.ShutdownTimeout)
    defer cancel()

    listenerErr := listener.Close()
    if errors.Is(listenerErr, net.ErrClosed) {
        listenerErr = nil
    }

    var stopErr error
    if options.Stop != nil {
        stopErr = options.Stop(shutdownCtx)
    }

    shutdownErr := server.Shutdown(shutdownCtx)
    var forceErr error
    if shutdownErr != nil {
        forceErr = server.Close()
    }

    var finalServeErr error
    if !serveDone {
        select {
        case finalServeErr = <-serveErr:
        case <-shutdownCtx.Done():
            forceErr = errors.Join(forceErr, server.Close())
            finalServeErr = <-serveErr
        }
    }

    var closeErr error
    if options.Close != nil {
        closeErr = options.Close(shutdownCtx)
    }
    return errors.Join(triggerErr, listenerErr, stopErr, shutdownErr, forceErr, finalServeErr, closeErr)
}

func normalizeServeError(err error) error {
    if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
        return nil
    }
    return err
}
```

The serve goroutine uses a buffered result channel and is always joined before dependency close. A timed-out `Shutdown` always calls `server.Close()` before waiting for `serveErr`, preventing the previous permanent wait.

- [x] **Step 4: Add shutdown-timeout and serve-error tests**

Add these tests with channel synchronization:

- `TestRunListenerForcesCloseAfterShutdownTimeout`: a handler closes `entered`, waits on `r.Context().Done()`, and closes `exited`. Cancel the runner after `entered`; use a 20 ms timeout. Assert `RunListener` returns an error satisfying `errors.Is(err, context.DeadlineExceeded)` and `exited` closes before the test returns. This proves force-close unblocks `Serve`.
- `TestRunListenerClosesDependenciesAfterServeError`: pass `failingListener{err: errServe}`; assert `errors.Is(err, errServe)` and that `Close` still runs.
- `TestRunListenerJoinsStopAndCloseErrors`: make both callbacks return sentinel errors and assert both are found with `errors.Is`.
- `TestRunListenerRejectsMissingTimeout`: pass zero and assert the exact error `ShutdownTimeout must be positive`.

- [x] **Step 5: Run runner tests under race detection**

Run: `go test -race ./internal/httpserve -count=1`

Expected: PASS with no race report.

- [x] **Step 6: Commit the runner**

```bash
git add -N internal/httpserve/run.go internal/httpserve/run_test.go
git add -p internal/httpserve/run.go internal/httpserve/run_test.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "feat: add ordered HTTP runner"
```

### Task 3: Move Both Binaries to Testable Command Paths

**Files:**
- Modify (pre-existing untracked): `internal/servercmd/execute.go`
- Modify (pre-existing untracked): `internal/servercmd/execute_test.go`
- Modify (pre-existing untracked): `internal/clientcmd/execute.go`
- Modify (pre-existing untracked): `internal/clientcmd/execute_test.go`
- Modify: `cmd/store-server/main.go`
- Modify: `cmd/store-client/main.go`

**Interfaces:**
- Consumes: `httpserve.Run`
- Produces: `func servercmd.Execute() int`
- Produces: `func servercmd.Run(ctx context.Context) error`
- Produces: `func clientcmd.Execute() int`
- Produces: `func clientcmd.Run(ctx context.Context) error`
- At this task boundary the server adapts the existing `SetRestartAfterImport` callback into a channel. Task 4 replaces only that adapter with `RestartRequested`, so every intermediate commit compiles.

- [x] **Step 1: Replace main functions with minimal entrypoints**

Server main:

```go
package main

import (
    "os"

    "lazycat.community/appstore/internal/servercmd"
)

func main() {
    os.Exit(servercmd.Execute())
}
```

Client main:

```go
package main

import (
    "os"

    "lazycat.community/appstore/internal/clientcmd"
)

func main() {
    os.Exit(clientcmd.Execute())
}
```

- [x] **Step 2: Run command package compilation and confirm failure**

Run: `go test ./cmd/store-server ./cmd/store-client`

Expected: FAIL because the command packages do not exist.

- [x] **Step 3: Implement signal-aware server execution**

`servercmd.Execute` creates only the signal context and exit code:

```go
var run = Run

func Execute() int {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    if err := run(ctx); err != nil {
        log.Printf("Store server stopped: %v", err)
        return 1
    }
    return 0
}
```

Implement `Run` with the existing restart callback so Task 3 does not depend on Task 4:

```go
func Run(ctx context.Context) error {
    cfg := config.Load()
    if err := cfg.Validate(); err != nil {
        return fmt.Errorf("validate server config: %w", err)
    }
    app, err := server.New(cfg)
    if err != nil {
        return fmt.Errorf("create server: %w", err)
    }
    restart := make(chan struct{})
    var restartOnce sync.Once
    app.SetRestartAfterImport(func() {
        restartOnce.Do(func() { close(restart) })
    })
    httpServer := &http.Server{
        Addr:              cfg.Addr,
        Handler:           app.Handler(),
        ReadHeaderTimeout: cfg.ReadHeaderTimeout,
        ReadTimeout:       cfg.ReadTimeout,
        WriteTimeout:      cfg.WriteTimeout,
        IdleTimeout:       cfg.IdleTimeout,
        MaxHeaderBytes:    cfg.MaxHeaderBytes,
    }
    return httpserve.Run(ctx, httpServer, httpserve.Options{
        ShutdownTimeout: cfg.ShutdownTimeout,
        Restart:         restart,
        Close: func(context.Context) error {
            return app.Close()
        },
    })
}
```

Do not also `defer app.Close`; the runner owns cleanup after `server.New` succeeds.

- [x] **Step 4: Implement signal-aware client execution**

Implement the client path completely:

```go
func Run(ctx context.Context) error {
    cfg := clientserver.LoadConfig()
    if err := cfg.Validate(); err != nil {
        return fmt.Errorf("validate client config: %w", err)
    }
    app, err := clientserver.New(cfg)
    if err != nil {
        return fmt.Errorf("create client: %w", err)
    }
    httpServer := &http.Server{
        Addr:              cfg.Addr,
        Handler:           app.Handler(),
        ReadHeaderTimeout: cfg.ReadHeaderTimeout,
        ReadTimeout:       cfg.ReadTimeout,
        WriteTimeout:      cfg.WriteTimeout,
        IdleTimeout:       cfg.IdleTimeout,
        MaxHeaderBytes:    cfg.MaxHeaderBytes,
    }
    return httpserve.Run(ctx, httpServer, httpserve.Options{
        ShutdownTimeout: cfg.ShutdownTimeout,
        Close: func(context.Context) error {
            return app.Close()
        },
    })
}
```

Add the complete client entry wrapper:

```go
var run = Run

func Execute() int {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    if err := run(ctx); err != nil {
        log.Printf("Store client stopped: %v", err)
        return 1
    }
    return 0
}
```

Add command tests that set an invalid default secret/public address, call `Run(t.Context())`, and assert validation fails before a listener or database is opened. A second test calls `Execute` through an injected package-private `run = Run` variable returning `errSentinel` and asserts exit code `1`; reset the variable with `t.Cleanup`.

- [x] **Step 5: Compile and run command packages**

Run: `go test ./cmd/store-server ./cmd/store-client ./internal/servercmd ./internal/clientcmd -count=1`

Expected: PASS; the command packages may report `[no test files]` at this stage.

- [x] **Step 6: Commit command bootstrap changes**

```bash
git add -N internal/servercmd/execute.go internal/servercmd/execute_test.go internal/clientcmd/execute.go internal/clientcmd/execute_test.go
git add -p internal/servercmd/execute.go internal/servercmd/execute_test.go internal/clientcmd/execute.go internal/clientcmd/execute_test.go cmd/store-server/main.go cmd/store-client/main.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "refactor: centralize binary lifecycle"
```

### Task 4: Replace Magic Restart Delay With a Restart Event

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/handlers_migration.go`
- Modify (pre-existing untracked): `internal/server/lifecycle_test.go`
- Modify: `internal/servercmd/execute.go`

**Interfaces:**
- Produces: `func (s *Server) RestartRequested() <-chan struct{}`
- Produces internal: `func (s *Server) requestRestart()`
- Produces internal: `func writeMigrationResult(w http.ResponseWriter, status int, payload any) error`; restart is requested only after encode, write, and flush succeed.

- [x] **Step 1: Write a failing restart event test**

```go
func TestRequestRestartClosesChannelOnce(t *testing.T) {
    srv := &Server{restartRequested: make(chan struct{})}
    srv.requestRestart()
    srv.requestRestart()

    select {
    case <-srv.RestartRequested():
    default:
        t.Fatal("restart channel is still open")
    }
}
```

- [x] **Step 2: Run the restart test and confirm failure**

Run: `go test ./internal/server -run TestRequestRestartClosesChannelOnce -count=1`

Expected: FAIL because the restart channel API does not exist.

- [x] **Step 3: Add the restart channel to Server**

Replace the callback field with:

```go
restartAfterImportOnce sync.Once
restartRequested       chan struct{}
```

Initialize the channel in `New` and add:

```go
func (s *Server) RestartRequested() <-chan struct{} {
    return s.restartRequested
}

func (s *Server) requestRestart() {
    s.restartAfterImportOnce.Do(func() {
        close(s.restartRequested)
    })
}
```

- [x] **Step 4: Flush the migration response before requesting restart**

Delete the goroutine and `time.Sleep(750 * time.Millisecond)`. Add a checked response helper that encodes before committing headers:

```go
func writeMigrationResult(w http.ResponseWriter, status int, payload any) error {
    raw, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    raw = append(raw, '\n')
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    if _, err := w.Write(raw); err != nil {
        return err
    }
    if err := http.NewResponseController(w).Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
        return err
    }
    return nil
}

payload := map[string]any{"result": result}
if options.Mode != migration.ImportModeReplace {
    writeJSON(w, http.StatusOK, payload)
    return
}
if err := writeMigrationResult(w, http.StatusOK, payload); err != nil {
    log.Printf("write migration response before restart: %v", err)
    return
}
s.requestRestart()
```

`httpserve.RunListener` receives `RestartRequested()` directly, stops accepting new requests, and `Shutdown` waits for this handler to return. No watcher goroutine or fixed delay is needed. In `servercmd.Run`, delete the Task 3 callback/channel adapter and set `Restart: app.RestartRequested()`.

- [x] **Step 5: Add an HTTP regression test**

Use `httptest.NewRecorder` with a replace-mode import fixture. Assert `rec.Flushed`, decode the JSON result, then assert `RestartRequested` is closed. Add a `failingResponseWriter` whose `Write` returns `io.ErrClosedPipe`; assert the restart channel remains open. Neither test uses `time.Sleep`.

- [x] **Step 6: Run restart tests under race detection**

Run: `go test -race ./internal/server -run 'Restart|MigrationImport' -count=1`

Expected: PASS without a race report or fixed-duration sleep.

- [x] **Step 7: Commit event-driven restart**

```bash
git add -N internal/server/lifecycle_test.go
git add -p internal/server/lifecycle_test.go internal/server/server.go internal/server/handlers_migration.go internal/servercmd/execute.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: restart server after migration response"
```

### Task 5: Make SSE Writes Error-Aware and Deadline-Aware

**Files:**
- Modify (pre-existing untracked): `internal/server/sse.go`
- Modify: `internal/server/handlers_chat.go`
- Modify: `internal/clientserver/chat.go`
- Modify (pre-existing untracked): `internal/server/sse_test.go`
- Modify (pre-existing untracked): `internal/clientserver/chat_sse_test.go`
- Modify: `internal/server/chat_hub.go`
- Modify (pre-existing untracked): `internal/server/chat_hub_test.go`

**Interfaces:**
- Produces server helper: `func writeSSE(w http.ResponseWriter, deadline time.Duration, payload string) error`
- Produces client helper: `func copySSE(ctx context.Context, w http.ResponseWriter, src io.Reader, deadline time.Duration) error`
- Produces: `func (h *chatHub) droppedEvents() uint64`; slow subscribers are non-blocking and dropped hints are counted, while clients continue to reload authoritative conversation state.

- [x] **Step 1: Write failing SSE helper tests**

Test that the helper flushes a heartbeat and propagates a writer error:

```go
func TestWriteSSEPropagatesWriteFailure(t *testing.T) {
    writer := &failingResponseWriter{err: io.ErrClosedPipe}
    err := writeSSE(writer, time.Second, ": heartbeat\n\n")
    if !errors.Is(err, io.ErrClosedPipe) {
        t.Fatalf("writeSSE() error = %v, want closed pipe", err)
    }
}
```

- [x] **Step 2: Run the helper tests and confirm failure**

Run: `go test ./internal/server -run TestWriteSSE -count=1`

Expected: FAIL because `writeSSE` does not exist.

- [x] **Step 3: Implement renewable write deadlines**

```go
func writeSSE(w http.ResponseWriter, deadline time.Duration, payload string) error {
    controller := http.NewResponseController(w)
    if deadline > 0 {
        if err := controller.SetWriteDeadline(time.Now().Add(deadline)); err != nil && !errors.Is(err, http.ErrNotSupported) {
            return err
        }
    }
    if _, err := io.WriteString(w, payload); err != nil {
        return err
    }
    return controller.Flush()
}
```

Use a 30-second deadline for the 25-second server heartbeat. Return from `handleChatEvents` on the first write or marshal failure.

- [x] **Step 4: Replace ignored chat event writes**

Build the event payload once and call `writeSSE`:

```go
raw, err := json.Marshal(event)
if err != nil {
    return
}
if err := writeSSE(w, 30*time.Second, "event: chat\ndata: "+string(raw)+"\n\n"); err != nil {
    return
}
```

The connected comment and heartbeat must use the same helper.

- [x] **Step 5: Implement checked SSE proxy copying in the client**

Add this helper to `internal/clientserver/chat.go` and replace the SSE route's `io.Copy` call:

```go
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
                err := controller.SetWriteDeadline(time.Now().Add(deadline))
                if err != nil && !errors.Is(err, http.ErrNotSupported) {
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
```

Use a 30-second local write deadline. Return from the handler on the first error. Keep the upstream request bound to `r.Context`; do not add an overall upstream timeout in this task.

- [x] **Step 6: Add a long-lived SSE regression test**

Add all of the following:

- Server test: start a real `http.Server` with `WriteTimeout: 20 * time.Millisecond`. The handler emits three heartbeats from a `time.NewTicker(15*time.Millisecond)` using a 25 ms renewable deadline. The client scanner must receive heartbeat three after more than the original 20 ms global deadline. Bound the test with `context.WithTimeout(t.Context(), time.Second)`; do not use `time.Sleep`.
- Server writer test: `failingResponseWriter.Write` returns `io.ErrClosedPipe`; assert `writeSSE` returns it.
- Client proxy EOF test: copy two SSE frames from `strings.NewReader`, assert exact output and nil error.
- Client cancellation test: use `io.Pipe`, cancel the context, close the writer with `context.Canceled`, and assert the helper returns cancellation.
- Client write-failure test: assert `io.ErrClosedPipe` propagates.

- [x] **Step 7: Document and count chat fanout drops**

In `chat_hub.go`, add an `atomic.Uint64` field and update the non-blocking branch:

```go
type chatHub struct {
    mu      sync.Mutex
    clients map[chan chatEvent]struct{}
    dropped atomic.Uint64
}

func (h *chatHub) droppedEvents() uint64 {
    return h.dropped.Load()
}

// broadcast sends invalidation hints only. A slow subscriber may lose a hint;
// every received hint causes the client to reload authoritative state.
func (h *chatHub) broadcast(event chatEvent) {
    h.mu.Lock()
    defer h.mu.Unlock()
    for ch := range h.clients {
        select {
        case ch <- event:
        default:
            h.dropped.Add(1)
        }
    }
}
```

The test fills one subscriber's eight-slot buffer, broadcasts once more, and asserts `droppedEvents() == 1` without blocking.

- [x] **Step 8: Run SSE and fanout tests under race detection**

Run: `go test -race ./internal/server ./internal/clientserver -run 'SSE|ChatEvents|ChatHub' -count=10`

Expected: PASS and no `errcheck` finding for the previous `fmt.Fprint/Fprintf` calls.

- [x] **Step 9: Commit SSE reliability changes**

```bash
git add -N internal/server/sse.go internal/server/sse_test.go internal/server/chat_hub_test.go internal/clientserver/chat_sse_test.go
git add -p internal/server/sse.go internal/server/sse_test.go internal/server/chat_hub_test.go internal/clientserver/chat_sse_test.go internal/server/handlers_chat.go internal/server/chat_hub.go internal/clientserver/chat.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: keep SSE connections reliable"
```

### Task 6: Serialize and Atomically Persist Source Password Rotation

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/settings.go`
- Modify (pre-existing untracked): `internal/server/settings_concurrency_test.go`

**Interfaces:**
- Produces internal mutex field: `sourcePasswordMu sync.Mutex`
- Preserves: `func (s *Server) sourcePassword(ctx context.Context) string`

- [x] **Step 1: Write a concurrent failing test**

Create an expired source password and start 32 callers at one barrier. Collect all returned values and assert a single value was observed and persisted:

```go
func TestSourcePasswordRotationIsAtomic(t *testing.T) {
    app := newTestApp(t)
    ctx := t.Context()
    mustSetSetting(t, app.server, settingSourcePassword, "old-password")
    mustSetSetting(t, app.server, settingSourcePasswordRotation, "1")
    mustSetSetting(t, app.server, sourcePasswordRotatedAtSetting, time.Now().Add(-48*time.Hour).UTC().Format(time.RFC3339))

    const callers = 32
    start := make(chan struct{})
    values := make(chan string, callers)
    var wg sync.WaitGroup
    for range callers {
        wg.Go(func() {
            <-start
            values <- app.server.sourcePassword(ctx)
        })
    }
    close(start)
    wg.Wait()
    close(values)

    unique := map[string]struct{}{}
    for value := range values {
        unique[value] = struct{}{}
    }
    if len(unique) != 1 {
        t.Fatalf("rotated password count = %d, want 1", len(unique))
    }
}
```

- [x] **Step 2: Run the concurrent test repeatedly**

Run: `go test -race ./internal/server -run TestSourcePasswordRotationIsAtomic -count=20`

Expected: FAIL intermittently because multiple tokens can be returned.

- [x] **Step 3: Add a dedicated rotation mutex and transaction**

Lock only the rotation check. Read a best-effort fallback password before opening the transaction, then read the authoritative password, rotation setting, and timestamp through the transaction. Return the pre-read/current password whenever transaction creation, token creation, either setting update, or commit fails. Never return a generated token until commit succeeds.

Use a named constant:

```go
const sourcePasswordRotatedAtSetting = "source_password_rotated_at"
```

Reuse the existing package-level `setSettingTx` in `internal/server/handlers_setup.go`; do not add another function with that name. Add only this read helper to `settings.go`:

```go
func settingValueTx(ctx context.Context, tx *entgo.Tx, key, fallback string) (string, error) {
    record, err := tx.SiteSetting.Query().Where(sitesetting.KeyEQ(key)).Only(ctx)
    if entgo.IsNotFound(err) {
        return fallback, nil
    }
    if err != nil {
        return "", err
    }
    return record.Value, nil
}
```

The complete rotation structure is:

```go
func (s *Server) sourcePassword(ctx context.Context) string {
    s.sourcePasswordMu.Lock()
    defer s.sourcePasswordMu.Unlock()

    fallbackCtx, fallbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
    fallback := s.setting(fallbackCtx, settingSourcePassword, s.cfg.SourcePassword)
    fallbackCancel()
    tx, err := s.db.Tx(ctx)
    if err != nil {
        return fallback
    }
    defer func() { _ = tx.Rollback() }()

    password, err := settingValueTx(ctx, tx, settingSourcePassword, fallback)
    if err != nil {
        return fallback
    }
    rotationRaw, err := settingValueTx(ctx, tx, settingSourcePasswordRotation, strconv.Itoa(s.cfg.SourcePasswordRotation))
    if err != nil {
        return password
    }
    rotationDays, err := strconv.Atoi(rotationRaw)
    if err != nil || rotationDays <= 0 || password == "" {
        return password
    }
    rotatedAtRaw, err := settingValueTx(ctx, tx, sourcePasswordRotatedAtSetting, "")
    if err != nil {
        return password
    }
    rotatedAt, err := time.Parse(time.RFC3339, rotatedAtRaw)
    if err != nil {
        if err := setSettingTx(ctx, tx, sourcePasswordRotatedAtSetting, time.Now().UTC().Format(time.RFC3339)); err != nil {
            return password
        }
        if err := tx.Commit(); err != nil {
            return password
        }
        return password
    }
    if time.Since(rotatedAt) < time.Duration(rotationDays)*24*time.Hour {
        return password
    }
    token, err := randomToken()
    if err != nil {
        return password
    }
    if err := setSettingTx(ctx, tx, settingSourcePassword, token); err != nil {
        return password
    }
    if err := setSettingTx(ctx, tx, sourcePasswordRotatedAtSetting, time.Now().UTC().Format(time.RFC3339)); err != nil {
        return password
    }
    if err := tx.Commit(); err != nil {
        return password
    }
    return token
}
```

- [x] **Step 4: Add persistence and failure-path tests**

Assert the returned token equals the stored token. Add a canceled-context case proving the current persisted password is returned and no token is published. Install this SQLite trigger before the failure-path call:

```sql
CREATE TRIGGER fail_source_password_rotation
BEFORE UPDATE ON site_settings
WHEN NEW.key = 'source_password_rotated_at'
BEGIN
    SELECT RAISE(ABORT, 'forced rotation timestamp failure');
END;
```

Force the timestamp to be expired, call `sourcePassword`, and prove the password update rolls back and the function returns the old persisted password.

- [x] **Step 5: Run rotation tests**

Run: `go test -race ./internal/server -run SourcePassword -count=20`

Expected: PASS with exactly one token per rotation.

- [x] **Step 6: Commit source password rotation**

```bash
git add -N internal/server/settings_concurrency_test.go
git add -p internal/server/settings_concurrency_test.go internal/server/server.go internal/server/settings.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: serialize source password rotation"
```

### Task 7: Make Shared Loads Cancellable

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/singleflight.go`
- Modify (pre-existing untracked): `internal/server/singleflight_test.go`
- Modify: `internal/servercmd/execute.go`

**Interfaces:**
- Adds server lifecycle fields: `ctx`, `cancel`, `loadMu`, `loadsClosing`, `loadWG`, `stopOnce`, `stopDone`, `stopErr`, `closeOnce`, `closeDone`, and `closeErr`.
- Preserves: `func (s *Server) sharedFirstLoad(ctx context.Context, key string, load func(context.Context) (any, error)) (any, error)`.
- Produces: `func (s *Server) Stop(ctx context.Context) error` and `func (s *Server) CloseContext(ctx context.Context) error`; existing `Close() error` remains as a compatibility wrapper.

- [x] **Step 1: Write waiter-cancellation and deduplication tests**

One test must start a blocked load, cancel the request context, and assert the waiter returns `context.Canceled` before the load is released. A second test starts two live callers and asserts the load function runs once.

- [x] **Step 2: Run tests and confirm the cancellation test fails**

Run: `go test ./internal/server -run SharedFirstLoad -count=1`

Expected: the cancellation test hangs or fails because `singleflight.Do` blocks and `WithoutCancel` detaches the load.

- [x] **Step 3: Add a server lifecycle context**

Create one application context in `New`, initialize stop/close channels, and add these exact entries to the existing `Server` literal:

```go
appCtx, appCancel := context.WithCancel(context.Background())
ctx:        appCtx,
cancel:     appCancel,
backupCtx:  appCtx,
stopDone:   make(chan struct{}),
closeDone:  make(chan struct{}),
```

Set `backupCtx` to `appCtx` and remove the separately-created backup context. Startup failure calls `appCancel` before closing the Ent client.

Add a load admission gate so `Wait` cannot race a later `Add`:

```go
func (s *Server) beginSharedLoad() bool {
    s.loadMu.Lock()
    defer s.loadMu.Unlock()
    if s.loadsClosing {
        return false
    }
    s.loadWG.Add(1)
    return true
}

func (s *Server) endSharedLoad() {
    s.loadWG.Done()
}
```

- [x] **Step 4: Replace `Do` with `DoChan`**

```go
func (s *Server) sharedFirstLoad(ctx context.Context, key string, load func(context.Context) (any, error)) (any, error) {
    result := s.firstLoadGroup.DoChan(key, func() (any, error) {
        if !s.beginSharedLoad() {
            return nil, context.Canceled
        }
        defer s.endSharedLoad()
        loadCtx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
        defer cancel()
        return load(loadCtx)
    })
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case item := <-result:
        return item.Val, item.Err
    }
}
```

Add bounded, idempotent lifecycle methods:

```go
func (s *Server) Stop(ctx context.Context) error {
    s.stopOnce.Do(func() {
        s.loadMu.Lock()
        s.loadsClosing = true
        s.loadMu.Unlock()
        s.cancel()
        go func() {
            s.backupWG.Wait()
            s.loadWG.Wait()
            close(s.stopDone)
        }()
    })
    select {
    case <-s.stopDone:
        return s.stopErr
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *Server) CloseContext(ctx context.Context) error {
    stopErr := s.Stop(ctx)
    s.closeOnce.Do(func() {
        go func() {
            s.closeErr = s.db.Close()
            close(s.closeDone)
        }()
    })
    select {
    case <-s.closeDone:
        return errors.Join(stopErr, s.closeErr)
    case <-ctx.Done():
        return errors.Join(stopErr, ctx.Err())
    }
}

func (s *Server) Close() error {
    timeout := s.cfg.ShutdownTimeout
    if timeout <= 0 {
        timeout = 10 * time.Second
    }
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    return s.CloseContext(ctx)
}
```

The `Stop` waiter goroutine is created once and always terminates when the two tracked groups terminate. No application worker is admitted after `loadsClosing` is set.
The database-close goroutine is also created once. If the shared shutdown context expires first, `CloseContext` returns the timeout while `closeDone` remains the join point for later callers; the goroutine performs only the bounded driver close and does not start application work.

Update `servercmd.Run` runner options:

```go
Stop:  app.Stop,
Close: app.CloseContext,
```

- [x] **Step 5: Run shared load tests under race detection**

Run: `go test -race ./internal/server -run 'SharedFirstLoad|ServerStop' -count=20`

Expected: PASS; one canceled waiter does not cancel the shared load for another live waiter, and `Stop` cancels then waits for the actual shared load before returning.

- [x] **Step 6: Commit cancellable shared loads**

```bash
git add -N internal/server/singleflight_test.go
git add -p internal/server/singleflight_test.go internal/server/server.go internal/server/singleflight.go internal/servercmd/execute.go
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: make shared loads cancellable"
```

### Task 8: Track Client Scheduler Goroutines and Close in Order

**Files:**
- Modify: `internal/clientserver/scheduler.go`
- Modify: `internal/clientserver/server.go`
- Modify (pre-existing untracked): `internal/clientserver/scheduler_test.go`
- Modify: `internal/clientcmd/execute.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Adds: `wg sync.WaitGroup` and `startupSync func(context.Context)` to `sourceSyncScheduler`.
- Produces: `func (s *sourceSyncScheduler) Close(ctx context.Context) error`.
- Produces client application methods `Stop(ctx)`, `CloseContext(ctx)`, and compatibility `Close()` using the configured shutdown timeout.

- [x] **Step 1: Write a deterministic startup-sync close test**

Add `newSourceSyncSchedulerWithStartup(server, hook)`; production `newSourceSyncScheduler` calls it with nil. The nil path assigns `scheduler.runStartupSyncs`. The test hook closes `started`, waits on `ctx.Done`, then closes `exited`. Start the scheduler, wait for `started`, call `Close(t.Context())`, and assert `exited` is already closed when `Close` returns.

- [x] **Step 2: Run the scheduler test and confirm failure**

Run: `go test ./internal/clientserver -run SchedulerCloseWaits -count=1`

Expected: FAIL because `Close` does not wait for `runStartupSyncs`.

- [x] **Step 3: Track startup work with Go 1.26 `WaitGroup.Go`**

Replace:

```go
go scheduler.runStartupSyncs()
```

with:

```go
scheduler.wg.Go(func() {
    scheduler.startupSync(scheduler.ctx)
})
```

Change `runStartupSyncs` to accept its context explicitly and use that context for every query and sync. Close in this order:

```go
func (s *sourceSyncScheduler) Close(ctx context.Context) error {
    s.cancel()
    wheelErr := s.wheel.Close()
    done := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(done)
    }()
    select {
    case <-done:
        return wheelErr
    case <-ctx.Done():
        return errors.Join(wheelErr, ctx.Err())
    }
}
```

- [x] **Step 4: Make Server.Close preserve scheduler and database errors**

Add `stopOnce`, `stopDone`, `stopErr`, `closeOnce`, `closeDone`, and `closeErr` fields to `clientserver.Server`; initialize both channels in `New` and `newTestServer`. Implement:

```go
func (s *Server) Stop(ctx context.Context) error {
    s.stopOnce.Do(func() {
        go func() {
            if s.syncScheduler != nil {
                s.stopErr = s.syncScheduler.Close(ctx)
            }
            close(s.stopDone)
        }()
    })
    select {
    case <-s.stopDone:
        return s.stopErr
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *Server) CloseContext(ctx context.Context) error {
    stopErr := s.Stop(ctx)
    s.closeOnce.Do(func() {
        go func() {
            s.closeErr = s.db.Close()
            close(s.closeDone)
        }()
    })
    select {
    case <-s.closeDone:
        return errors.Join(stopErr, s.closeErr)
    case <-ctx.Done():
        return errors.Join(stopErr, ctx.Err())
    }
}
```

Keep existing tests compiling with this wrapper:

```go
func (s *Server) Close() error {
    timeout := s.cfg.ShutdownTimeout
    if timeout <= 0 {
        timeout = 10 * time.Second
    }
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    return s.CloseContext(ctx)
}
```

The scheduler waiter and database-close goroutines are one-shot lifecycle adapters. They are joined through `stopDone`/`closeDone` when the shared shutdown context allows; after a timeout they continue only long enough to observe scheduler cancellation or finish the driver close, and later callers join the same channels rather than starting duplicate goroutines.

Update `clientcmd.Run` runner options to `Stop: app.Stop` and `Close: app.CloseContext`.

- [x] **Step 5: Add a goroutine leak regression check**

Install the test-only leak checker:

```bash
go get go.uber.org/goleak@v1.3.0
```

In `TestSchedulerCloseWaits`, begin with `defer goleak.VerifyNone(t, goleak.IgnoreCurrent())`. The channel-based hook proves logical completion; `goleak` proves the startup worker and its WaitGroup waiter do not remain. Run the focused test 50 times; do not compare raw `runtime.NumGoroutine` counts and do not use fixed sleeps.

- [x] **Step 6: Run scheduler tests**

Run: `go test -race ./internal/clientserver -run 'Scheduler|AutoSync' -count=20`

Expected: PASS with no goroutine growth or database-close race.

- [x] **Step 7: Commit scheduler lifecycle changes**

```bash
git add -N internal/clientserver/scheduler_test.go
git add -p internal/clientserver/scheduler_test.go internal/clientserver/scheduler.go internal/clientserver/server.go internal/clientcmd/execute.go go.mod go.sum
git diff --cached --check
git diff --cached --unified=0
git commit -m "fix: wait for client sync scheduler"
```

### Task 9: Verify Runtime Hardening as a Unit

**Files:**
- Modify only if verification exposes a defect in files already owned by Tasks 1–8.

**Interfaces:**
- Consumes all runtime hardening interfaces.
- Produces a green runtime-hardening checkpoint for the memory/performance plan.

- [x] **Step 1: Run focused unit tests**

Run:

```bash
go test ./internal/httpserve ./internal/servercmd ./internal/clientcmd ./internal/config ./internal/server ./internal/clientserver -count=1
```

Expected: PASS.

- [x] **Step 2: Run focused race tests**

Run:

```bash
go test -race ./internal/httpserve ./internal/server ./internal/clientserver -count=1
go test -race ./internal/clientserver -run SchedulerCloseWaits -count=50
```

Expected: PASS with no race report.

- [x] **Step 3: Run static verification**

Run:

```bash
go vet ./cmd/... ./internal/httpserve ./internal/servercmd ./internal/clientcmd ./internal/config ./internal/server ./internal/clientserver
LINT_BIN="$(command -v golangci-lint || true)"
if [ -z "$LINT_BIN" ]; then
  LINT_BIN="$(find "$HOME/.local/share/mise/installs/golangci-lint" -type f -name 'golangci-lint*' -perm -111 2>/dev/null | sort -V | tail -n 1)"
fi
test -n "$LINT_BIN"
"$LINT_BIN" run --timeout=5m ./cmd/... ./internal/httpserve/... ./internal/servercmd/... ./internal/clientcmd/... ./internal/config/... ./internal/server/... ./internal/clientserver/...
```

Expected: no findings introduced by this plan. Pre-existing findings outside the owned files are reported to the integration task.

- [x] **Step 4: Confirm no magic restart wait remains**

Run:

```bash
rg -n 'time\.Sleep\(750 \* time\.Millisecond\)|context\.WithoutCancel|go scheduler\.runStartupSyncs' internal cmd
rg -n '^\s*go ' internal/httpserve internal/server/singleflight.go internal/server/chat_hub.go internal/clientserver/scheduler.go internal/servercmd internal/clientcmd
```

Expected: the first command has no output. Every result from the second command is one of the explicitly joined serve/wait/close adapter goroutines documented in Tasks 2, 7, and 8; there is no restart watcher, detached shared load, or untracked scheduler worker. `ChatHub` tests confirm slow-subscriber drops are counted.

- [x] **Step 5: Commit verification-only fixes if needed**

If Tasks 1–8 already pass without additional changes, do not create an empty commit. Otherwise stage only the runtime-hardening files and commit:

```bash
git add -N internal/clientserver/config_test.go internal/clientserver/scheduler_test.go internal/server/lifecycle_test.go internal/server/settings_concurrency_test.go internal/server/singleflight_test.go
git add -p cmd/store-server/main.go cmd/store-client/main.go \
  internal/config/config.go internal/config/config_test.go \
  internal/clientserver/config.go internal/clientserver/config_test.go internal/clientserver/server.go internal/clientserver/scheduler.go internal/clientserver/scheduler_test.go internal/clientserver/chat.go internal/clientserver/chat_sse_test.go \
  internal/server/server.go internal/server/handlers_migration.go internal/server/handlers_chat.go \
  internal/server/lifecycle_test.go internal/server/sse.go internal/server/sse_test.go internal/server/chat_hub.go internal/server/chat_hub_test.go internal/server/settings.go internal/server/settings_concurrency_test.go internal/server/singleflight.go internal/server/singleflight_test.go \
  internal/httpserve/run.go internal/httpserve/run_test.go internal/servercmd/execute_test.go internal/clientcmd/execute_test.go \
  internal/servercmd/execute.go internal/clientcmd/execute.go go.mod go.sum
git diff --cached --check
git diff --cached --unified=0
git commit -m "test: complete runtime hardening"
```

- Backend runtime hardening Tasks 1–9: complete; focused tests, race suites, scheduler leak repetitions, vet, lint, and lifecycle scans passed.
