package migration

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"lazycat.community/appstore/ent"
	apppkg "lazycat.community/appstore/ent/app"
	versionpkg "lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/asset"
	"lazycat.community/appstore/ent/assetlink"
	"lazycat.community/appstore/ent/user"

	_ "github.com/lib-x/entsqlite"
)

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
	if manifest.Counts["users"] != 1 || manifest.Counts["apps"] != 1 || manifest.Counts["appVisibilities"] != 1 {
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
	if target.AppVersion.Query().Where(versionpkg.AppIDEQ(record.ID), versionpkg.VersionEQ("1.0.0")).CountX(ctx) != 1 {
		t.Fatal("app version was not imported")
	}
	if target.AppVisibility.Query().Where(appvisibility.AppIDEQ(record.ID)).CountX(ctx) != 1 {
		t.Fatal("app visibility was not imported")
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

func newMigrationTestDB(t *testing.T) *ent.Client {
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

func seedMigrationData(t *testing.T, db *ent.Client) {
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
		SetStatus(apppkg.StatusAPPROVED).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SaveX(ctx)
	db.AppVersion.Create().SetAppID(appRecord.ID).SetUploaderID(owner.ID).SetVersion("1.0.0").SetStatus(versionpkg.StatusAPPROVED).SetSourceType(versionpkg.SourceTypeLOCAL).SetDownloadURL("/files/example.lpk").SetStoragePath("2026/01/01/example.lpk").SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
	db.AppTag.Create().SetAppID(appRecord.ID).SetTagID(tagRecord.ID).SetCreatedAt(now).SaveX(ctx)
	db.AppVisibility.Create().SetAppID(appRecord.ID).SetGroupID(group.ID).SetCreatedAt(now).SaveX(ctx)
	db.SiteSetting.Create().SetKey("site_title").SetValue("Migrated").SetCreatedAt(now).SetUpdatedAt(now).SaveX(ctx)
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
