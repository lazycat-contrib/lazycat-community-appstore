package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("SITE_MAX_LPK_SIZE", "")
	t.Setenv("SITE_MAX_VERSIONS", "")

	cfg := Load()

	if cfg.DBDriver != "sqlite3" {
		t.Fatalf("DBDriver = %q, want sqlite3", cfg.DBDriver)
	}
	if cfg.DBDSN != "./data/store.db" {
		t.Fatalf("DBDSN = %q, want ./data/store.db", cfg.DBDSN)
	}
	if cfg.MaxLPKSize != 524288000 {
		t.Fatalf("MaxLPKSize = %d, want 524288000", cfg.MaxLPKSize)
	}
	if cfg.MaxVersions != 10 {
		t.Fatalf("MaxVersions = %d, want 10", cfg.MaxVersions)
	}
}

func TestLoadSupportsConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsn    string
	}{
		{name: "sqlite", driver: "sqlite3", dsn: "./tmp/store.db"},
		{name: "postgres", driver: "postgres", dsn: "postgres://user:pass@localhost/store?sslmode=disable"},
		{name: "mysql", driver: "mysql", dsn: "user:pass@tcp(localhost:3306)/store?parseTime=true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_DRIVER", tt.driver)
			t.Setenv("DB_DSN", tt.dsn)

			cfg := Load()

			if cfg.DBDriver != tt.driver {
				t.Fatalf("DBDriver = %q, want %q", cfg.DBDriver, tt.driver)
			}
			if cfg.DBDSN != tt.dsn {
				t.Fatalf("DBDSN = %q, want %q", cfg.DBDSN, tt.dsn)
			}
		})
	}
}
