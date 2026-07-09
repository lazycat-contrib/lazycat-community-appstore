package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/internal/migration"
	"lazycat.community/appstore/internal/storage"
)

const (
	defaultBackupScheduleTime = "03:00"
	defaultBackupDirectory    = "backups/appstore"
	backupStatusSuccess       = "success"
	backupStatusPartial       = "partial"
	backupStatusFailed        = "failed"
)

var errBackupAlreadyRunning = errors.New("backup is already running")

type backupSettingsDTO struct {
	Enabled      bool                 `json:"enabled"`
	ScheduleTime string               `json:"scheduleTime"`
	StorageKeys  []string             `json:"storageKeys"`
	Targets      []backupTargetConfig `json:"targets"`
	LastRun      *backupRunResult     `json:"lastRun,omitempty"`
	IsRunning    bool                 `json:"isRunning"`
}

type backupSettingsInput struct {
	Enabled      *bool                 `json:"enabled"`
	ScheduleTime *string               `json:"scheduleTime"`
	StorageKeys  *[]string             `json:"storageKeys"`
	Targets      *[]backupTargetConfig `json:"targets"`
}

type backupTargetConfig struct {
	StorageKey string `json:"storageKey"`
	Directory  string `json:"directory"`
}

type backupRunResult struct {
	StartedAt      string               `json:"startedAt"`
	FinishedAt     string               `json:"finishedAt,omitempty"`
	Trigger        string               `json:"trigger"`
	Status         string               `json:"status"`
	ObjectPath     string               `json:"objectPath,omitempty"`
	Size           int64                `json:"size,omitempty"`
	SHA256         string               `json:"sha256,omitempty"`
	ManifestCounts map[string]int       `json:"manifestCounts,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
	Targets        []backupTargetResult `json:"targets,omitempty"`
	Error          string               `json:"error,omitempty"`
}

type backupTargetResult struct {
	StorageKey  string `json:"storageKey"`
	StorageName string `json:"storageName"`
	Directory   string `json:"directory,omitempty"`
	ObjectPath  string `json:"objectPath,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

type backupTarget struct {
	key       string
	name      string
	directory string
	writer    storage.ObjectWriter
}

func (s *Server) handleGetBackupSettings(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	settings, err := s.loadBackupSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BACKUP_SETTINGS_FAILED", "Could not load backup settings", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (s *Server) handleUpdateBackupSettings(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	current, err := s.loadBackupSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BACKUP_SETTINGS_FAILED", "Could not load backup settings", nil)
		return
	}
	var input backupSettingsInput
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	next := current
	if input.Enabled != nil {
		next.Enabled = *input.Enabled
	}
	if input.ScheduleTime != nil {
		next.ScheduleTime = strings.TrimSpace(*input.ScheduleTime)
	}
	if input.StorageKeys != nil {
		next.StorageKeys = *input.StorageKeys
		next.Targets = backupTargetsFromKeys(*input.StorageKeys)
	}
	if input.Targets != nil {
		next.Targets = *input.Targets
		next.StorageKeys = backupStorageKeysFromTargets(*input.Targets)
	}
	scheduleTime, err := normalizeBackupScheduleTime(next.ScheduleTime)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	targets, err := s.normalizeBackupTargets(r.Context(), next.Targets, next.Enabled)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	next.ScheduleTime = scheduleTime
	next.Targets = targets
	next.StorageKeys = backupStorageKeysFromTargets(targets)
	if err := s.saveBackupSettings(r.Context(), next); err != nil {
		writeError(w, http.StatusInternalServerError, "BACKUP_SETTINGS_SAVE_FAILED", "Could not save backup settings", nil)
		return
	}
	settings, err := s.loadBackupSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BACKUP_SETTINGS_FAILED", "Could not load backup settings", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (s *Server) handleRunBackup(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	settings, err := s.loadBackupSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BACKUP_SETTINGS_FAILED", "Could not load backup settings", nil)
		return
	}
	result, err := s.runBackupWithTargets(r.Context(), "manual", settings.Targets)
	if errors.Is(err, errBackupAlreadyRunning) {
		writeError(w, http.StatusConflict, "BACKUP_ALREADY_RUNNING", "A backup is already running", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "BACKUP_RUN_FAILED", err.Error(), nil)
		return
	}
	updated, err := s.loadBackupSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BACKUP_SETTINGS_FAILED", "Could not load backup settings", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "settings": updated})
}

