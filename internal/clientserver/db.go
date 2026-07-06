package clientserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"lazycat.community/appstore/ent"

	_ "github.com/lib-x/entsqlite"
)

func openDB(cfg Config) (*ent.Client, error) {
	if err := ensureSQLiteDir(cfg.DBDSN); err != nil {
		return nil, err
	}
	client, err := ent.Open("sqlite3", sqliteDSN(cfg.DBDSN))
	if err != nil {
		return nil, err
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func sqliteDSN(dsn string) string {
	if strings.HasPrefix(dsn, "file:") || strings.Contains(dsn, "?") {
		return ensureForeignKeysPragma(dsn)
	}
	return "file:" + dsn + "?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)"
}

func ensureForeignKeysPragma(dsn string) string {
	if strings.Contains(dsn, "_pragma=foreign_keys") {
		return dsn
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "_pragma=foreign_keys(1)"
}

func ensureSQLiteDir(dsn string) error {
	dsn = strings.TrimPrefix(dsn, "file:")
	if i := strings.IndexByte(dsn, '?'); i >= 0 {
		dsn = dsn[:i]
	}
	if dsn == "" || dsn == ":memory:" {
		return nil
	}
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
