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
}

func LoadConfig() Config {
	return Config{
		Addr:              env("CLIENT_ADDR", "127.0.0.1:8090"),
		DBDSN:             env("CLIENT_DB_DSN", "./data/client.db"),
		DefaultSourceURL:  strings.TrimSpace(os.Getenv("CLIENT_DEFAULT_SOURCE_URL")),
		DefaultSourceName: env("CLIENT_DEFAULT_SOURCE_NAME", "Community Store"),
		SyncTimeout:       20 * time.Second,
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