func (s *Server) loadBackupSettings(ctx context.Context) (backupSettingsDTO, error) {
	scheduleTime, err := normalizeBackupScheduleTime(s.setting(ctx, settingBackupScheduleTime, defaultBackupScheduleTime))
	if err != nil {
		scheduleTime = defaultBackupScheduleTime
	}
	settings := backupSettingsDTO{
		Enabled:      s.settingBool(ctx, settingBackupEnabled, false),
		ScheduleTime: scheduleTime,
		IsRunning:    s.isBackupRunning(),
	}
	settings.StorageKeys = decodeBackupStorageKeys(s.setting(ctx, settingBackupStorageKeys, ""))
	settings.Targets = decodeBackupTargets(s.setting(ctx, settingBackupTargets, ""), settings.StorageKeys)
	settings.StorageKeys = backupStorageKeysFromTargets(settings.Targets)
	if raw := strings.TrimSpace(s.setting(ctx, settingBackupLastRun, "")); raw != "" {
		var result backupRunResult
		if err := json.Unmarshal([]byte(raw), &result); err == nil && result.StartedAt != "" {
			settings.LastRun = &result
		}
	}
	return settings, nil
}

func (s *Server) saveBackupSettings(ctx context.Context, settings backupSettingsDTO) error {
	keys, err := json.Marshal(settings.StorageKeys)
	if err != nil {
		return err
	}
	targets, err := json.Marshal(settings.Targets)
	if err != nil {
		return err
	}
	if err := s.setSetting(ctx, settingBackupEnabled, fmt.Sprintf("%t", settings.Enabled)); err != nil {
		return err
	}
	if err := s.setSetting(ctx, settingBackupScheduleTime, settings.ScheduleTime); err != nil {
		return err
	}
	if err := s.setSetting(ctx, settingBackupStorageKeys, string(keys)); err != nil {
		return err
	}
	return s.setSetting(ctx, settingBackupTargets, string(targets))
}

func (s *Server) setBackupLastRun(ctx context.Context, result backupRunResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.setSetting(ctx, settingBackupLastRun, string(raw))
}

func (s *Server) startBackupScheduler() {
	if s.backupCtx == nil {
		return
	}
	s.backupWG.Add(1)
	go func() {
		defer s.backupWG.Done()
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				s.runBackupSchedulerTick()
				timer.Reset(time.Minute)
			case <-s.backupCtx.Done():
				return
			}
		}
	}()
}

func (s *Server) runBackupSchedulerTick() {
	ctx, cancel := context.WithTimeout(s.backupCtx, 6*time.Hour)
	defer cancel()
	settings, err := s.loadBackupSettings(ctx)
	if err != nil || !settings.Enabled || len(settings.Targets) == 0 {
		return
	}
	if !shouldRunScheduledBackup(time.Now(), settings.ScheduleTime, settings.LastRun) {
		return
	}
	_, _ = s.runBackupWithTargets(ctx, "scheduled", settings.Targets)
}

func shouldRunScheduledBackup(now time.Time, scheduleTime string, lastRun *backupRunResult) bool {
	parsed, err := time.Parse("15:04", scheduleTime)
	if err != nil {
		return false
	}
	scheduledAt := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	if now.Before(scheduledAt) {
		return false
	}
	if lastRun == nil || strings.TrimSpace(lastRun.StartedAt) == "" {
		return true
	}
	lastStartedAt, err := time.Parse(time.RFC3339, lastRun.StartedAt)
	if err != nil {
		return true
	}
	return lastStartedAt.Before(scheduledAt)
}

