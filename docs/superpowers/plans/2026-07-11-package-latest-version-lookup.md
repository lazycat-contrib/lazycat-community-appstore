# Package Latest-Version Lookup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let update clients retrieve the latest approved version through an exact LPK package identifier.

**Architecture:** Add a read-only route that resolves the unique `App.package_id`, applies the existing request visibility predicate, and selects the same newest approved version ordering used by app summaries. Return a compact envelope containing the package ID and existing version DTO; use the existing indistinguishable `APP_NOT_FOUND` response for absent, hidden, unpublished, and versionless apps.

**Tech Stack:** Go 1.26, net/http ServeMux, Ent, OpenAPI 3.0, Go testing.

## Global Constraints

- Public endpoint: `GET /api/v1/packages/{packageId}/latest-version`.
- `packageId` must be matched exactly after trimming; it is a unique Ent field.
- Return only an `APPROVED` app and an `APPROVED` version ordered by `published_at DESC, created_at DESC`.
- Reuse `userCanSeeApp` and accept valid group codes through the existing request parser; do not reveal whether a hidden or missing package exists.
- Use the existing structured `404 APP_NOT_FOUND` response for every unavailable result.
- Do not add dependencies or change existing API contracts.

---

### Task 1: Add the exact latest-version API and contract

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/handlers_apps.go`
- Create: `internal/server/package_latest_version_test.go`
- Modify: `docs/openapi.yaml`
- Test: `internal/server/package_latest_version_test.go`

**Interfaces:**
- Produces `GET /api/v1/packages/{packageId}/latest-version`.
- Success response: `{"packageId":"community.lazycat.example","latestVersion":<Version>}`.
- Unavailable response: existing `404` body with `error.code == "APP_NOT_FOUND"`.

- [x] **Step 1: Write focused failing handler tests**

Create `internal/server/package_latest_version_test.go` with an approved app, two approved versions, and unavailable cases:

```go
func TestPackageLatestVersion(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.latest-test").
		SetName("Latest Test").
		SetSlug("latest-test").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	store.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("1.0.0").SetStatus(appversion.StatusAPPROVED).SetPublishedAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).SaveX(ctx)
	store.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("2.0.0").SetStatus(appversion.StatusAPPROVED).SetPublishedAt(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)).SaveX(ctx)
	store.server.db.App.Create().SetOwnerID(admin.ID).SetPackageID("community.lazycat.versionless").SetName("Versionless").SetSlug("versionless").SetStatus(app.StatusAPPROVED).SaveX(ctx)
	store.server.db.App.Create().SetOwnerID(admin.ID).SetPackageID("community.lazycat.pending").SetName("Pending").SetSlug("pending").SetStatus(app.StatusPENDING).SaveX(ctx)

	rec := store.do(http.MethodGet, "/api/v1/packages/community.lazycat.latest-test/latest-version", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"2.0.0"`) {
		t.Fatalf("latest version response = %d %s", rec.Code, rec.Body.String())
	}

	for _, packageID := range []string{"community.lazycat.missing", "community.lazycat.versionless", "community.lazycat.pending"} {
		rec := store.do(http.MethodGet, "/api/v1/packages/"+packageID+"/latest-version", nil)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "APP_NOT_FOUND") {
			t.Fatalf("package %s response = %d %s", packageID, rec.Code, rec.Body.String())
		}
	}
}
```

Also create the `versionless` approved app and a `PENDING` app whose endpoint must return the same `404` response.

- [x] **Step 2: Run the new test to confirm the route is absent**

Run:

```bash
go test ./internal/server -run '^TestPackageLatestVersion$'
```

Expected: FAIL because the unregistered route returns `404` without `APP_NOT_FOUND`.

- [x] **Step 3: Register and implement the smallest exact lookup**

In `internal/server/server.go`, register the route before the generic app routes:

```go
s.mux.HandleFunc("GET /api/v1/packages/{packageId}/latest-version", s.handleGetPackageLatestVersion)
```

In `internal/server/handlers_apps.go`, add:

```go
func (s *Server) handleGetPackageLatestVersion(w http.ResponseWriter, r *http.Request) {
	packageID := strings.TrimSpace(r.PathValue("packageId"))
	record, err := s.db.App.Query().Where(app.PackageIDEQ(packageID)).Only(r.Context())
	if err != nil || record.Status != app.StatusAPPROVED || !s.userCanSeeApp(r, record, s.optionalUser(r)) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	latest, err := s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		First(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packageId": record.PackageID, "latestVersion": toVersionDTO(latest)})
}
```

- [x] **Step 4: Document the public contract**

Add `/api/v1/packages/{packageId}/latest-version` to `docs/openapi.yaml`, with an exact `packageId` path parameter, the success envelope using the existing `Version` schema, and the standard `404 Error` response.

- [x] **Step 5: Run focused and contract verification**

Run:

```bash
gofmt -w internal/server/handlers_apps.go internal/server/package_latest_version_test.go internal/server/server.go
go test ./internal/server -run '^TestPackageLatestVersion$'
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
```

Expected: all commands exit 0.

- [~] **Step 6: Run release verification, bump versions, and push**

## Completion Evidence

- Exact lookup added: `GET /api/v1/packages/{packageId}/latest-version`.
- The response exposes only the latest approved version of an approved application visible to the caller, including callers with a valid group code; all unavailable states use `404 APP_NOT_FOUND`.
- Verified 2026-07-11: focused API test, `go test ./...`, `go vet ./...`, `go test -race ./...`, golangci-lint, `go mod tidy -diff`, `go mod verify`, and OpenAPI validation all passed before version bump.
- Server/client metadata updated to `0.1.29` / `0.1.24`; both local LPKs pass `lzc-cli lpk info` and `lzc-cli lpk lint`. `[~]` remains until the verified commit is pushed.

Run:

```bash
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
go mod tidy -diff
go mod verify
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
git diff --check
```

If all checks pass, increase both LazyCat package patch versions from `0.1.28` / `0.1.23` to `0.1.29` / `0.1.24`, rebuild both local LPKs, inspect them with `lzc-cli lpk info`, stage only the changed endpoint, docs, generated embed assets, version files, and plan/spec files, then commit and push `main`.
