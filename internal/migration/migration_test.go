package migration

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"lazycat.community/appstore/ent"
	apppkg "lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appdownload"
	versionpkg "lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/appvote"
	"lazycat.community/appstore/ent/asset"
	"lazycat.community/appstore/ent/assetlink"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/storage"

	_ "github.com/lib-x/entsqlite"
)

type readerAtSpy struct {
	next  io.ReaderAt
	calls atomic.Int64
}

func (s *readerAtSpy) ReadAt(p []byte, off int64) (int, error) {
	if len(p) > 128<<10 {
		return 0, fmt.Errorf("ReadAt requested %d bytes", len(p))
	}
	s.calls.Add(1)
	return s.next.ReadAt(p, off)
}

func TestPreviewPackageUsesReaderAtWithoutCopyingArchive(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeTestJSON(t, zw, manifestName, Manifest{
		FormatVersion: FormatVersion,
		ServerVersion: "test",
		CreatedAt:     time.Now().UTC(),
		Modules:       []Module{ModuleSite},
	})
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "migration-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	spy := &readerAtSpy{next: file}
	if _, err := PreviewPackage(t.Context(), spy, int64(buf.Len())); err != nil {
		t.Fatal(err)
	}
	if _, err := NewImporter(nil, nil).Preview(t.Context(), spy, int64(buf.Len())); err != nil {
		t.Fatal(err)
	}
	if spy.calls.Load() == 0 {
		t.Fatal("ReaderAt was not used")
	}
	for _, test := range []struct {
		reader io.ReaderAt
		size   int64
	}{
		{reader: nil, size: 0},
		{reader: file, size: -1},
		{reader: file, size: maxCompressedBytes + 1},
	} {
		if _, err := PreviewPackage(t.Context(), test.reader, test.size); err == nil {
			t.Fatalf("PreviewPackage(%v, %d) error = nil", test.reader, test.size)
		}
	}
}

func TestPreviewPackageRejectsMissingManifest(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("data/site.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("{}"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewPackage(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len())); err == nil {
		t.Fatal("PreviewPackage succeeded without manifest")
	}
}

func TestPreviewPackageRejectsTraversalEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("{}"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewPackage(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len())); err == nil {
		t.Fatal("PreviewPackage accepted traversal entry")
	}
}

func TestPreviewPackageReadsManifestCounts(t *testing.T) {
	var buf bytes.Buffer
	manifest := Manifest{
		FormatVersion: FormatVersion,
		ServerVersion: "test",
		CreatedAt:     time.Now().UTC(),
		Modules:       []Module{ModuleSite},
		Counts:        map[string]int{"siteSettings": 2},
	}
	zw := zip.NewWriter(&buf)
	writeTestJSON(t, zw, manifestName, manifest)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewPackage(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("PreviewPackage: %v", err)
	}
	if preview.Counts["siteSettings"] != 2 {
		t.Fatalf("siteSettings count = %d, want 2", preview.Counts["siteSettings"])
	}
}

