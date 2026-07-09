package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupRunWritesMigrationPackageToMultipleStorages(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()

	for _, cfg := range []appStorageConfig{
		{
			Key:          "backup-a",
			Name:         "Backup A",
			Provider:     storageProviderLocal,
			DeliveryMode: storageDeliveryServer,
			LocalPath:    firstRoot,
		},
		{
			Key:          "backup-b",
			Name:         "Backup B",
			Provider:     storageProviderLocal,
			DeliveryMode: storageDeliveryServer,
			LocalPath:    secondRoot,
		},
	} {
		if err := app.server.saveStorageConfig(ctx, cfg); err != nil {
			t.Fatalf("save storage %s: %v", cfg.Key, err)
		}
	}

	result, err := app.server.runBackup(ctx, "manual", []string{"backup-a", "backup-b"})
	if err != nil {
		t.Fatalf("runBackup returned error: %v", err)
	}
	if result.Status != backupStatusSuccess {
		t.Fatalf("backup status = %q, error = %q", result.Status, result.Error)
	}
	if result.ObjectPath == "" || result.Size <= 0 || result.SHA256 == "" {
		t.Fatalf("backup result missing artifact metadata: %+v", result)
	}
	if len(result.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(result.Targets))
	}
	roots := map[string]string{"backup-a": firstRoot, "backup-b": secondRoot}
	for _, target := range result.Targets {
		if target.Status != backupStatusSuccess {
			t.Fatalf("target %s status = %q, error = %q", target.StorageKey, target.Status, target.Error)
		}
		if target.ObjectPath != result.ObjectPath {
			t.Fatalf("target object path = %q, want %q", target.ObjectPath, result.ObjectPath)
		}
		if _, err := os.Stat(filepath.Join(roots[target.StorageKey], filepath.FromSlash(target.ObjectPath))); err != nil {
			t.Fatalf("stored backup for %s missing: %v", target.StorageKey, err)
		}
	}
}

func TestBackupRunWritesToCustomTargetDirectories(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()

	for _, cfg := range []appStorageConfig{
		{
			Key:          "backup-a",
			Name:         "Backup A",
			Provider:     storageProviderLocal,
			DeliveryMode: storageDeliveryServer,
			LocalPath:    firstRoot,
		},
		{
			Key:          "backup-b",
			Name:         "Backup B",
			Provider:     storageProviderLocal,
			DeliveryMode: storageDeliveryServer,
			LocalPath:    secondRoot,
		},
	} {
		if err := app.server.saveStorageConfig(ctx, cfg); err != nil {
			t.Fatalf("save storage %s: %v", cfg.Key, err)
		}
	}

	result, err := app.server.runBackupWithTargets(ctx, "manual", []backupTargetConfig{
		{StorageKey: "backup-a", Directory: "site-a"},
		{StorageKey: "backup-b", Directory: "archives/site-b"},
	})
	if err != nil {
		t.Fatalf("runBackupWithTargets returned error: %v", err)
	}
	if result.Status != backupStatusSuccess {
		t.Fatalf("backup status = %q, error = %q", result.Status, result.Error)
	}
	if len(result.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(result.Targets))
	}
	roots := map[string]string{"backup-a": firstRoot, "backup-b": secondRoot}
	wantedPrefixes := map[string]string{"backup-a": "site-a/", "backup-b": "archives/site-b/"}
	for _, target := range result.Targets {
		if !strings.HasPrefix(target.ObjectPath, wantedPrefixes[target.StorageKey]) {
			t.Fatalf("target %s object path = %q, want prefix %q", target.StorageKey, target.ObjectPath, wantedPrefixes[target.StorageKey])
		}
		if _, err := os.Stat(filepath.Join(roots[target.StorageKey], filepath.FromSlash(target.ObjectPath))); err != nil {
			t.Fatalf("stored custom backup for %s missing: %v", target.StorageKey, err)
		}
	}
}

func TestBackupSettingsAcceptTargetDirectories(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	root := t.TempDir()
	if err := app.server.saveStorageConfig(ctx, appStorageConfig{
		Key:          "backup-a",
		Name:         "Backup A",
		Provider:     storageProviderLocal,
		DeliveryMode: storageDeliveryServer,
		LocalPath:    root,
	}); err != nil {
		t.Fatalf("save storage: %v", err)
	}
	app.login("admin", "changeme")

	rec := app.do(http.MethodPatch, "/api/v1/admin/backups/settings", map[string]any{
		"enabled":      true,
		"scheduleTime": "04:30",
		"targets": []map[string]string{
			{"storageKey": "backup-a", "directory": "custom/site"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update backup settings = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"storageKeys":["backup-a"]`, `"storageKey":"backup-a"`, `"directory":"custom/site"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("backup settings response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestShouldRunScheduledBackupOncePerScheduledDay(t *testing.T) {
	now := time.Date(2026, 7, 9, 4, 5, 0, 0, time.UTC)
	if !shouldRunScheduledBackup(now, "04:00", nil) {
		t.Fatal("expected backup to run after schedule time without previous run")
	}
	lastBeforeSchedule := &backupRunResult{StartedAt: time.Date(2026, 7, 9, 3, 59, 0, 0, time.UTC).Format(time.RFC3339)}
	if !shouldRunScheduledBackup(now, "04:00", lastBeforeSchedule) {
		t.Fatal("expected backup to run when previous run was before today's schedule")
	}
	lastAfterSchedule := &backupRunResult{StartedAt: time.Date(2026, 7, 9, 4, 1, 0, 0, time.UTC).Format(time.RFC3339)}
	if shouldRunScheduledBackup(now, "04:00", lastAfterSchedule) {
		t.Fatal("did not expect backup to run twice after today's schedule")
	}
	beforeSchedule := time.Date(2026, 7, 9, 3, 55, 0, 0, time.UTC)
	if shouldRunScheduledBackup(beforeSchedule, "04:00", nil) {
		t.Fatal("did not expect backup to run before schedule time")
	}
}
