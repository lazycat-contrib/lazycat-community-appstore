package dbpool

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib-x/entsqlite"
)

func TestSQLiteConcurrentTransactions(t *testing.T) {
	db := openSQLiteCounter(t, 1)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	errs := make(chan error, 16*50)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 50 {
				if err := incrementSQLiteCounter(ctx, db); err != nil {
					errs <- err
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent increment: %v", err)
	}
	var got int
	if err := db.QueryRowContext(ctx, "SELECT value FROM counters WHERE id = 1").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != 800 {
		t.Fatalf("counter = %d, want 800", got)
	}
}

func BenchmarkSQLitePoolContention(b *testing.B) {
	maxOpen, err := strconv.Atoi(os.Getenv("APPSTORE_BENCH_SQLITE_MAX_OPEN"))
	if err != nil || (maxOpen != 1 && maxOpen != 4) {
		b.Fatal("APPSTORE_BENCH_SQLITE_MAX_OPEN must be 1 or 4")
	}
	db := openSQLiteCounter(b, maxOpen)
	b.Cleanup(func() { _ = db.Close() })
	var lockErrors atomic.Int64
	b.ReportAllocs()
	for b.Loop() {
		var wg sync.WaitGroup
		for range 16 {
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(b.Context(), 2*time.Second)
				defer cancel()
				if err := incrementSQLiteCounter(ctx, db); err != nil {
					if strings.Contains(strings.ToLower(err.Error()), "locked") {
						lockErrors.Add(1)
					}
					b.Error(err)
				}
			})
		}
		wg.Wait()
	}
	b.ReportMetric(float64(lockErrors.Load()), "lock-errors")
}

func openSQLiteCounter(tb testing.TB, maxOpen int) *sql.DB {
	tb.Helper()
	dsn := fmt.Sprintf("file:%s?cache=shared&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", filepath.Join(tb.TempDir(), "counter.db"))
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		tb.Fatal(err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)
	if _, err := db.ExecContext(tb.Context(), "CREATE TABLE counters(id INTEGER PRIMARY KEY, value INTEGER NOT NULL)"); err != nil {
		_ = db.Close()
		tb.Fatal(err)
	}
	if _, err := db.ExecContext(tb.Context(), "INSERT INTO counters(id, value) VALUES (1, 0)"); err != nil {
		_ = db.Close()
		tb.Fatal(err)
	}
	return db
}

func incrementSQLiteCounter(ctx context.Context, db *sql.DB) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && err == nil && rollbackErr != sql.ErrTxDone {
			err = rollbackErr
		}
	}()
	if _, err := tx.ExecContext(ctx, "UPDATE counters SET value = value + 1 WHERE id = 1"); err != nil {
		return err
	}
	return tx.Commit()
}
