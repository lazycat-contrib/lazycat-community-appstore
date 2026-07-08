package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("SITE_MAX_LPK_SIZE", "")
	t.Setenv("SITE_MAX_VERSIONS", "")
	t.Setenv("SOURCE_V1_ENABLED", "")
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")

	cfg := Load()

	if cfg.DBDriver != "sqlite3" {
		t.Fatalf("DBDriver = %q, want sqlite3", cfg.DBDriver)
	}
	if cfg.DBDSN != DefaultSQLiteDSN {
		t.Fatalf("DBDSN = %q, want %q", cfg.DBDSN, DefaultSQLiteDSN)
	}
	if cfg.MaxLPKSize != 524288000 {
		t.Fatalf("MaxLPKSize = %d, want 524288000", cfg.MaxLPKSize)
	}
	if cfg.MaxVersions != 10 {
		t.Fatalf("MaxVersions = %d, want 10", cfg.MaxVersions)
	}
	if !cfg.SourceV1Enabled {
		t.Fatal("SourceV1Enabled = false, want true by default")
	}
	if cfg.AdminBootstrap {
		t.Fatal("AdminBootstrap = true, want false when ADMIN_USERNAME and ADMIN_PASSWORD are unset")
	}
}

func TestLoadEnablesAdminBootstrapWhenAdminEnvProvided(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_PASSWORD", "secret-password")

	cfg := Load()

	if !cfg.AdminBootstrap {
		t.Fatal("AdminBootstrap = false, want true")
	}
	if cfg.AdminUsername != "root" || cfg.AdminPassword != "secret-password" {
		t.Fatalf("admin credentials = %q/%q", cfg.AdminUsername, cfg.AdminPassword)
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

func TestLoadParsesTrustLazyCatClientChat(t *testing.T) {
	tests := []struct {
		name     string
		comments string
		chat     string
		want     bool
	}{
		{name: "explicit true", comments: "false", chat: "true", want: true},
		{name: "explicit false", comments: "true", chat: "false", want: false},
		{name: "fallback to comments trust", comments: "on", chat: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TRUST_LAZYCAT_CLIENT_COMMENTS", tt.comments)
			t.Setenv("TRUST_LAZYCAT_CLIENT_CHAT", tt.chat)

			cfg := Load()

			if cfg.TrustLazyCatClientChat != tt.want {
				t.Fatalf("TrustLazyCatClientChat = %v, want %v", cfg.TrustLazyCatClientChat, tt.want)
			}
		})
	}
}

func TestLoadParsesSourceV1Enabled(t *testing.T) {
	t.Setenv("SOURCE_V1_ENABLED", "false")

	cfg := Load()

	if cfg.SourceV1Enabled {
		t.Fatal("SourceV1Enabled = true, want false")
	}
}
