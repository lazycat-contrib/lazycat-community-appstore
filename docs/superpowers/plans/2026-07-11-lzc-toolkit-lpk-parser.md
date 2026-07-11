# lzc-toolkit-go LPK Parser Migration Implementation Plan

> **For agentic workers:** Execute inline in the existing isolated worktree. Do not delegate tasks or alter the URL-fetching policy.

**Goal:** Replace the in-repository LPK metadata archive parser with `github.com/lib-x/lzc-toolkit-go` and remove `internal/lpkmeta`.

**Architecture:** `internal/lpkinspect` remains the boundary for upload parsing and bounded remote LPK acquisition. It opens archives through toolkit `lpk.OpenReaderAt`/`lpk.OpenFile`, obtains package metadata with `EffectiveManifest` and `PackageInfo`, and reads only the selected icon entry through `OpenEntry`. A local metadata DTO preserves the existing server-facing contract; it contains no archive parsing code.

**Tech Stack:** Go 1.26, `github.com/lib-x/lzc-toolkit-go@v0.3.1`, existing `catalogmeta`, multipart uploads, and `net/http`.

## Global Constraints

- Preserve the external download URL policy, mirror selection, checksum calculation, and maximum LPK size.
- Do not expose package contents, access tokens, or install credentials in errors.
- Parse only toolkit-supported LPK layouts; malformed or package-less archives remain deterministic failures.
- Preserve icon input limits: a selected icon must not exceed 2 MiB.

---

### Task 1: Add toolkit-backed metadata boundary

**Files:**

- Create: `internal/lpkinspect/metadata.go`
- Modify: `internal/lpkinspect/lpkinspect.go`
- Modify: `go.mod`, `go.sum`
- Test: `internal/lpkinspect/lpkinspect_test.go`

**Interfaces:**

- Produces `type Metadata struct` with the existing application/package, locale, and icon fields.
- Produces `parseLPKReaderAt(ctx context.Context, src io.ReaderAt, size int64, maxInputBytes int64) (Metadata, error)`.
- Produces `parseLPKFile(ctx context.Context, filename string, maxInputBytes int64) (Metadata, error)`.

- [ ] Add a failing upload test containing a toolkit-readable `package.yml` and an icon; assert package ID, version, locale fallbacks, and icon bytes.
- [ ] Add `lzc-toolkit-go@v0.3.1` to the module and implement the two functions using `lpk.OpenReaderAt`/`lpk.OpenFile` with archive input limits.
- [ ] Decode the toolkit `PackageInfo` document into a local DTO only for application-specific `icon` and locale rendering. Read the icon with `Reader.OpenEntry`; reject a selected icon over 2 MiB.
- [ ] Change `ParseUploaded` and remote `inspectFetchCandidate` to use the new helpers while preserving the seek-to-zero behavior.
- [ ] Run `go test ./internal/lpkinspect -count=1`.

### Task 2: Migrate server consumers and remove the old package

**Files:**

- Modify: `internal/server/lpk_fetch.go`
- Modify: `internal/server/assets.go`
- Modify: `internal/server/handlers_apps.go`
- Modify: `internal/server/lpk_inspection.go`
- Delete: `internal/lpkmeta/lpkmeta.go`
- Delete: `internal/lpkmeta/lpkmeta_test.go`

**Interfaces:**

- Server helpers consume `lpkinspect.Metadata` through the existing `lpkInspection` result.
- No import path in the repository references `internal/lpkmeta`.

- [ ] Update metadata argument and return types to `lpkinspect.Metadata` without changing application field precedence or owner authorization.
- [ ] Delete the previous tar/zip/YAML parser and its tests.
- [ ] Run `rg 'internal/lpkmeta|lpkmeta\.'` and require no source-code references.
- [ ] Run `go test ./internal/lpkinspect ./internal/server -count=1`.

### Task 3: Verify the integration

**Files:**

- Modify only files required by failures from Tasks 1–2.

- [ ] Format modified Go files with `gofmt -w`.
- [ ] Run `go test ./...`, `go vet ./...`, `go mod tidy -diff`, `go mod verify`, and `git diff --check`.
- [ ] Record failures separately from pre-existing failures; do not claim completion without clean outputs.