func (s *Server) runBackup(ctx context.Context, trigger string, storageKeys []string) (*backupRunResult, error) {
	return s.runBackupWithTargets(ctx, trigger, backupTargetsFromKeys(storageKeys))
}

func (s *Server) runBackupWithTargets(ctx context.Context, trigger string, targetConfigs []backupTargetConfig) (*backupRunResult, error) {
	if !s.beginBackupRun() {
		return nil, errBackupAlreadyRunning
	}
	defer s.finishBackupRun()

	startedAt := time.Now().UTC()
	fileName := fmt.Sprintf("appstore-backup-%s.zip", startedAt.Format("20060102-150405.000000000"))
	result := backupRunResult{
		StartedAt:  startedAt.Format(time.RFC3339),
		Trigger:    trigger,
		Status:     backupStatusFailed,
		ObjectPath: path.Join(defaultBackupDirectory, fileName),
	}
	finish := func() *backupRunResult {
		result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		_ = s.setBackupLastRun(ctx, result)
		return &result
	}

	targets, err := s.resolveBackupTargets(ctx, targetConfigs)
	if err != nil {
		result.Error = err.Error()
		return finish(), nil
	}

	var buf bytes.Buffer
	manifest, err := migration.NewExporter(s.db, s.migrationStorageResolver(), appVersion()).Export(ctx, &buf, migration.DefaultOptions())
	if err != nil {
		result.Error = fmt.Sprintf("export migration package: %v", err)
		return finish(), nil
	}
	sum := sha256.Sum256(buf.Bytes())
	result.Size = int64(buf.Len())
	result.SHA256 = hex.EncodeToString(sum[:])
	result.ManifestCounts = manifest.Counts
	result.Warnings = manifest.Warnings

	successCount := 0
	for _, target := range targets {
		targetResult := backupTargetResult{
			StorageKey:  target.key,
			StorageName: target.name,
			Directory:   target.directory,
			Status:      backupStatusFailed,
		}
		objectPath := path.Join(target.directory, fileName)
		obj, err := target.writer.SaveObject(ctx, objectPath, bytes.NewReader(buf.Bytes()))
		if err != nil {
			targetResult.Error = err.Error()
			result.Targets = append(result.Targets, targetResult)
			continue
		}
		targetResult.Status = backupStatusSuccess
		targetResult.ObjectPath = obj.Path
		targetResult.DownloadURL = s.absoluteURL(ctx, obj.DownloadURL)
		result.Targets = append(result.Targets, targetResult)
		successCount++
	}
	switch {
	case successCount == len(targets):
		result.Status = backupStatusSuccess
	case successCount > 0:
		result.Status = backupStatusPartial
		result.Error = fmt.Sprintf("%d of %d backup targets failed", len(targets)-successCount, len(targets))
	default:
		result.Status = backupStatusFailed
		result.Error = "all backup targets failed"
	}
	return finish(), nil
}

func (s *Server) resolveBackupTargets(ctx context.Context, targetConfigs []backupTargetConfig) ([]backupTarget, error) {
	targetConfigs, err := s.normalizeBackupTargets(ctx, targetConfigs, true)
	if err != nil {
		return nil, err
	}
	targets := make([]backupTarget, 0, len(targetConfigs))
	for _, targetConfig := range targetConfigs {
		cfg, err := s.effectiveStorageConfigByKey(ctx, targetConfig.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("storage %q is not configured", targetConfig.StorageKey)
		}
		backend, err := storageBackendFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("storage %q is invalid: %w", targetConfig.StorageKey, err)
		}
		writer, ok := backend.(storage.ObjectWriter)
		if !ok {
			return nil, fmt.Errorf("storage %q does not support writing backup objects", targetConfig.StorageKey)
		}
		targets = append(targets, backupTarget{key: targetConfig.StorageKey, name: storageDisplayName(cfg), directory: targetConfig.Directory, writer: writer})
	}
	return targets, nil
}

