package clientserver

import (
	"context"
	"strings"
	"testing"

	"lazycat.community/appstore/ent"
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
