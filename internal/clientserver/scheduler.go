package clientserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsyncsetting"

	"github.com/lib-x/timewheel"
)

const (
	sourceSyncScanInterval = time.Minute
	sourceSyncScanSlots    = 60
)

type sourceSyncScheduler struct {
	server *Server
	wheel  *timewheel.TimeWheel[string]

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	running map[string]struct{}
}

func newSourceSyncScheduler(server *Server) (*sourceSyncScheduler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	wheel, err := timewheel.New[string](
		sourceSyncScanInterval,
		sourceSyncScanSlots,
		nil,
		timewheel.WithWorkerPool[string](1, 4, timewheel.Drop),
	)
	if err != nil {
		cancel()
		return nil, err
	}
	scheduler := &sourceSyncScheduler{
		server:  server,
		wheel:   wheel,
		ctx:     ctx,
		cancel:  cancel,
		running: make(map[string]struct{}),
	}
	if err := wheel.Start(ctx); err != nil {
		cancel()
		_ = wheel.Close()
		return nil, err
	}
	if _, err := wheel.AddRepeatingTimerWithContextJob(
		sourceSyncScanInterval,
		"client-source-sync-scan",
		scheduler.runDueAutoSyncs,
		timewheel.RepeatOptions{Mode: timewheel.SkipIfRunning},
	); err != nil {
		cancel()
		_ = wheel.Close()
		return nil, err
	}
	go scheduler.runStartupSyncs()
	return scheduler, nil
}

func (s *sourceSyncScheduler) Close() error {
	s.cancel()
	return s.wheel.Close()
}

func (s *sourceSyncScheduler) runDueAutoSyncs(ctx context.Context, _ string) error {
	now := time.Now()
	settings, err := s.server.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.AutoSyncEnabledEQ(true)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, setting := range settings {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !autoSyncDue(setting, now) {
			continue
		}
		s.syncUser(ctx, setting.UserID)
	}
	return nil
}

func (s *sourceSyncScheduler) runStartupSyncs() {
	settings, err := s.server.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.SyncOnStartupEQ(true)).
		All(s.ctx)
	if err != nil {
		return
	}
	for _, setting := range settings {
		if s.ctx.Err() != nil {
			return
		}
		s.syncUser(s.ctx, setting.UserID)
	}
}

func autoSyncDue(setting *ent.ClientSyncSetting, now time.Time) bool {
	if setting == nil || !setting.AutoSyncEnabled {
		return false
	}
	if setting.LastAutoSyncAt == nil {
		return true
	}
	interval := time.Duration(sanitizeAutoSyncInterval(setting.AutoSyncIntervalMinutes)) * time.Minute
	return !setting.LastAutoSyncAt.Add(interval).After(now)
}

func (s *sourceSyncScheduler) syncUser(ctx context.Context, userID string) {
	if !s.markRunning(userID) {
		return
	}
	defer s.unmarkRunning(userID)

	result, err := s.server.syncAllSources(ctx, userID)
	if ctx.Err() != nil {
		return
	}
	status, message := autoSyncResultStatus(result, err)
	_ = s.recordAutoSyncResult(ctx, userID, status, message)
}

func (s *sourceSyncScheduler) markRunning(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[userID]; ok {
		return false
	}
	s.running[userID] = struct{}{}
	return true
}

func (s *sourceSyncScheduler) unmarkRunning(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, userID)
}

func autoSyncResultStatus(result SyncAllResult, err error) (clientsyncsetting.LastAutoSyncStatus, string) {
	if err != nil {
		return clientsyncsetting.LastAutoSyncStatusFailed, err.Error()
	}
	switch {
	case result.Failed == 0:
		return clientsyncsetting.LastAutoSyncStatusSuccess, ""
	case result.Success > 0:
		return clientsyncsetting.LastAutoSyncStatusPartial, fmt.Sprintf("%d source syncs failed", result.Failed)
	default:
		return clientsyncsetting.LastAutoSyncStatusFailed, fmt.Sprintf("%d source syncs failed", result.Failed)
	}
}

func (s *sourceSyncScheduler) recordAutoSyncResult(ctx context.Context, userID string, status clientsyncsetting.LastAutoSyncStatus, message string) error {
	now := time.Now()
	record, err := s.server.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.UserIDEQ(userID)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if ent.IsNotFound(err) {
		create := s.server.db.ClientSyncSetting.Create().
			SetUserID(userID).
			SetAutoSyncIntervalMinutes(defaultAutoSyncIntervalMinutes).
			SetLastAutoSyncAt(now).
			SetLastAutoSyncStatus(status)
		if message != "" {
			create.SetLastAutoSyncError(message)
		}
		_, err = create.Save(ctx)
		return err
	}

	update := s.server.db.ClientSyncSetting.UpdateOneID(record.ID).
		SetLastAutoSyncAt(now).
		SetLastAutoSyncStatus(status)
	if message != "" {
		update.SetLastAutoSyncError(message)
	} else {
		update.ClearLastAutoSyncError()
	}
	_, err = update.Save(ctx)
	return err
}
