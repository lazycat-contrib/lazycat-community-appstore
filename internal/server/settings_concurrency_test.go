package server

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSourcePasswordRotationIsAtomic(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	for key, value := range map[string]string{
		settingSourcePassword:          "old-password",
		settingSourcePasswordRotation:  "1",
		sourcePasswordRotatedAtSetting: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
	} {
		if err := app.server.setSetting(ctx, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	const callers = 32
	start := make(chan struct{})
	values := make(chan string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			<-start
			values <- app.server.sourcePassword(ctx)
		})
	}
	close(start)
	wg.Wait()
	close(values)
	unique := map[string]struct{}{}
	for value := range values {
		unique[value] = struct{}{}
	}
	if len(unique) != 1 {
		t.Fatalf("rotated password count = %d, want 1", len(unique))
	}
	stored := app.server.setting(context.Background(), settingSourcePassword, "")
	for value := range unique {
		if value != stored {
			t.Fatalf("returned password = %q, stored = %q", value, stored)
		}
	}
}

func TestSourcePasswordCanceledContextReturnsPersistedFallback(t *testing.T) {
	app := newTestApp(t)
	if err := app.server.setSetting(t.Context(), settingSourcePassword, "persisted-password"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got := app.server.sourcePassword(ctx); got != "persisted-password" {
		t.Fatalf("sourcePassword() = %q, want persisted fallback", got)
	}
}

func TestSourcePasswordInvalidRotationIsFailSafe(t *testing.T) {
	app := newTestApp(t)
	for key, value := range map[string]string{
		settingSourcePassword:          "persisted-password",
		settingSourcePasswordRotation:  "not-a-number",
		sourcePasswordRotatedAtSetting: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
	} {
		if err := app.server.setSetting(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	if got := app.server.sourcePassword(t.Context()); got != "persisted-password" {
		t.Fatalf("sourcePassword() = %q, want persisted password", got)
	}
	if got := app.server.setting(t.Context(), settingSourcePassword, ""); got != "persisted-password" {
		t.Fatalf("stored password = %q", got)
	}
}

func TestSourcePasswordTimestampFailureRollsBackPassword(t *testing.T) {
	app := newTestApp(t)
	for key, value := range map[string]string{
		settingSourcePassword:          "old-password",
		settingSourcePasswordRotation:  "1",
		sourcePasswordRotatedAtSetting: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
	} {
		if err := app.server.setSetting(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.server.sqlDB.ExecContext(t.Context(), `
CREATE TRIGGER fail_source_password_rotation
BEFORE UPDATE ON site_settings
WHEN NEW.key = 'source_password_rotated_at'
BEGIN
    SELECT RAISE(ABORT, 'forced rotation timestamp failure');
END`); err != nil {
		t.Fatal(err)
	}
	if got := app.server.sourcePassword(t.Context()); got != "old-password" {
		t.Fatalf("sourcePassword() = %q, want old password", got)
	}
	if got := app.server.setting(t.Context(), settingSourcePassword, ""); got != "old-password" {
		t.Fatalf("stored password = %q, want rollback", got)
	}
}
