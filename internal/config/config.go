package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultSQLiteDSN = "file:./data/store.db?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"

type Config struct {
	Addr                       string
	BaseURL                    string
	ClientOrigins              []string
	DBDriver                   string
	DBDSN                      string
	StorageBackend             string
	LocalStoragePath           string
	WebDAVURL                  string
	WebDAVUser                 string
	WebDAVPass                 string
	WebDAVPublicURL            string
	S3Endpoint                 string
	S3Bucket                   string
	S3AccessKey                string
	S3SecretKey                string
	S3UseSSL                   bool
	S3PublicURL                string
	MaxLPKSize                 int64
	MaxVersions                int
	RequireEmailVerify         bool
	SourcePassword             string
	SourcePasswordRotation     int
	GitHubDownloadMirrors      string
	GitHubRawMirrors           string
	SMTPHost                   string
	SMTPPort                   int
	SMTPUser                   string
	SMTPPass                   string
	SMTPFrom                   string
	TrustLazyCatClientComments bool
	SitePublicURL              string
	AdminUsername              string
	AdminPassword              string
	AdminBootstrap             bool
	SessionSecret              string
	ReadTimeout                time.Duration
	WriteTimeout               time.Duration
}

func Load() Config {
	return Config{
		Addr:                       env("APP_ADDR", ":8080"),
		BaseURL:                    strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		ClientOrigins:              splitEnv("CLIENT_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
		DBDriver:                   normalizeDriver(env("DB_DRIVER", "sqlite3")),
		DBDSN:                      env("DB_DSN", DefaultSQLiteDSN),
		StorageBackend:             env("STORAGE_BACKEND", "local"),
		LocalStoragePath:           env("LOCAL_STORAGE_PATH", "./data/files"),
		WebDAVURL:                  os.Getenv("WEBDAV_URL"),
		WebDAVUser:                 os.Getenv("WEBDAV_USER"),
		WebDAVPass:                 os.Getenv("WEBDAV_PASS"),
		WebDAVPublicURL:            os.Getenv("WEBDAV_PUBLIC_URL"),
		S3Endpoint:                 os.Getenv("S3_ENDPOINT"),
		S3Bucket:                   os.Getenv("S3_BUCKET"),
		S3AccessKey:                os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:                os.Getenv("S3_SECRET_KEY"),
		S3UseSSL:                   envBool("S3_USE_SSL", true),
		S3PublicURL:                os.Getenv("S3_PUBLIC_URL"),
		MaxLPKSize:                 envInt64("SITE_MAX_LPK_SIZE", 524288000),
		MaxVersions:                envInt("SITE_MAX_VERSIONS", 10),
		RequireEmailVerify:         envBool("REQUIRE_EMAIL_VERIFY", false),
		SourcePassword:             os.Getenv("SOURCE_PASSWORD"),
		SourcePasswordRotation:     envInt("SOURCE_PASSWORD_ROTATION", 0),
		GitHubDownloadMirrors:      strings.TrimSpace(os.Getenv("GITHUB_DOWNLOAD_MIRRORS")),
		GitHubRawMirrors:           strings.TrimSpace(os.Getenv("GITHUB_RAW_MIRRORS")),
		SMTPHost:                   os.Getenv("SMTP_HOST"),
		SMTPPort:                   envInt("SMTP_PORT", 25),
		SMTPUser:                   os.Getenv("SMTP_USER"),
		SMTPPass:                   os.Getenv("SMTP_PASS"),
		SMTPFrom:                   os.Getenv("SMTP_FROM"),
		TrustLazyCatClientComments: envBool("TRUST_LAZYCAT_CLIENT_COMMENTS", false),
		SitePublicURL:              strings.TrimRight(env("SITE_PUBLIC_URL", env("BASE_URL", "http://localhost:8080")), "/"),
		AdminUsername:              env("ADMIN_USERNAME", "admin"),
		AdminPassword:              env("ADMIN_PASSWORD", "changeme"),
		AdminBootstrap:             envProvided("ADMIN_USERNAME") || envProvided("ADMIN_PASSWORD"),
		SessionSecret:              env("SESSION_SECRET", "dev-session-secret-change-me"),
		ReadTimeout:                10 * time.Second,
		WriteTimeout:               60 * time.Second,
	}
}

func splitEnv(key, fallback string) []string {
	value := env(key, fallback)
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "sqlite", "sqlite3":
		return "sqlite3"
	case "postgres", "postgresql":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return driver
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envProvided(key string) bool {
	return strings.TrimSpace(os.Getenv(key)) != ""
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

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
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
