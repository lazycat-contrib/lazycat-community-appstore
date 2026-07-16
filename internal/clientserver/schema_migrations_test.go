package clientserver

import (
	"context"
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
	if got := storedClientSchemaVersion(ctx, client); got != 3 {
		t.Fatalf("schema version = %d, want 3", got)
	}
}
