# Server and Client Dual Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After all feature work is green, release server `0.1.39` and client `0.1.32` from the same reviewed commit, push `main`, publish both component tags, and verify both GitHub/LazyCat release workflows.

**Architecture:** Treat the tracked `clientembed/dist` bundle and two `package.yml` version fields as release sources of truth. Run a deep review and full Release Gate 2.0, bump each component by one patch following the repository's existing independent-tag convention, commit only intended files, push `main`, then push the two tags sequentially and verify structured workflow/release state.

**Tech Stack:** Git, GitHub CLI, GitHub Actions, Go/Node verification, LazyCat LPK v2 manifests.

## Global Constraints

- Do not release until every feature plan and full verification pass.
- Preserve unrelated user work; never stash, reset, clean, or overwrite it.
- Re-read HEAD and full status immediately before commit and before push.
- Server version: `0.1.38` → `0.1.39`; client version: `0.1.31` → `0.1.32`.
- Tags must be `server-v0.1.39` and `client-v0.1.32`, each matching its `package.yml` exactly.
- Stable app-store lane only; no preview/beta tag.
- Push `main` before tags; verify first tag workflow succeeds before pushing the second tag to simplify failure isolation.
- A workflow status page is not artifact proof: inspect each GitHub Release asset and workflow conclusion.

---

## File Map

- `lazycat/server/package.yml`: server release version.
- `lazycat/client/package.yml`: client release version.
- `clientembed/dist/`: tracked frontend bundle regenerated from `client/src`.
- `docs/spec-wish-wall-and-client-access.md`, `tasks/`, `docs/superpowers/plans/`: approved specification and execution records.
- `.github/workflows/ci.yml`: required verification source.
- `.github/workflows/lazycat-release.yml`: tag/version matching and stable publish workflow.

### Task 1: Run deep pre-release review and Release Gate 2.0

**Files:**
- Review: all changes from `162e0de` to working tree/HEAD, then refresh base immediately before release.
- Use: `/home/czyt/.cc-switch/skills/check/scripts/release_gate.py`.

**Interfaces:**
- Produces a release decision with zero unresolved hard stops.

- [ ] **Step 1: Capture worktree and release baseline**

```bash
git status --short --branch -uall
git rev-parse HEAD
git fetch origin --tags
git rev-list --left-right --count origin/main...HEAD
git tag --sort=-version:refname | sed -n '1,20p'
```

Expected: every dirty file traces to the approved scope; local branch is not behind `origin/main`; proposed tags do not exist.

- [ ] **Step 2: Seed Release Gate 2.0**

```bash
python3 /home/czyt/.cc-switch/skills/check/scripts/release_gate.py --root /home/czyt/code/go/lazycat-community-appstore
```

Record its status lines. Fill remaining matrix rows with current evidence for generated bundle, package content, stable app-store lane, CI, GitHub releases, and absence of issue/PR actions.

- [ ] **Step 3: Review the full diff as deep/auth-data-mutation scope**

```bash
git diff --stat 162e0de
git diff --check 162e0de
git diff --name-status 162e0de
git diff 162e0de -- ':!ent' ':!clientembed/dist'
```

Manually verify permission predicates, anonymous DTO redaction, state-transition transactions, block coverage, input limits, source response limits, identity stability, and migration compatibility. Run sequential security, architecture and adversarial passes because the current execution mode does not authorize delegated reviewers.

- [ ] **Step 4: Sweep risky sibling patterns**

```bash
rg -n 'X-LazyCat-Client-User-ID|resolveCommentActor|chatActorFromRequest|handleMarkOutdated|handleSourceIndex' internal/server internal/clientserver
rg -n 'contactEmail|contactOther|clientUserId' client/src internal/server docs/openapi.yaml
rg -n 'CLIENT_BLOCKED|LAZYCAT_CLIENT_REQUIRED' internal client/src docs/openapi.yaml
```

Expected: every trusted-client entry point is classified; private fields have no public DTO leak; error codes are consistently handled.

- [ ] **Step 5: Fix findings and rerun focused tests**

For every confirmed finding, add a regression test before or with the fix, then rerun the smallest affected package. Do not carry unresolved HIGH/CRITICAL findings into release.

