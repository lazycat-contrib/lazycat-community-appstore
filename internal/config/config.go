package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultSQLiteDSN     = "file:./data/store.db?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"
	defaultSessionSecret = "dev-session-secret-change-me"
)

type Config struct {
	Addr                       string
	BaseURL                    string
	ClientOrigins              []string
	DBDriver                   string
	DBDSN                      string
	DBMaxOpenConns             int
	DBMaxIdleConns             int
	DBConnMaxLifetime          time.Duration
	DBConnMaxIdleTime          time.Duration
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
	SourceV1Enabled            bool
	GitHubDownloadMirrors      string
	GitHubRawMirrors           string
	SMTPHost                   string
	SMTPPort                   int
	SMTPUser                   string
	SMTPPass                   string
	SMTPFrom                   string
	SMTPFromName               string
	TrustLazyCatClientComments bool
	TrustLazyCatClientChat     bool
	SitePublicURL              string
	AdminUsername              string
	AdminPassword              string
	AdminBootstrap             bool
	SessionSecret              string
	ReadHeaderTimeout          time.Duration
	ReadTimeout                time.Duration
	WriteTimeout               time.Duration
	IdleTimeout                time.Duration
	ShutdownTimeout            time.Duration
	MaxHeaderBytes             int
}

func Load() Config {
	driver := normalizeDriver(env("DB_DRIVER", "sqlite3"))
	defaultMaxOpen, defaultMaxIdle := defaultDBPool(driver)
	return Config{
		Addr:                       env("APP_ADDR", ":8080"),
		BaseURL:                    strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		ClientOrigins:              splitEnv("CLIENT_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
		DBDriver:                   driver,
		DBDSN:                      env("DB_DSN", DefaultSQLiteDSN),
		DBMaxOpenConns:             envInt("DB_MAX_OPEN_CONNS", defaultMaxOpen),
		DBMaxIdleConns:             envInt("DB_MAX_IDLE_CONNS", defaultMaxIdle),
		DBConnMaxLifetime:          envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		DBConnMaxIdleTime:          envDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
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
		SourceV1Enabled:            envBool("SOURCE_V1_ENABLED", true),
		GitHubDownloadMirrors:      strings.TrimSpace(os.Getenv("GITHUB_DOWNLOAD_MIRRORS")),
		GitHubRawMirrors:           strings.TrimSpace(os.Getenv("GITHUB_RAW_MIRRORS")),
		SMTPHost:                   os.Getenv("SMTP_HOST"),
		SMTPPort:                   envInt("SMTP_PORT", 25),
		SMTPUser:                   os.Getenv("SMTP_USER"),
		SMTPPass:                   os.Getenv("SMTP_PASS"),
		SMTPFrom:                   os.Getenv("SMTP_FROM"),
		SMTPFromName:               os.Getenv("SMTP_FROM_NAME"),
		TrustLazyCatClientComments: envBool("TRUST_LAZYCAT_CLIENT_COMMENTS", false),
		TrustLazyCatClientChat:     envBool("TRUST_LAZYCAT_CLIENT_CHAT", envBool("TRUST_LAZYCAT_CLIENT_COMMENTS", false)),
		SitePublicURL:              strings.TrimRight(env("SITE_PUBLIC_URL", env("BASE_URL", "http://localhost:8080")), "/"),
		AdminUsername:              env("ADMIN_USERNAME", "admin"),
		AdminPassword:              env("ADMIN_PASSWORD", "changeme"),
		AdminBootstrap:             envProvided("ADMIN_USERNAME") || envProvided("ADMIN_PASSWORD"),
		SessionSecret:              env("SESSION_SECRET", defaultSessionSecret),
		ReadHeaderTimeout:          5 * time.Second,
		ReadTimeout:                10 * time.Second,
		WriteTimeout:               60 * time.Second,
		IdleTimeout:                2 * time.Minute,
		ShutdownTimeout:            10 * time.Second,
		MaxHeaderBytes:             1 << 20,
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.SessionSecret) == "" {
		return errors.New("SESSION_SECRET is required")
	}
	if c.SessionSecret == defaultSessionSecret && (!loopbackURL(c.BaseURL) || !loopbackURL(c.SitePublicURL)) {
		return errors.New("SESSION_SECRET must be changed for non-loopback deployments")
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

func defaultDBPool(driver string) (maxOpen, maxIdle int) {
	if driver == "sqlite3" {
		return 1, 1
	}
	return 20, 10
}

func loopbackURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
