package clientserver

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultClientSessionSecret = "dev-client-session-secret-change-me"

type Config struct {
	Addr              string
	DBDSN             string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
	DefaultSourceURL  string
	DefaultSourceName string
	SyncTimeout       time.Duration
	SessionSecret     string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
	OIDCEnabled       bool
	OIDCIssuerURL     string
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCRedirectURL   string
	OIDCScopes        []string
}

func LoadConfig() Config {
	oidcIssuer := strings.TrimSpace(os.Getenv("CLIENT_OIDC_ISSUER_URL"))
	oidcClientID := strings.TrimSpace(os.Getenv("CLIENT_OIDC_CLIENT_ID"))
	oidcClientSecret := strings.TrimSpace(os.Getenv("CLIENT_OIDC_CLIENT_SECRET"))
	oidcConfigured := oidcIssuer != "" && oidcClientID != "" && oidcClientSecret != ""
	return Config{
		Addr:              env("CLIENT_ADDR", "127.0.0.1:8090"),
		DBDSN:             env("CLIENT_DB_DSN", "./data/client.db"),
		DBMaxOpenConns:    envInt("CLIENT_DB_MAX_OPEN_CONNS", 1),
		DBMaxIdleConns:    envInt("CLIENT_DB_MAX_IDLE_CONNS", 1),
		DBConnMaxLifetime: envDuration("CLIENT_DB_CONN_MAX_LIFETIME", 30*time.Minute),
		DBConnMaxIdleTime: envDuration("CLIENT_DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		DefaultSourceURL:  strings.TrimSpace(os.Getenv("CLIENT_DEFAULT_SOURCE_URL")),
		DefaultSourceName: env("CLIENT_DEFAULT_SOURCE_NAME", "喵喵私有商店"),
		SyncTimeout:       20 * time.Second,
		SessionSecret:     env("CLIENT_SESSION_SECRET", defaultClientSessionSecret),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ShutdownTimeout:   10 * time.Second,
		MaxHeaderBytes:    1 << 20,
		OIDCEnabled:       envBool("CLIENT_OIDC_ENABLED", oidcConfigured),
		OIDCIssuerURL:     oidcIssuer,
		OIDCClientID:      oidcClientID,
		OIDCClientSecret:  oidcClientSecret,
		OIDCRedirectURL:   defaultOIDCRedirectURL(),
		OIDCScopes:        envCSV("CLIENT_OIDC_SCOPES", []string{"openid", "profile", "email"}),
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.SessionSecret) == "" {
		return errors.New("CLIENT_SESSION_SECRET is required")
	}
	host, _, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return fmt.Errorf("parse CLIENT_ADDR: %w", err)
	}
	ip := net.ParseIP(host)
	loopback := strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
	if c.SessionSecret == defaultClientSessionSecret && !loopback {
		return errors.New("CLIENT_SESSION_SECRET must be changed for non-loopback deployments")
	}
	if c.ReadHeaderTimeout <= 0 || c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return errors.New("HTTP timeout values must be positive")
	}
	if c.MaxHeaderBytes < 64<<10 {
		return errors.New("MaxHeaderBytes must be at least 65536")
	}
	if c.DBMaxOpenConns <= 0 {
		return errors.New("DBMaxOpenConns must be positive")
	}
	if c.DBMaxIdleConns < 0 || c.DBMaxIdleConns > c.DBMaxOpenConns {
		return errors.New("DBMaxIdleConns must be between zero and DBMaxOpenConns")
	}
	if c.DBConnMaxLifetime < 0 || c.DBConnMaxIdleTime < 0 {
		return errors.New("database connection lifetimes must not be negative")
	}
	return nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envCSV(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func defaultOIDCRedirectURL() string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv("CLIENT_OIDC_REDIRECT_URL")), "/"); value != "" {
		return value
	}
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv("LAZYCAT_PUBLIC_URL")), "/"); value != "" {
		return value + "/auth/oidc/callback"
	}
	if domain := strings.TrimSpace(os.Getenv("LAZYCAT_APP_DOMAIN")); domain != "" {
		return "https://" + strings.TrimRight(domain, "/") + "/auth/oidc/callback"
	}
	return ""
}
