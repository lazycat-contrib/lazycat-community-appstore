package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("SITE_MAX_LPK_SIZE", "")
	t.Setenv("SITE_MAX_VERSIONS", "")
	t.Setenv("SOURCE_V1_ENABLED", "")
	t.Setenv("SOURCE_CACHE_PATH", "")
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
	if cfg.SourceCachePath != "./data/source-cache" {
		t.Fatalf("SourceCachePath = %q, want ./data/source-cache", cfg.SourceCachePath)
	}
	if cfg.AdminBootstrap {
		t.Fatal("AdminBootstrap = true, want false when ADMIN_USERNAME and ADMIN_PASSWORD are unset")
	}
	if cfg.DBMaxOpenConns != 1 || cfg.DBMaxIdleConns != 1 || cfg.DBConnMaxLifetime != 30*time.Minute || cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("database pool defaults = %d/%d/%v/%v", cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime, cfg.DBConnMaxIdleTime)
	}
}

func TestLoadParsesDatabasePoolConfiguration(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_MAX_OPEN_CONNS", "24")
	t.Setenv("DB_MAX_IDLE_CONNS", "12")
	t.Setenv("DB_CONN_MAX_LIFETIME", "45m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "7m")
	cfg := Load()
	if cfg.DBMaxOpenConns != 24 || cfg.DBMaxIdleConns != 12 || cfg.DBConnMaxLifetime != 45*time.Minute || cfg.DBConnMaxIdleTime != 7*time.Minute {
		t.Fatalf("database pool = %+v", cfg)
	}
}

func TestConfigValidateRejectsInvalidDatabasePool(t *testing.T) {
	tests := []func(*Config){
		func(cfg *Config) { cfg.DBMaxOpenConns = 0 },
		func(cfg *Config) { cfg.DBMaxIdleConns = -1 },
		func(cfg *Config) { cfg.DBMaxIdleConns = cfg.DBMaxOpenConns + 1 },
		func(cfg *Config) { cfg.DBConnMaxLifetime = -1 },
		func(cfg *Config) { cfg.DBConnMaxIdleTime = -1 },
	}
	for index, mutate := range tests {
		cfg := Load()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("case %d Validate() error = nil", index)
		}
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

func TestLoadParsesTrustLazyCatClientInstall(t *testing.T) {
	t.Setenv("TRUST_LAZYCAT_CLIENT_INSTALL", "true")

	cfg := Load()

	if !cfg.TrustLazyCatClientInstall {
		t.Fatal("TrustLazyCatClientInstall = false, want true")
	}
}

func TestLoadParsesSourceV1Enabled(t *testing.T) {
	t.Setenv("SOURCE_V1_ENABLED", "false")

	cfg := Load()

	if cfg.SourceV1Enabled {
		t.Fatal("SourceV1Enabled = true, want false")
	}
}

func TestLoadParsesSourceCachePath(t *testing.T) {
	t.Setenv("SOURCE_CACHE_PATH", "/var/cache/appstore/source")

	cfg := Load()

	if cfg.SourceCachePath != "/var/cache/appstore/source" {
		t.Fatalf("SourceCachePath = %q", cfg.SourceCachePath)
	}
}

func TestConfigValidateRejectsDefaultSessionSecretOnPublicURL(t *testing.T) {
	cfg := Load()
	cfg.BaseURL = "https://store.example.com"
	cfg.SitePublicURL = "https://store.example.com"
	cfg.SessionSecret = defaultSessionSecret

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("Validate() error = %v, want SESSION_SECRET error", err)
	}
}

func TestConfigValidateAllowsDefaultSessionSecretForLoopbackDevelopment(t *testing.T) {
	cfg := Load()
	cfg.BaseURL = "http://localhost:8080"
	cfg.SitePublicURL = "http://127.0.0.1:8080"
	cfg.SessionSecret = defaultSessionSecret

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadSetsBoundedHTTPRuntimeDefaults(t *testing.T) {
	cfg := Load()

	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", cfg.ReadHeaderTimeout)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %v, want 10s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 60*time.Second {
		t.Fatalf("WriteTimeout = %v, want 60s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout = %v, want 2m", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", cfg.MaxHeaderBytes, 1<<20)
	}
}

func TestConfigValidateRejectsInvalidHTTPRuntimeLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "read header timeout", mutate: func(cfg *Config) { cfg.ReadHeaderTimeout = 0 }},
		{name: "read timeout", mutate: func(cfg *Config) { cfg.ReadTimeout = 0 }},
		{name: "write timeout", mutate: func(cfg *Config) { cfg.WriteTimeout = 0 }},
		{name: "idle timeout", mutate: func(cfg *Config) { cfg.IdleTimeout = 0 }},
		{name: "shutdown timeout", mutate: func(cfg *Config) { cfg.ShutdownTimeout = 0 }},
		{name: "header bytes", mutate: func(cfg *Config) { cfg.MaxHeaderBytes = 1024 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Load()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid runtime limit error")
			}
		})
	}
}
