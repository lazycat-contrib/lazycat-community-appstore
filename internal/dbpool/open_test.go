package dbpool

import (
	"testing"
	"time"

	"lazycat.community/appstore/ent"

	_ "github.com/lib-x/entsqlite"
)

func TestOpenConfiguresSQLitePool(t *testing.T) {
	cfg := Config{
		Driver:      "sqlite3",
		DSN:         "file:" + t.TempDir() + "/pool.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)",
		MaxOpen:     1,
		MaxIdle:     1,
		MaxLifetime: 30 * time.Minute,
		MaxIdleTime: 5 * time.Minute,
	}
	db, driver, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	if got := db.Stats().MaxOpenConnections; got != cfg.MaxOpen {
		t.Fatalf("MaxOpenConnections = %d, want %d", got, cfg.MaxOpen)
	}
	if err := client.Schema.Create(t.Context()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
}

func TestOpenRejectsInvalidConfiguration(t *testing.T) {
	tests := []Config{
		{Driver: "sqlite3", DSN: ":memory:", MaxOpen: 0, MaxIdle: 0},
		{Driver: "sqlite3", DSN: ":memory:", MaxOpen: 1, MaxIdle: -1},
		{Driver: "sqlite3", DSN: ":memory:", MaxOpen: 1, MaxIdle: 2},
		{Driver: "sqlite3", DSN: ":memory:", MaxOpen: 1, MaxIdle: 1, MaxLifetime: -1},
		{Driver: "unsupported", DSN: "ignored", MaxOpen: 1, MaxIdle: 1},
	}
	for _, cfg := range tests {
		if db, _, err := Open(cfg); err == nil {
			_ = db.Close()
			t.Fatalf("Open(%+v) error = nil", cfg)
		}
	}
}
