package clientserver

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMigrateSchemaV3InvalidatesLegacySourceUpdateTimes(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	t.Cleanup(func() { _ = client.Close() })

	source, err := client.ClientSource.Create().
		SetUserID("alice").
		SetName("Community").
		SetURL("https://store.example/source/v2/index.json").
		SetLastEtag(`"legacy-etag"`).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	legacyUpdatedAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	if _, err := client.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetPackageID("cloud.lazycat.app.notes").
		SetName("Notes").
		SetSlug("notes").
		SetUpdatedAt(legacyUpdatedAt).
		Save(ctx); err != nil {
		t.Fatal(err)
	}
	if err := setSystemClientSetting(ctx, client, settingClientSchemaVersion, "2"); err != nil {
		t.Fatal(err)
	}

	if err := migrateSchema(ctx, client); err != nil {
		t.Fatal(err)
	}

	app := client.ClientSourceApp.Query().OnlyX(ctx)
	if want := time.Unix(0, 0).UTC(); !app.UpdatedAt.Equal(want) {
		t.Fatalf("updated_at = %s, want %s", app.UpdatedAt, want)
	}
	migratedSource := client.ClientSource.GetX(ctx, source.ID)
	if migratedSource.LastEtag != "" {
		t.Fatalf("last_etag = %q, want empty", migratedSource.LastEtag)
	}
	if got := storedClientSchemaVersion(ctx, client); got != currentClientSchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, currentClientSchemaVersion)
	}
}

func TestMigrateSchemaV4InvalidatesSourceETagsWithoutChangingAppUpdateTimes(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	t.Cleanup(func() { _ = client.Close() })

	source := client.ClientSource.Create().
		SetUserID("alice").
		SetName("Community").
		SetURL("https://store.example/source/v2/index.json").
		SetLastEtag(`"download-count-missing"`).
		SaveX(ctx)
	updatedAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	client.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetPackageID("cloud.lazycat.app.notes").
		SetName("Notes").
		SetSlug("notes").
		SetUpdatedAt(updatedAt).
		SaveX(ctx)
	if err := setSystemClientSetting(ctx, client, settingClientSchemaVersion, "3"); err != nil {
		t.Fatal(err)
	}

	if err := migrateSchema(ctx, client); err != nil {
		t.Fatal(err)
	}

	if got := client.ClientSource.GetX(ctx, source.ID).LastEtag; got != "" {
		t.Fatalf("last_etag = %q, want empty", got)
	}
	if got := client.ClientSourceApp.Query().OnlyX(ctx).UpdatedAt; !got.Equal(updatedAt) {
		t.Fatalf("updated_at = %s, want %s", got, updatedAt)
	}
	if got := storedClientSchemaVersion(ctx, client); got != currentClientSchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, currentClientSchemaVersion)
	}
}

func TestMigrateSchemaV5InvalidatesSourceETagsForWishWallCapability(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	t.Cleanup(func() { _ = client.Close() })
	source := client.ClientSource.Create().SetUserID("alice").SetName("Community").
		SetURL("https://store.example/source/v2/index.json").SetLastEtag(`"wish-wall-unknown"`).SaveX(ctx)
	if err := setSystemClientSetting(ctx, client, settingClientSchemaVersion, "4"); err != nil {
		t.Fatal(err)
	}
	if err := migrateSchema(ctx, client); err != nil {
		t.Fatal(err)
	}
	if got := client.ClientSource.GetX(ctx, source.ID).LastEtag; got != "" {
		t.Fatalf("last_etag = %q, want empty", got)
	}
	if got := storedClientSchemaVersion(ctx, client); got != currentClientSchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, currentClientSchemaVersion)
	}
}

func TestMigrateSchemaV6InvalidatesSourceETagsForSiteIcons(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	t.Cleanup(func() { _ = client.Close() })
	source := client.ClientSource.Create().SetUserID("alice").SetName("Community").
		SetURL("https://store.example/source/v2/index.json").SetLastEtag(`"site-icon-unknown"`).SaveX(ctx)
	if err := setSystemClientSetting(ctx, client, settingClientSchemaVersion, "5"); err != nil {
		t.Fatal(err)
	}
	if err := migrateSchema(ctx, client); err != nil {
		t.Fatal(err)
	}
	if got := client.ClientSource.GetX(ctx, source.ID).LastEtag; got != "" {
		t.Fatalf("last_etag = %q, want empty", got)
	}
	if got := storedClientSchemaVersion(ctx, client); got != currentClientSchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, currentClientSchemaVersion)
	}
}

func TestMigrateSchemaCleansUnlinkedClientAssets(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	t.Cleanup(func() { _ = client.Close() })
	client.ClientAsset.Create().
		SetSha256(strings.Repeat("a", 64)).
		SetMediaType("image/png").
		SetSize(1).
		SetData([]byte{0}).
		SaveX(ctx)
	if err := setSystemClientSetting(ctx, client, settingClientSchemaVersion, strconv.Itoa(currentClientSchemaVersion)); err != nil {
		t.Fatal(err)
	}
	if err := migrateSchema(ctx, client); err != nil {
		t.Fatal(err)
	}
	if count := client.ClientAsset.Query().CountX(ctx); count != 0 {
		t.Fatalf("unlinked client assets = %d, want 0", count)
	}
}