func TestExporterIncludesSelectedModulesAndManifestCounts(t *testing.T) {
	ctx := context.Background()
	db := newMigrationTestDB(t)
	seedMigrationData(t, db)

	var buf bytes.Buffer
	manifest, err := NewExporter(db, nil, "test").Export(ctx, &buf, Options{IncludeSite: true, IncludePeople: true, IncludeApps: true})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if manifest.Counts["users"] != 1 || manifest.Counts["apps"] != 1 || manifest.Counts["appVisibilities"] != 1 || manifest.Counts["downloads"] != 1 || manifest.Counts["appVotes"] != 1 {
		t.Fatalf("unexpected manifest counts: %#v", manifest.Counts)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{manifestName, "data/site.json", "data/people.json", "data/apps.json"} {
		if _, err := readZipEntry(zr, name, maxJSONEntryBytes); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestExporterOmitsUnselectedModules(t *testing.T) {
	ctx := context.Background()
	db := newMigrationTestDB(t)
	seedMigrationData(t, db)

	var buf bytes.Buffer
	if _, err := NewExporter(db, nil, "test").Export(ctx, &buf, Options{IncludeSite: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readZipEntry(zr, "data/site.json", maxJSONEntryBytes); err != nil {
		t.Fatalf("missing site data: %v", err)
	}
	if _, err := readZipEntry(zr, "data/people.json", maxJSONEntryBytes); err == nil {
		t.Fatal("people data was exported when unselected")
	}
	if _, err := readZipEntry(zr, "data/apps.json", maxJSONEntryBytes); err == nil {
		t.Fatal("apps data was exported when unselected")
	}
}

func TestImporterMergeCreatesRecordsByStableKeys(t *testing.T) {
	ctx := context.Background()
	source := newMigrationTestDB(t)
	seedMigrationData(t, source)
	var buf bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(ctx, &buf, Options{IncludeSite: true, IncludePeople: true, IncludeApps: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	target := newMigrationTestDB(t)
	result, err := NewImporter(target, nil).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ImportOptions{Options: Options{IncludeSite: true, IncludePeople: true, IncludeApps: true}, Mode: ImportModeMerge})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Created == 0 {
		t.Fatalf("created = %d, want > 0", result.Created)
	}
	if target.User.Query().Where(user.UsernameEQ("owner")).CountX(ctx) != 1 {
		t.Fatal("owner user was not imported")
	}
	record := target.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.example")).OnlyX(ctx)
	if record.Author != "LazyCat Community" || record.Homepage != "https://example.com/app" || record.License != "MIT" || record.MinOsVersion != "1.3.0" {
		t.Fatalf("app metadata was not imported: %+v", record)
	}
	if target.AppVersion.Query().Where(versionpkg.AppIDEQ(record.ID), versionpkg.VersionEQ("1.0.0")).CountX(ctx) != 1 {
		t.Fatal("app version was not imported")
	}
	if target.AppVisibility.Query().Where(appvisibility.AppIDEQ(record.ID)).CountX(ctx) != 1 {
		t.Fatal("app visibility was not imported")
	}
	if target.AppDownload.Query().Where(appdownload.AppIDEQ(record.ID)).CountX(ctx) != 1 {
		t.Fatal("app download event was not imported")
	}
	if target.AppVote.Query().Where(appvote.AppIDEQ(record.ID)).CountX(ctx) != 1 {
		t.Fatal("app vote was not imported")
	}
}

func TestMigrationImportsLegacyDownloadVersion(t *testing.T) {
	for _, tc := range []struct {
		name        string
		legacyID    int
		appVersions []AppVersionRecord
		wantVersion string
	}{
		{
			name:     "resolves archived version id",
			legacyID: 41,
			appVersions: []AppVersionRecord{{
				ID:         41,
				AppID:      11,
				UploaderID: 7,
				Version:    "2.4.1",
				Status:     string(versionpkg.StatusAPPROVED),
				SourceType: string(versionpkg.SourceTypeLOCAL),
			}},
			wantVersion: "2.4.1",
		},
		{
			name:        "keeps unresolved event",
			legacyID:    999,
			wantVersion: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
			data := AppsData{
				Apps: []AppRecord{{
					ID:        11,
					OwnerID:   7,
					PackageID: "cloud.lazycat.test.legacy-download",
					Name:      "Legacy Download",
					Slug:      "legacy-download",
					Status:    string(apppkg.StatusAPPROVED),
					CreatedAt: now,
					UpdatedAt: now,
				}},
				AppVersions: tc.appVersions,
				AppDownloads: []AppDownloadRecord{{
					ID:              61,
					AppID:           11,
					LegacyVersionID: tc.legacyID,
					CreatedAt:       now,
				}},
			}
			for idx := range data.AppVersions {
				data.AppVersions[idx].CreatedAt = now
				data.AppVersions[idx].UpdatedAt = now
			}
			archive := testAppsMigrationArchive(t, data)
			target := newMigrationTestDB(t)
			actor := target.User.Create().
				SetUsername("actor").
				SetPasswordHash("hash").
				SetRole(user.RoleSITE_ADMIN).
				SaveX(t.Context())
			result, err := NewImporter(target, nil).Import(
				t.Context(),
				bytes.NewReader(archive),
				int64(len(archive)),
				ImportOptions{
					Options:     Options{IncludeApps: true},
					Mode:        ImportModeMerge,
					ActorUserID: actor.ID,
				},
			)
			if err != nil {
				t.Fatalf("import legacy archive: %v", err)
			}
			if result.Created == 0 {
				t.Fatalf("import result = %+v", result)
			}
			event := target.AppDownload.Query().OnlyX(t.Context())
			if event.Version != tc.wantVersion {
				t.Fatalf("download version = %q, want %q", event.Version, tc.wantVersion)
			}
		})
	}
}

func TestMigrationExportCarriesDownloadVersionSnapshot(t *testing.T) {
	db := newMigrationTestDB(t)
	seedMigrationData(t, db)
	data, err := collectAppsData(t.Context(), db)
	if err != nil {
		t.Fatalf("collect apps data: %v", err)
	}
	if len(data.AppDownloads) != 1 {
		t.Fatalf("downloads = %d, want 1", len(data.AppDownloads))
	}
	record := data.AppDownloads[0]
	if record.Version != "1.0.0" {
		t.Fatalf("download version = %q, want 1.0.0", record.Version)
	}
	if record.LegacyVersionID != 0 {
		t.Fatalf("legacy version id = %d, want 0", record.LegacyVersionID)
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal download record: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"version":"1.0.0"`)) || bytes.Contains(raw, []byte(`"version_id"`)) {
		t.Fatalf("download record JSON = %s", raw)
	}
}

func TestMigrationPreservesAppRetentionOverride(t *testing.T) {
	source := newMigrationTestDB(t)
	seedMigrationData(t, source)
	sourceApp := source.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.example")).OnlyX(t.Context())
	source.App.UpdateOneID(sourceApp.ID).SetVersionRetentionCount(7).SaveX(t.Context())
	var archive bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(t.Context(), &archive, Options{IncludePeople: true, IncludeApps: true}); err != nil {
		t.Fatalf("export: %v", err)
	}
	target := newMigrationTestDB(t)
	if _, err := NewImporter(target, nil).Import(
		t.Context(),
		bytes.NewReader(archive.Bytes()),
		int64(archive.Len()),
		ImportOptions{
			Options:        Options{IncludePeople: true, IncludeApps: true},
			Mode:           ImportModeReplace,
			ConfirmReplace: "RESTORE",
		},
	); err != nil {
		t.Fatalf("replace import: %v", err)
	}
	imported := target.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.example")).OnlyX(t.Context())
	if imported.VersionRetentionCount == nil || *imported.VersionRetentionCount != 7 {
		t.Fatalf("version retention count = %v, want 7", imported.VersionRetentionCount)
	}
}

func TestImporterRestoresAssetsAndRemapsAssetURLs(t *testing.T) {
	ctx := context.Background()
	source := newMigrationTestDB(t)
	seedMigrationData(t, source)
	sourceApp := source.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.example")).OnlyX(ctx)
	payload := []byte("source app icon")
	sum := sha256.Sum256(payload)
	sourceAsset := source.Asset.Create().
		SetSha256(hex.EncodeToString(sum[:])).
		SetMediaType("image/png").
		SetSize(int64(len(payload))).
		SetData(payload).
		SaveX(ctx)
	source.App.UpdateOneID(sourceApp.ID).SetIconURL(assetURL(sourceAsset.ID)).SaveX(ctx)
	source.AssetLink.Create().
		SetAssetID(sourceAsset.ID).
		SetOwnerType(assetOwnerApp).
		SetOwnerID(sourceApp.ID).
		SetRole("icon").
		SaveX(ctx)

	var buf bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(ctx, &buf, Options{IncludePeople: true, IncludeApps: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	target := newMigrationTestDB(t)
	target.Asset.Create().
		SetSha256("0123456789abcdef").
		SetMediaType("image/png").
		SetSize(4).
		SetData([]byte("seed")).
		SaveX(ctx)
	result, err := NewImporter(target, nil).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ImportOptions{Options: Options{IncludePeople: true, IncludeApps: true}, Mode: ImportModeMerge})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Created == 0 {
		t.Fatalf("created = %d, want > 0", result.Created)
	}
	importedAsset := target.Asset.Query().Where(asset.Sha256EQ(hex.EncodeToString(sum[:]))).OnlyX(ctx)
	importedApp := target.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.example")).OnlyX(ctx)
	if importedApp.IconURL == nil || *importedApp.IconURL != assetURL(importedAsset.ID) {
		t.Fatalf("app icon URL = %v, want %q", importedApp.IconURL, assetURL(importedAsset.ID))
	}
	if target.AssetLink.Query().
		Where(assetlink.AssetIDEQ(importedAsset.ID), assetlink.OwnerTypeEQ(assetOwnerApp), assetlink.OwnerIDEQ(importedApp.ID), assetlink.RoleEQ("icon")).
		CountX(ctx) != 1 {
		t.Fatal("asset link was not restored with remapped ids")
	}
}

func TestImporterMergeUpdatesExistingRecordsByStableKeys(t *testing.T) {
	ctx := context.Background()
	source := newMigrationTestDB(t)
	seedMigrationData(t, source)
	var buf bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(ctx, &buf, Options{IncludePeople: true, IncludeApps: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	target := newMigrationTestDB(t)
	u := target.User.Create().SetUsername("owner").SetNickname("Old").SetPasswordHash("old-hash").SetRole(user.RoleUSER).SaveX(ctx)
	target.Category.Create().SetName("Old").SetSlug("tools").SaveX(ctx)
	target.Tag.Create().SetName("Old").SetSlug("infra").SaveX(ctx)
	target.App.Create().SetOwnerID(u.ID).SetPackageID("cloud.lazycat.example").SetName("Old").SetSlug("old-app").SaveX(ctx)
	result, err := NewImporter(target, nil).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ImportOptions{Options: Options{IncludePeople: true, IncludeApps: true}, Mode: ImportModeMerge})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Updated == 0 {
		t.Fatalf("updated = %d, want > 0", result.Updated)
	}
	imported := target.User.Query().Where(user.UsernameEQ("owner")).OnlyX(ctx)
	if imported.Nickname != "Owner" || imported.PasswordHash != "hash-owner" {
		t.Fatalf("user not updated: nickname=%q hash=%q", imported.Nickname, imported.PasswordHash)
	}
}

func TestImporterMergeUpdatesActorCredentialsFromPackage(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	source := newMigrationTestDB(t)
	source.User.Create().
		SetUsername("admin").
		SetNickname("Imported Admin").
		SetPasswordHash("source-hash").
		SetRole(user.RoleUSER).
		SetDisabled(true).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	var buf bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(ctx, &buf, Options{IncludePeople: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	target := newMigrationTestDB(t)
	actor := target.User.Create().
		SetUsername("admin").
		SetNickname("Target Admin").
		SetPasswordHash("target-hash").
		SetRole(user.RoleSITE_ADMIN).
		SetDisabled(false).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	result, err := NewImporter(target, nil).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ImportOptions{
		Options:     Options{IncludePeople: true},
		Mode:        ImportModeMerge,
		ActorUserID: actor.ID,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Updated == 0 {
		t.Fatalf("updated = %d, want > 0", result.Updated)
	}
	imported := target.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	if imported.PasswordHash != "source-hash" || imported.Role != user.RoleUSER || !imported.Disabled {
		t.Fatalf("actor credentials were not updated from package: hash=%q role=%s disabled=%v", imported.PasswordHash, imported.Role, imported.Disabled)
	}
	if imported.Nickname != "Imported Admin" {
		t.Fatalf("non-auth profile fields were not merged: nickname=%q", imported.Nickname)
	}
}

func TestImporterReplaceDeletesTargetUsersAndImportsPackageUsers(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	source := newMigrationTestDB(t)
	source.User.Create().
		SetUsername("imported-admin").
		SetNickname("Imported Admin").
		SetPasswordHash("source-hash").
		SetRole(user.RoleSITE_ADMIN).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	source.User.Create().
		SetUsername("member").
		SetNickname("Member").
		SetPasswordHash("member-hash").
		SetRole(user.RoleUSER).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	var buf bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(ctx, &buf, Options{IncludePeople: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}

	target := newMigrationTestDB(t)
	actor := target.User.Create().
		SetUsername("admin").
		SetNickname("Target Admin").
		SetPasswordHash("target-hash").
		SetRole(user.RoleSITE_ADMIN).
		SetDisabled(false).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	target.User.Create().
		SetUsername("stale").
		SetNickname("Stale").
		SetPasswordHash("stale-hash").
		SetRole(user.RoleUSER).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)

	if _, err := NewImporter(target, nil).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ImportOptions{
		Options:        Options{IncludePeople: true},
		Mode:           ImportModeReplace,
		ConfirmReplace: "OVERWRITE",
		ActorUserID:    actor.ID,
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if target.User.Query().Where(user.UsernameEQ("admin")).CountX(ctx) != 0 {
		t.Fatal("target actor user was preserved even though it is not in the package")
	}
	if target.User.Query().Where(user.UsernameEQ("stale")).CountX(ctx) != 0 {
		t.Fatal("stale user was not removed by replace import")
	}
	imported := target.User.Query().Where(user.UsernameEQ("imported-admin")).OnlyX(ctx)
	if imported.PasswordHash != "source-hash" || imported.Role != user.RoleSITE_ADMIN || imported.Disabled {
		t.Fatalf("package user was not imported exactly: hash=%q role=%s disabled=%v", imported.PasswordHash, imported.Role, imported.Disabled)
	}
	if target.User.Query().Where(user.UsernameEQ("member")).CountX(ctx) != 1 {
		t.Fatal("member user was not imported")
	}
}

func TestImporterReplaceRequiresConfirmation(t *testing.T) {
	ctx := context.Background()
	source := newMigrationTestDB(t)
	seedMigrationData(t, source)
	var buf bytes.Buffer
	if _, err := NewExporter(source, nil, "test").Export(ctx, &buf, Options{IncludePeople: true}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	target := newMigrationTestDB(t)
	if _, err := NewImporter(target, nil).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ImportOptions{Options: Options{IncludePeople: true}, Mode: ImportModeReplace}); err == nil {
		t.Fatal("replace import succeeded without confirmation")
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type oversizeCleanupBackend struct {
	cancel       context.CancelFunc
	deleteCalls  int
	deleteCtxErr []error
}

type boundedZeroReadCloser struct {
	remaining int64
	closed    *atomic.Bool
}

func (r *boundedZeroReadCloser) Read(p []byte) (int, error) {
	if len(p) > 128<<10 {
		return 0, fmt.Errorf("read buffer = %d, want <= 128 KiB", len(p))
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	clear(p[:n])
	r.remaining -= n
	return int(n), nil
}

func (r *boundedZeroReadCloser) Close() error {
	r.closed.Store(true)
	return nil
}

type streamingAttachmentBackend struct {
	size       int64
	openClosed atomic.Bool
	savedSize  int64
	savedHash  string
	deletes    atomic.Int64
}

func (b *streamingAttachmentBackend) Open(context.Context, string) (storage.Reader, error) {
	return storage.Reader{
		Body: &boundedZeroReadCloser{remaining: b.size, closed: &b.openClosed},
		Size: b.size,
	}, nil
}

func (b *streamingAttachmentBackend) Save(_ context.Context, _ string, r io.Reader) (storage.Object, error) {
	hasher := sha256.New()
	n, err := io.CopyBuffer(hasher, r, make([]byte, 64<<10))
	if err != nil {
		return storage.Object{}, err
	}
	b.savedSize = n
	b.savedHash = hex.EncodeToString(hasher.Sum(nil))
	return storage.Object{Path: "restored/object.lpk"}, nil
}

func (b *streamingAttachmentBackend) Delete(context.Context, string) error {
	b.deletes.Add(1)
	return nil
}

func (*streamingAttachmentBackend) PublicURL(string) string { return "" }

func TestMigrationAttachmentStreamsThirtyTwoMiB(t *testing.T) {
	const attachmentSize = int64(32 << 20)
	source := newMigrationTestDB(t)
	seedMigrationData(t, source)
	backend := &streamingAttachmentBackend{size: attachmentSize}
	resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) {
		return backend, nil
	})
	var archive bytes.Buffer
	manifest, err := NewExporter(source, resolver, "test").Export(t.Context(), &archive, Options{IncludeApps: true, IncludeFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Size != attachmentSize {
		t.Fatalf("manifest files = %+v", manifest.Files)
	}
	if !backend.openClosed.Load() {
		t.Fatal("export attachment reader was not closed")
	}
	target := newMigrationTestDB(t)
	result, err := NewImporter(target, resolver).Import(t.Context(), bytes.NewReader(archive.Bytes()), int64(archive.Len()), ImportOptions{
		Options: Options{IncludeApps: true, IncludeFiles: true},
		Mode:    ImportModeMerge,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Created == 0 {
		t.Fatalf("import result = %+v", result)
	}
	if backend.savedSize != attachmentSize || backend.savedHash != manifest.Files[0].SHA256 {
		t.Fatalf("saved size/hash = %d/%s, manifest = %d/%s", backend.savedSize, backend.savedHash, manifest.Files[0].Size, manifest.Files[0].SHA256)
	}
	if backend.deletes.Load() != 0 {
		t.Fatalf("unexpected deletes = %d", backend.deletes.Load())
	}
}

type attachmentReadCloser struct {
	data     []byte
	read     bool
	readErr  error
	closed   bool
	closeErr error
}

func (r *attachmentReadCloser) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		return copy(p, r.data), nil
	}
	if r.readErr != nil {
		return 0, r.readErr
	}
	return 0, io.EOF
}

func (r *attachmentReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}

type attachmentOpenBackend struct {
	reader storage.Reader
	err    error
}

func (b attachmentOpenBackend) Open(context.Context, string) (storage.Reader, error) {
	return b.reader, b.err
}
func (attachmentOpenBackend) Save(context.Context, string, io.Reader) (storage.Object, error) {
	return storage.Object{}, errors.New("read only")
}
func (attachmentOpenBackend) Delete(context.Context, string) error { return nil }
func (attachmentOpenBackend) PublicURL(string) string              { return "" }

func TestWriteFileEntriesHandlesPreEntryAndMidStreamFailures(t *testing.T) {
	t.Run("declared too large is warning and closes reader", func(t *testing.T) {
		reader := &attachmentReadCloser{}
		backend := attachmentOpenBackend{reader: storage.Reader{Body: reader, Size: maxJSONEntryBytes + 1}}
		resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) { return backend, nil })
		var archive bytes.Buffer
		zw := zip.NewWriter(&archive)
		files, warnings, err := writeFileEntries(t.Context(), zw, resolver, []fileRef{{Kind: "version", StorageKey: "local", StoragePath: "app.lpk"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 || len(warnings) != 1 || !reader.closed {
			t.Fatalf("files=%v warnings=%v closed=%v", files, warnings, reader.closed)
		}
	})

	t.Run("mid stream read failure aborts and closes reader", func(t *testing.T) {
		errRead := errors.New("forced attachment read failure")
		reader := &attachmentReadCloser{data: []byte("partial"), readErr: errRead}
		backend := attachmentOpenBackend{reader: storage.Reader{Body: reader, Size: 128}}
		resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) { return backend, nil })
		var archive bytes.Buffer
		zw := zip.NewWriter(&archive)
		_, _, err := writeFileEntries(t.Context(), zw, resolver, []fileRef{{Kind: "version", StorageKey: "local", StoragePath: "app.lpk"}})
		if !errors.Is(err, errRead) {
			t.Fatalf("writeFileEntries() error = %v", err)
		}
		if !reader.closed {
			t.Fatal("reader was not closed after stream failure")
		}
		_ = zw.Close()
	})
}

type mismatchImportBackend struct {
	deletes atomic.Int64
}

func (*mismatchImportBackend) Open(context.Context, string) (storage.Reader, error) {
	return storage.Reader{}, errors.New("not implemented")
}
func (*mismatchImportBackend) Save(_ context.Context, _ string, r io.Reader) (storage.Object, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return storage.Object{}, err
	}
	return storage.Object{Path: "restored/object.lpk"}, nil
}
func (b *mismatchImportBackend) Delete(context.Context, string) error {
	b.deletes.Add(1)
	return nil
}
func (*mismatchImportBackend) PublicURL(string) string { return "" }

func TestImportFilesDeletesSizeAndHashMismatches(t *testing.T) {
	payload := []byte("attachment payload")
	sum := sha256.Sum256(payload)
	for _, tt := range []struct {
		name string
		size int64
		hash string
		want string
	}{
		{name: "size", size: int64(len(payload) + 1), hash: hex.EncodeToString(sum[:]), want: "size mismatch"},
		{name: "hash", size: int64(len(payload)), hash: strings.Repeat("0", 64), want: "hash mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var archive bytes.Buffer
			zw := zip.NewWriter(&archive)
			entry, err := zw.Create("files/local/object.lpk")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(payload); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			zr, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
			if err != nil {
				t.Fatal(err)
			}
			backend := &mismatchImportBackend{}
			resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) { return backend, nil })
			_, _, err = importFiles(t.Context(), zr, resolver, Manifest{Files: []FileManifest{{
				Path: "files/local/object.lpk", StorageKey: "local", StoragePath: "object.lpk", Size: tt.size, SHA256: tt.hash,
			}}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("importFiles() error = %v, want %q", err, tt.want)
			}
			if backend.deletes.Load() != 1 {
				t.Fatalf("deletes = %d, want 1", backend.deletes.Load())
			}
		})
	}
}

func (b *oversizeCleanupBackend) Save(_ context.Context, _ string, r io.Reader) (storage.Object, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return storage.Object{}, err
	}
	b.cancel()
	return storage.Object{Path: "oversized/object.lpk"}, nil
}

func (b *oversizeCleanupBackend) Delete(ctx context.Context, _ string) error {
	b.deleteCalls++
	b.deleteCtxErr = append(b.deleteCtxErr, ctx.Err())
	return errors.New("forced cleanup failure")
}

func (*oversizeCleanupBackend) PublicURL(string) string { return "" }

func (*oversizeCleanupBackend) Open(context.Context, string) (storage.Reader, error) {
	return storage.Reader{}, errors.New("not implemented")
}

func TestImportOversizedAttachmentReportsFailedDetachedCleanup(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	entry, err := zw.CreateHeader(&zip.FileHeader{Name: "files/local/object.lpk", Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(entry, zeroReader{}, maxJSONEntryBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	backend := &oversizeCleanupBackend{cancel: cancel}
	resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) {
		return backend, nil
	})
	_, _, err = importFiles(ctx, zr, resolver, Manifest{Files: []FileManifest{{
		Path:        "files/local/object.lpk",
		StorageKey:  "local",
		StoragePath: "object.lpk",
	}}})
	if !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("importFiles() error = %v, want ErrTooLarge", err)
	}
	if !strings.Contains(err.Error(), "oversized/object.lpk") || !strings.Contains(err.Error(), "forced cleanup failure") {
		t.Fatalf("importFiles() error does not identify orphan cleanup: %v", err)
	}
	if backend.deleteCalls != 2 {
		t.Fatalf("delete calls = %d, want SaveFile cleanup plus import retry", backend.deleteCalls)
	}
	for i, ctxErr := range backend.deleteCtxErr {
		if ctxErr != nil {
			t.Fatalf("delete context %d was canceled: %v", i, ctxErr)
		}
	}
}

func newMigrationTestDB(t testing.TB) *ent.Client {
	t.Helper()
	db, err := ent.Open(dialect.SQLite, "file:"+filepath.Join(t.TempDir(), "store.db")+"?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open ent: %v", err)
	}
	if err := db.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedMigrationData(t testing.TB, db *ent.Client) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	owner := db.User.Create().
		SetUsername("owner").
		SetNickname("Owner").
		SetPasswordHash("hash-owner").
		SetRole(user.RoleSITE_ADMIN).
		SetEmailVerified(true).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	cat := db.Category.Create().SetName("Tools").SetNameI18n(`{"zh-CN":"工具","en":"Tools"}`).SetSlug("tools").SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
	tagRecord := db.Tag.Create().SetName("Infra").SetNameI18n(`{"zh-CN":"基础设施","en":"Infra"}`).SetSlug("infra").SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
	group := db.UserGroup.Create().SetOwnerID(owner.ID).SetName("Team").SetSlug("team").SetCode("A1B2C3").SetCodeUpdatedAt(now).SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
	db.GroupMember.Create().SetGroupID(group.ID).SetUserID(owner.ID).SetCreatedAt(now).SaveX(ctx)
	appRecord := db.App.Create().
		SetOwnerID(owner.ID).
		SetCategoryID(cat.ID).
		SetPackageID("cloud.lazycat.example").
		SetName("Example").
		SetNameI18nJSON(`{"zh-CN":"示例","en":"Example"}`).
		SetSlug("example").
		SetAuthor("LazyCat Community").
		SetHomepage("https://example.com/app").
		SetLicense("MIT").
		SetMinOsVersion("1.3.0").
		SetStatus(apppkg.StatusAPPROVED).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	version := db.AppVersion.Create().SetAppID(appRecord.ID).SetUploaderID(owner.ID).SetVersion("1.0.0").SetStatus(versionpkg.StatusAPPROVED).SetSourceType(versionpkg.SourceTypeLOCAL).SetDownloadURL("/files/example.lpk").SetStoragePath("2026/01/01/example.lpk").SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
	db.AppDownload.Create().SetAppID(appRecord.ID).SetVersion(version.Version).SetCreatedAt(now).SaveX(ctx)
	db.AppTag.Create().SetAppID(appRecord.ID).SetTagID(tagRecord.ID).SetCreatedAt(now).SaveX(ctx)
	db.AppVisibility.Create().SetAppID(appRecord.ID).SetGroupID(group.ID).SetCreatedAt(now).SaveX(ctx)
	db.AppVote.Create().SetAppID(appRecord.ID).SetUserID(owner.ID).SetValue(1).SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
	db.SiteSetting.Create().SetKey("site_title").SetValue("Migrated").SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
}

func testAppsMigrationArchive(t *testing.T, data AppsData) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	writeTestJSON(t, zw, manifestName, Manifest{
		FormatVersion: FormatVersion,
		ServerVersion: "legacy-test",
		CreatedAt:     time.Now().UTC(),
		Modules:       []Module{ModuleApps},
	})
	writeTestJSON(t, zw, "data/apps.json", data)
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return archive.Bytes()
}

func writeTestJSON(t *testing.T, zw *zip.Writer, name string, value any) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func assetURL(id int) string {
	return serverAssetURLPrefix + "/" + strconv.Itoa(id)
}