### Task 2: Run full repository verification

**Files:**
- Verify: Go packages, client tests/build, OpenAPI, LazyCat YAML.

**Interfaces:**
- Produces: current-session evidence required before version bump and tags.

- [ ] **Step 1: Run Go tests and static checks**

```bash
go test ./...
go vet ./...
go test -race ./...
CGO_ENABLED=0 go test ./...
go mod tidy -diff
```

Expected: every command exits 0 and `go mod tidy -diff` prints no diff.

- [ ] **Step 2: Run frontend tests, audit and build**

```bash
cd client && npm ci
cd client && node --test --experimental-strip-types 'src/**/*.test.mjs'
cd client && npm audit --audit-level=high --registry=https://registry.npmjs.org
cd client && npm run build
```

Expected: tests/build succeed and audit has no high-or-higher vulnerability.

- [ ] **Step 3: Validate API and LazyCat manifests**

```bash
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
npx --yes js-yaml lazycat/server/package.yml
npx --yes js-yaml lazycat/server/lzc-manifest.yml
npx --yes js-yaml lazycat/server/lzc-deploy-params.yml
npx --yes js-yaml lazycat/server/lzc-build.yml
npx --yes js-yaml lazycat/client/package.yml
npx --yes js-yaml lazycat/client/lzc-manifest.yml
npx --yes js-yaml lazycat/client/lzc-build.yml
```

Expected: all validators exit 0. Confirm LPK v2 `min_os_version` stays at server `1.5.2` and client `1.5.0`, with no unsupported fields added.

### Task 3: Update versions and tracked embedded frontend

**Files:**
- Modify: `lazycat/server/package.yml`
- Modify: `lazycat/client/package.yml`
- Regenerate: `clientembed/dist/`

**Interfaces:**
- Produces the exact sources consumed by `server-v0.1.39` and `client-v0.1.32` workflows.

- [ ] **Step 1: Change only top-level package versions**

```yaml
# lazycat/server/package.yml
version: 0.1.39
```

```yaml
# lazycat/client/package.yml
version: 0.1.32
```

Use `apply_patch`; do not use global replacement commands.

- [ ] **Step 2: Rebuild and synchronize tracked frontend assets**

```bash
cd client && npm run build
rsync -a --delete --exclude app-config.js dist/ ../clientembed/dist/
```

Write `clientembed/dist/app-config.js` using the existing neutral tracked configuration, then verify:

```bash
cd client && diff -qr --exclude=app-config.js dist ../clientembed/dist
```

Expected: no differences other than runtime-injected `app-config.js`.

- [ ] **Step 3: Re-run version and bundle checks**

```bash
awk '/^version:/ {print FILENAME, $2; exit}' lazycat/server/package.yml
awk '/^version:/ {print FILENAME, $2; exit}' lazycat/client/package.yml
git diff --check
git status --short --branch -uall
```

Expected: exact target versions and only intended source/spec/plan/generated/version files.

### Task 4: Commit and push main safely

**Files:**
- Stage: only approved feature, test, docs, generated ent, embedded bundle, plan and version files.

**Interfaces:**
- Produces one release HEAD on `origin/main`.

- [ ] **Step 1: Re-read concurrency-sensitive state**

```bash
git rev-parse HEAD
git status --short --branch -uall
git diff --cached --stat
```

If HEAD moved or unknown files appeared since Task 1, stop and reconcile rather than sweeping them into the release.

- [ ] **Step 2: Stage explicit intended paths and inspect index**

```bash
git add ent internal client/src clientembed/dist docs README.md tasks lazycat/server/package.yml lazycat/client/package.yml
git diff --cached --check
git diff --cached --stat
git diff --cached --name-status
```

Expected: no unrelated file, secret, local database, build directory, or LPK artifact.

- [ ] **Step 3: Commit the release**

```bash
git commit -m "feat: add wish wall and client access controls"
```

- [ ] **Step 4: Verify committed state before push**

```bash
git rev-parse HEAD
git status --short --branch -uall
git show --stat --oneline --decorate HEAD
git rev-list --left-right --count origin/main...HEAD
```

Expected: clean worktree and local branch ahead only by intended commits.

- [ ] **Step 5: Push main and re-read remote**

