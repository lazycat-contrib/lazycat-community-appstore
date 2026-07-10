package clientserver

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidateRejectsDefaultSessionSecretOnPublicAddress(t *testing.T) {
	cfg := LoadConfig()
	cfg.Addr = "0.0.0.0:8090"
	cfg.SessionSecret = defaultClientSessionSecret

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "CLIENT_SESSION_SECRET") {
		t.Fatalf("Validate() error = %v, want CLIENT_SESSION_SECRET error", err)
	}
}

func TestLoadConfigSetsBoundedHTTPRuntimeDefaults(t *testing.T) {
	cfg := LoadConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second || cfg.ReadTimeout != 30*time.Second || cfg.WriteTimeout != 60*time.Second {
		t.Fatalf("request timeouts = %v/%v/%v", cfg.ReadHeaderTimeout, cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 2*time.Minute || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("lifecycle timeouts = %v/%v", cfg.IdleTimeout, cfg.ShutdownTimeout)
	}
	if cfg.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", cfg.MaxHeaderBytes, 1<<20)
	}
	if cfg.DBMaxOpenConns != 1 || cfg.DBMaxIdleConns != 1 || cfg.DBConnMaxLifetime != 30*time.Minute || cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("database pool defaults = %d/%d/%v/%v", cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime, cfg.DBConnMaxIdleTime)
	}
}

func TestLoadConfigParsesDatabasePoolConfiguration(t *testing.T) {
	t.Setenv("CLIENT_DB_MAX_OPEN_CONNS", "4")
	t.Setenv("CLIENT_DB_MAX_IDLE_CONNS", "2")
	t.Setenv("CLIENT_DB_CONN_MAX_LIFETIME", "45m")
	t.Setenv("CLIENT_DB_CONN_MAX_IDLE_TIME", "7m")
	cfg := LoadConfig()
	if cfg.DBMaxOpenConns != 4 || cfg.DBMaxIdleConns != 2 || cfg.DBConnMaxLifetime != 45*time.Minute || cfg.DBConnMaxIdleTime != 7*time.Minute {
		t.Fatalf("database pool = %+v", cfg)
	}
}

func TestClientConfigValidateRejectsInvalidDatabasePool(t *testing.T) {
	tests := []func(*Config){
		func(cfg *Config) { cfg.DBMaxOpenConns = 0 },
		func(cfg *Config) { cfg.DBMaxIdleConns = -1 },
		func(cfg *Config) { cfg.DBMaxIdleConns = cfg.DBMaxOpenConns + 1 },
		func(cfg *Config) { cfg.DBConnMaxLifetime = -1 },
		func(cfg *Config) { cfg.DBConnMaxIdleTime = -1 },
	}
	for index, mutate := range tests {
		cfg := LoadConfig()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("case %d Validate() error = nil", index)
		}
	}
}
