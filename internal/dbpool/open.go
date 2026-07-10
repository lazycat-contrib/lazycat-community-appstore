package dbpool

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type Config struct {
	Driver      string
	DSN         string
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

func Open(cfg Config) (*sql.DB, dialect.Driver, error) {
	if err := validate(cfg); err != nil {
		return nil, nil, err
	}
	entDialect, err := entDialectName(cfg.Driver)
	if err != nil {
		return nil, nil, err
	}
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpen)
	db.SetMaxIdleConns(min(cfg.MaxIdle, cfg.MaxOpen))
	db.SetConnMaxLifetime(cfg.MaxLifetime)
	db.SetConnMaxIdleTime(cfg.MaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return db, entsql.OpenDB(entDialect, db), nil
}

func validate(cfg Config) error {
	if cfg.MaxOpen <= 0 {
		return fmt.Errorf("max open connections must be positive")
	}
	if cfg.MaxIdle < 0 || cfg.MaxIdle > cfg.MaxOpen {
		return fmt.Errorf("max idle connections must be between zero and max open connections")
	}
	if cfg.MaxLifetime < 0 || cfg.MaxIdleTime < 0 {
		return fmt.Errorf("connection lifetimes must not be negative")
	}
	return nil
}

func entDialectName(driver string) (string, error) {
	switch driver {
	case "sqlite3":
		return dialect.SQLite, nil
	case "postgres":
		return dialect.Postgres, nil
	case "mysql":
		return dialect.MySQL, nil
	default:
		return "", fmt.Errorf("unsupported database driver %q", driver)
	}
}
