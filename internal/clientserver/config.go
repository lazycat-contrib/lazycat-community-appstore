package clientserver

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Addr              string
	DBDSN             string
	DefaultSourceURL  string
	DefaultSourceName string
	SyncTimeout       time.Duration
	SessionSecret     string
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
		DefaultSourceURL:  strings.TrimSpace(os.Getenv("CLIENT_DEFAULT_SOURCE_URL")),
		DefaultSourceName: env("CLIENT_DEFAULT_SOURCE_NAME", "喵喵私有商店"),
		SyncTimeout:       20 * time.Second,
		SessionSecret:     env("CLIENT_SESSION_SECRET", "dev-client-session-secret-change-me"),
		OIDCEnabled:       envBool("CLIENT_OIDC_ENABLED", oidcConfigured),
		OIDCIssuerURL:     oidcIssuer,
		OIDCClientID:      oidcClientID,
		OIDCClientSecret:  oidcClientSecret,
		OIDCRedirectURL:   defaultOIDCRedirectURL(),
		OIDCScopes:        envCSV("CLIENT_OIDC_SCOPES", []string{"openid", "profile", "email"}),
	}
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