func (s *Server) normalizeBackupStorageKeys(ctx context.Context, raw []string, require bool) ([]string, error) {
	targets, err := s.normalizeBackupTargets(ctx, backupTargetsFromKeys(raw), require)
	if err != nil {
		return nil, err
	}
	return backupStorageKeysFromTargets(targets), nil
}

func (s *Server) normalizeBackupTargets(ctx context.Context, raw []backupTargetConfig, require bool) ([]backupTargetConfig, error) {
	seen := map[string]struct{}{}
	targets := make([]backupTargetConfig, 0, len(raw))
	for _, item := range raw {
		key := normalizeStorageKey(item.StorageKey)
		if key == "" {
			return nil, fmt.Errorf("backup storage key %q is invalid", item.StorageKey)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		directory, err := normalizeBackupDirectory(item.Directory)
		if err != nil {
			return nil, err
		}
		backend, err := s.storageBackendForKey(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("storage %q is not configured", key)
		}
		if _, ok := backend.(storage.ObjectWriter); !ok {
			return nil, fmt.Errorf("storage %q does not support writing backup objects", key)
		}
		seen[key] = struct{}{}
		targets = append(targets, backupTargetConfig{StorageKey: key, Directory: directory})
	}
	if len(targets) > 8 {
		return nil, fmt.Errorf("at most 8 backup storages can be selected")
	}
	if require && len(targets) == 0 {
		return nil, fmt.Errorf("at least one backup storage is required")
	}
	return targets, nil
}

func normalizeBackupScheduleTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultBackupScheduleTime
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return "", fmt.Errorf("scheduleTime must use HH:mm format")
	}
	return parsed.Format("15:04"), nil
}

func normalizeBackupDirectory(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return defaultBackupDirectory, nil
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("backup directory must be a relative path")
	}
	value = strings.TrimSuffix(value, "/")
	if value == "" {
		return "", fmt.Errorf("backup directory must be a relative path")
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("backup directory contains an invalid segment")
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("backup directory must stay inside storage root")
	}
	if len(cleaned) > 240 {
		return "", fmt.Errorf("backup directory is too long")
	}
	return cleaned, nil
}

func decodeBackupStorageKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return nil
	}
	return keys
}

func decodeBackupTargets(raw string, fallbackKeys []string) []backupTargetConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return backupTargetsFromKeys(fallbackKeys)
	}
	var targets []backupTargetConfig
	if err := json.Unmarshal([]byte(raw), &targets); err != nil || len(targets) == 0 {
		return backupTargetsFromKeys(fallbackKeys)
	}
	for index := range targets {
		key := normalizeStorageKey(targets[index].StorageKey)
		if key == "" {
			return backupTargetsFromKeys(fallbackKeys)
		}
		directory, err := normalizeBackupDirectory(targets[index].Directory)
		if err != nil {
			return backupTargetsFromKeys(fallbackKeys)
		}
		targets[index] = backupTargetConfig{StorageKey: key, Directory: directory}
	}
	return targets
}

func backupTargetsFromKeys(keys []string) []backupTargetConfig {
	targets := make([]backupTargetConfig, 0, len(keys))
	for _, key := range keys {
		key = normalizeStorageKey(key)
		if key == "" {
			continue
		}
		targets = append(targets, backupTargetConfig{StorageKey: key, Directory: defaultBackupDirectory})
	}
	return targets
}

func backupStorageKeysFromTargets(targets []backupTargetConfig) []string {
	keys := make([]string, 0, len(targets))
	for _, target := range targets {
		key := normalizeStorageKey(target.StorageKey)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func (s *Server) beginBackupRun() bool {
	s.backupRunMu.Lock()
	defer s.backupRunMu.Unlock()
	if s.backupRunning {
		return false
	}
	s.backupRunning = true
	return true
}

func (s *Server) finishBackupRun() {
	s.backupRunMu.Lock()
	s.backupRunning = false
	s.backupRunMu.Unlock()
}

func (s *Server) isBackupRunning() bool {
	s.backupRunMu.Lock()
	defer s.backupRunMu.Unlock()
	return s.backupRunning
}