```bash
git push origin main
git fetch origin main
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"
```

Expected: exact commit equality.

### Task 5: Publish and verify server tag

**Files:**
- Tag: `server-v0.1.39` at release HEAD.

**Interfaces:**
- Produces server GitHub Release and LazyCat server update via `.github/workflows/lazycat-release.yml`.

- [ ] **Step 1: Create and push the server tag**

```bash
test -z "$(git tag -l server-v0.1.39)"
git tag -a server-v0.1.39 -m "MiaoMiao Server v0.1.39"
git push origin server-v0.1.39
```

- [ ] **Step 2: Resolve and poll the structured workflow state**

```bash
gh run list --workflow lazycat-release.yml --branch server-v0.1.39 --limit 1 --json databaseId,status,conclusion,headSha,url
SERVER_RUN_ID=$(gh run list --workflow lazycat-release.yml --branch server-v0.1.39 --limit 1 --json databaseId --jq '.[0].databaseId')
test -n "$SERVER_RUN_ID"
gh run view "$SERVER_RUN_ID" --json status,conclusion,headSha,url,jobs
```

Poll with bounded waits no longer than 60 seconds per call until completed. Expected: `conclusion=success` and `headSha` equals release HEAD.

- [ ] **Step 3: Verify release asset and metadata**

```bash
gh release view server-v0.1.39 --json tagName,name,url,assets
```

Download the LPK asset to a `mktemp -d` directory, compute SHA256, and inspect package metadata with `lzc-cli lpk info` when available. At minimum assert one `.lpk` asset exists and has a non-empty digest reported by GitHub.

### Task 6: Publish and verify client tag

**Files:**
- Tag: `client-v0.1.32` at the same release HEAD.

**Interfaces:**
- Produces client GitHub Release and LazyCat client update via `.github/workflows/lazycat-release.yml`.

- [ ] **Step 1: Create and push the client tag after server success**

```bash
test -z "$(git tag -l client-v0.1.32)"
git tag -a client-v0.1.32 -m "MiaoMiao Client v0.1.32"
git push origin client-v0.1.32
```

- [ ] **Step 2: Resolve and poll structured workflow state**

```bash
gh run list --workflow lazycat-release.yml --branch client-v0.1.32 --limit 1 --json databaseId,status,conclusion,headSha,url
CLIENT_RUN_ID=$(gh run list --workflow lazycat-release.yml --branch client-v0.1.32 --limit 1 --json databaseId --jq '.[0].databaseId')
test -n "$CLIENT_RUN_ID"
gh run view "$CLIENT_RUN_ID" --json status,conclusion,headSha,url,jobs
```

Expected: `conclusion=success`, correct HEAD.

- [ ] **Step 3: Verify client release asset and metadata**

```bash
gh release view client-v0.1.32 --json tagName,name,url,assets
```

Download/read back the `.lpk`, verify SHA256/digest and inspect metadata where `lzc-cli` is available.

### Task 7: Final remote-state audit

**Files:**
- Read-only audit of branch, tags, releases, CI and app-store publish jobs.

**Interfaces:**
- Produces final completion ledger with no hidden pending action.

- [ ] **Step 1: Re-read branch/tag state**

```bash
git fetch origin --tags
git status --short --branch -uall
git rev-parse HEAD origin/main server-v0.1.39 client-v0.1.32
git ls-remote --tags origin refs/tags/server-v0.1.39 refs/tags/client-v0.1.32
```

Expected: clean tree, branch/tag commit identities resolved to the release commit (annotated tag objects may require `^{}` dereference when comparing).

- [ ] **Step 2: Re-read required CI and release jobs**

```bash
gh run list --commit "$(git rev-parse HEAD)" --limit 20 --json workflowName,status,conclusion,url
gh release view server-v0.1.39 --json url,assets
gh release view client-v0.1.32 --json url,assets
```

Expected: CI and both release workflows successful; both release assets present; workflow LazyCat publish steps successful or an exact external blocker reported.

- [ ] **Step 3: Report shipped state using the check completion ledger**

State commit hash, pushed branch, both tags, both release URLs/assets, CI results, LazyCat publish step results, review depth/findings, new test count, and any unavailable local LPK inspection as a named verification gap.
