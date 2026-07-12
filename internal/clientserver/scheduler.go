package clientserver

import (
	"context"
	"fmt"
	"strings"
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

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error

	startupSync func(context.Context)

	mu      sync.Mutex
	running map[string]struct{}
}

func newSourceSyncScheduler(server *Server) (*sourceSyncScheduler, error) {
	return newSourceSyncSchedulerWithStartup(server, nil)
}

func newSourceSyncSchedulerWithStartup(server *Server, startup func(context.Context)) (*sourceSyncScheduler, error) {
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
		server:    server,
		wheel:     wheel,
		ctx:       ctx,
		cancel:    cancel,
		running:   make(map[string]struct{}),
		closeDone: make(chan struct{}),
	}
	if startup == nil {
		scheduler.startupSync = scheduler.runStartupSyncs
	} else {
		scheduler.startupSync = startup
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
	scheduler.startStartupSyncs()
	return scheduler, nil
}

func (s *sourceSyncScheduler) Close() error {
	return s.CloseContext(context.Background())
}

func (s *sourceSyncScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *sourceSyncScheduler) CloseContext(ctx context.Context) error {
	s.closeOnce.Do(func() {
		if s.closeDone == nil {
			s.closeDone = make(chan struct{})
		}
		s.Stop()
		go func() {
			if s.wheel != nil {
				s.closeErr = s.wheel.Close()
			}
			s.wg.Wait()
			close(s.closeDone)
		}()
	})
	select {
	case <-s.closeDone:
		return s.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *sourceSyncScheduler) startStartupSyncs() {
	if s.startupSync == nil {
		return
	}
	s.wg.Go(func() {
		s.startupSync(s.ctx)
	})
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
	return s.runDueAutoUpdates(ctx, "")
}

func (s *sourceSyncScheduler) runDueAutoUpdates(ctx context.Context, _ string) error {
	now := time.Now()
	settings, err := s.server.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.AutoUpdateEnabledEQ(true)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, setting := range settings {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !autoUpdateDue(setting, now) {
			continue
		}
		s.updateUser(ctx, setting.UserID)
	}
	return nil
}

func (s *sourceSyncScheduler) runStartupSyncs(ctx context.Context) {
	settings, err := s.server.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.SyncOnStartupEQ(true)).
		All(ctx)
	if err != nil {
		return
	}
	for _, setting := range settings {
		if ctx.Err() != nil {
			return
		}
		s.syncUser(ctx, setting.UserID)
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

func autoUpdateDue(setting *ent.ClientSyncSetting, now time.Time) bool {
	if setting == nil || !setting.AutoUpdateEnabled {
		return false
	}
	if setting.LastAutoUpdateAt == nil {
		return true
	}
	interval := time.Duration(sanitizeAutoSyncInterval(setting.AutoUpdateIntervalMinutes)) * time.Minute
	return !setting.LastAutoUpdateAt.Add(interval).After(now)
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

func (s *sourceSyncScheduler) updateUser(ctx context.Context, userID string) {
	if !s.markRunning(userID) {
		return
	}
	defer s.unmarkRunning(userID)

	syncResult, err := s.server.syncAllSources(ctx, userID)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		_ = s.recordAutoUpdateResult(ctx, userID, clientsyncsetting.LastAutoUpdateStatusFailed, err.Error())
		return
	}
	if syncResult.Success == 0 && syncResult.Failed > 0 {
		_ = s.recordAutoUpdateResult(ctx, userID, clientsyncsetting.LastAutoUpdateStatusFailed, fmt.Sprintf("%d source syncs failed", syncResult.Failed))
		return
	}
	queueResult := s.server.RunUpdateQueueWithOptions(ctx, userID, UpdateQueueRequestDTO{RespectAutoUpdatePolicy: true})
	status, message := autoUpdateResultStatus(syncResult, queueResult)
	_ = s.recordAutoUpdateResult(ctx, userID, status, message)
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

func autoUpdateResultStatus(syncResult SyncAllResult, queueResult UpdateQueueResultDTO) (clientsyncsetting.LastAutoUpdateStatus, string) {
	message := strings.TrimSpace(queueResult.Error)
	switch queueResult.Status {
	case "failed":
		if message == "" {
			message = "automatic update queue failed"
		}
		return clientsyncsetting.LastAutoUpdateStatusFailed, message
	case "partial":
		if message == "" {
			message = "some applications could not be updated"
		}
		return clientsyncsetting.LastAutoUpdateStatusPartial, message
	case "already_running", "no_updates", "cancelled":
		return clientsyncsetting.LastAutoUpdateStatusSkipped, message
	default:
		if syncResult.Failed > 0 {
			return clientsyncsetting.LastAutoUpdateStatusPartial, fmt.Sprintf("%d source syncs failed", syncResult.Failed)
		}
		return clientsyncsetting.LastAutoUpdateStatusSuccess, message
	}
}

func (s *sourceSyncScheduler) recordAutoUpdateResult(ctx context.Context, userID string, status clientsyncsetting.LastAutoUpdateStatus, message string) error {
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
			SetAutoUpdateIntervalMinutes(defaultAutoSyncIntervalMinutes).
			SetLastAutoUpdateAt(now).
			SetLastAutoUpdateStatus(status)
		if message != "" {
			create.SetLastAutoUpdateError(message)
		}
		_, err = create.Save(ctx)
		return err
	}
	update := s.server.db.ClientSyncSetting.UpdateOneID(record.ID).
		SetLastAutoUpdateAt(now).
		SetLastAutoUpdateStatus(status)
	if message != "" {
		update.SetLastAutoUpdateError(message)
	} else {
		update.ClearLastAutoUpdateError()
	}
	_, err = update.Save(ctx)
	return err
}
