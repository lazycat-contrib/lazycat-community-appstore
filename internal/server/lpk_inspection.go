package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/backoff"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/lpkinspectionjob"
	"lazycat.community/appstore/internal/catalogmeta"
)

const (
	lpkInspectionScanInterval = time.Second
	manualLPKInspectionWait   = 30 * time.Second
)

type lpkInspectionScheduler struct {
	server *Server
	ctx    context.Context
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}
	once   sync.Once
}

func newLPKInspectionScheduler(server *Server) (*lpkInspectionScheduler, error) {
	if server == nil || server.ctx == nil {
		return nil, errors.New("LPK inspection scheduler requires a running server")
	}
	ctx, cancel := context.WithCancel(server.ctx)
	scheduler := &lpkInspectionScheduler{
		server: server,
		ctx:    ctx,
		cancel: cancel,
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	if !server.beginBackground() {
		cancel()
		return nil, errors.New("server is stopping")
	}
	go func() {
		defer server.endBackground()
		defer close(scheduler.done)
		scheduler.recoverInterruptedJobs(ctx)
		scheduler.runDue(ctx)
		ticker := time.NewTicker(lpkInspectionScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scheduler.runDue(ctx)
			case <-scheduler.wake:
				scheduler.runDue(ctx)
			}
		}
	}()
	return scheduler, nil
}

func (s *lpkInspectionScheduler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *lpkInspectionScheduler) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

func (s *lpkInspectionScheduler) CloseContext(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.once.Do(s.Stop)
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *lpkInspectionScheduler) recoverInterruptedJobs(ctx context.Context) {
	_, _ = s.server.db.LPKInspectionJob.Update().
		Where(lpkinspectionjob.StateEQ(lpkinspectionjob.StateRUNNING)).
		SetState(lpkinspectionjob.StatePENDING).
		Save(ctx)
}

func (s *lpkInspectionScheduler) runDue(ctx context.Context) {
	now := s.server.now()
	jobs, err := s.server.db.LPKInspectionJob.Query().
		Where(lpkinspectionjob.StateEQ(lpkinspectionjob.StatePENDING)).
		Order(entgo.Asc(lpkinspectionjob.FieldNextAttemptAt), entgo.Asc(lpkinspectionjob.FieldCreatedAt)).
		Limit(8).
		All(ctx)
	if err != nil {
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		if job.NextAttemptAt != nil && job.NextAttemptAt.After(now) {
			continue
		}
		_ = s.server.runLPKInspectionJob(ctx, job.ID, now)
	}
}

func (s *Server) enqueueAutomaticLPKInspection(ctx context.Context, appID, versionID, userID int, downloadURL string) error {
	wait := s.automaticLPKInspectionWait(ctx)
	if wait <= 0 {
		return nil
	}
	now := s.now()
	_, err := s.db.LPKInspectionJob.Create().
		SetAppID(appID).
		SetVersionID(versionID).
		SetUserID(userID).
		SetDownloadURL(downloadURL).
		SetTrigger(lpkinspectionjob.TriggerAPI_TOKEN_FIRST_SUBMISSION).
		SetDeadlineAt(now.Add(wait)).
		SetNextAttemptAt(now).
		Save(ctx)
	if err == nil && s.lpkInspectionScheduler != nil {
		s.lpkInspectionScheduler.notify()
	}
	return err
}

func (s *Server) enqueueManualLPKInspection(ctx context.Context, appID, userID int, overwrite bool) (*entgo.LPKInspectionJob, error) {
	if existing, err := s.db.LPKInspectionJob.Query().
		Where(
			lpkinspectionjob.AppIDEQ(appID),
			lpkinspectionjob.StateIn(lpkinspectionjob.StatePENDING, lpkinspectionjob.StateRUNNING),
		).
		Order(entgo.Desc(lpkinspectionjob.FieldCreatedAt)).
		First(ctx); err == nil {
		return existing, nil
	} else if !entgo.IsNotFound(err) {
		return nil, err
	}
	version, err := s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(appID), appversion.DownloadURLNEQ("")).
		Order(entgo.Desc(appversion.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		return nil, errors.New("app has no external LPK version to inspect")
	}
	now := s.now()
	job, err := s.db.LPKInspectionJob.Create().
		SetAppID(appID).
		SetVersionID(version.ID).
		SetUserID(userID).
		SetDownloadURL(version.DownloadURL).
		SetTrigger(lpkinspectionjob.TriggerMANUAL).
		SetOverwriteExistingMetadata(overwrite).
		SetDeadlineAt(now.Add(manualLPKInspectionWait)).
		SetNextAttemptAt(now).
		Save(ctx)
	if err == nil && s.lpkInspectionScheduler != nil {
		s.lpkInspectionScheduler.notify()
	}
	return job, err
}

type createLPKInspectionRequest struct {
	OverwriteExistingMetadata bool `json:"overwriteExistingMetadata"`
}

type bulkLPKInspectionItem struct {
	AppID      int                    `json:"appId"`
	AppName    string                 `json:"appName"`
	Inspection lpkInspectionStatusDTO `json:"inspection"`
}

type bulkLPKInspectionSkip struct {
	AppID   int    `json:"appId"`
	AppName string `json:"appName"`
	Reason  string `json:"reason"`
}

type bulkLPKInspectionResponse struct {
	Inspections []bulkLPKInspectionItem `json:"inspections"`
	Skipped     []bulkLPKInspectionSkip `json:"skipped"`
}

type bulkLPKInspectionStatusRequest struct {
	IDs []int `json:"ids"`
}

type createBulkLPKInspectionRequest struct {
	AppIDs                    []int `json:"appIds"`
	OverwriteExistingMetadata bool  `json:"overwriteExistingMetadata"`
}

func (s *Server) handleCreateBulkLPKInspections(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input createBulkLPKInspectionRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if len(input.AppIDs) == 0 || len(input.AppIDs) > 200 {
		badRequest(w, errors.New("app ids must contain between 1 and 200 entries"))
		return
	}
	records, err := s.db.App.Query().
		Where(app.OwnerIDEQ(u.ID), app.IDIn(input.AppIDs...)).
		Order(entgo.Desc(app.FieldUpdatedAt)).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LPK_INSPECTION_LIST_FAILED", "Could not list owned apps", nil)
		return
	}
	response := bulkLPKInspectionResponse{Inspections: []bulkLPKInspectionItem{}, Skipped: []bulkLPKInspectionSkip{}}
	for _, record := range records {
		job, enqueueErr := s.enqueueManualLPKInspection(r.Context(), record.ID, u.ID, input.OverwriteExistingMetadata)
		if enqueueErr != nil {
			response.Skipped = append(response.Skipped, bulkLPKInspectionSkip{AppID: record.ID, AppName: record.Name, Reason: enqueueErr.Error()})
			continue
		}
		response.Inspections = append(response.Inspections, bulkLPKInspectionItem{AppID: record.ID, AppName: record.Name, Inspection: toLPKInspectionStatusDTO(job)})
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (s *Server) handleBulkLPKInspectionStatus(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input bulkLPKInspectionStatusRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if len(input.IDs) == 0 || len(input.IDs) > 200 {
		badRequest(w, errors.New("inspection ids must contain between 1 and 200 entries"))
		return
	}
	records, err := s.db.App.Query().Where(app.OwnerIDEQ(u.ID)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LPK_INSPECTION_LIST_FAILED", "Could not list owned apps", nil)
		return
	}
	appNames := make(map[int]string, len(records))
	appIDs := make([]int, 0, len(records))
	for _, record := range records {
		appNames[record.ID] = record.Name
		appIDs = append(appIDs, record.ID)
	}
	response := bulkLPKInspectionResponse{Inspections: []bulkLPKInspectionItem{}, Skipped: []bulkLPKInspectionSkip{}}
	if len(appIDs) == 0 {
		writeJSON(w, http.StatusOK, response)
		return
	}
	jobs, err := s.db.LPKInspectionJob.Query().
		Where(lpkinspectionjob.IDIn(input.IDs...), lpkinspectionjob.AppIDIn(appIDs...)).
		Order(entgo.Asc(lpkinspectionjob.FieldID)).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LPK_INSPECTION_STATUS_FAILED", "Could not load inspection status", nil)
		return
	}
	for _, job := range jobs {
		response.Inspections = append(response.Inspections, bulkLPKInspectionItem{AppID: job.AppID, AppName: appNames[job.AppID], Inspection: toLPKInspectionStatusDTO(job)})
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateLPKInspection(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || appID <= 0 {
		badRequest(w, errors.New("invalid app id"))
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.canUploadVersion(r, record, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to inspect this app", nil)
		return
	}
	var input createLPKInspectionRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	job, err := s.enqueueManualLPKInspection(r.Context(), record.ID, u.ID, input.OverwriteExistingMetadata)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "LPK_INSPECTION_UNAVAILABLE", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"inspection": toLPKInspectionStatusDTO(job)})
}

func (s *Server) runLPKInspectionJob(ctx context.Context, jobID int, now time.Time) error {
	job, err := s.db.LPKInspectionJob.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if lpkInspectionTerminal(job.State) {
		return nil
	}
	if job.DeadlineAt != nil && !now.Before(*job.DeadlineAt) {
		return s.finishLPKInspectionJob(ctx, job, lpkinspectionjob.StateTIMED_OUT, "LPK metadata was not available before the inspection deadline")
	}
	job, err = job.Update().
		SetState(lpkinspectionjob.StateRUNNING).
		SetAttempts(job.Attempts + 1).
		ClearLastError().
		Save(ctx)
	if err != nil {
		return err
	}
	remaining := manualLPKInspectionWait
	if job.DeadlineAt != nil {
		remaining = time.Until(*job.DeadlineAt)
		if s.now != nil {
			remaining = job.DeadlineAt.Sub(now)
		}
	}
	if remaining <= 0 {
		return s.finishLPKInspectionJob(ctx, job, lpkinspectionjob.StateTIMED_OUT, "LPK metadata was not available before the inspection deadline")
	}
	inspect := s.inspectLPKForJob
	if inspect == nil {
		inspect = s.inspectLPKURLWithTimeout
	}
	inspected, err := inspect(ctx, job.DownloadURL, s.effectiveMaxLPKSize(ctx), true, remaining)
	if err == nil {
		err = s.applyLPKInspectionMetadata(ctx, job, inspected)
	}
	if err == nil {
		return s.finishLPKInspectionJob(ctx, job, lpkinspectionjob.StateSUCCEEDED, "")
	}
	if !temporaryLPKInspectionError(err) {
		return s.finishLPKInspectionJob(ctx, job, lpkinspectionjob.StateFAILED, err.Error())
	}
	next := now.Add(lpkInspectionBackoff(job.Attempts))
	if job.DeadlineAt != nil && !next.Before(*job.DeadlineAt) {
		return s.finishLPKInspectionJob(ctx, job, lpkinspectionjob.StateTIMED_OUT, err.Error())
	}
	_, updateErr := job.Update().
		SetState(lpkinspectionjob.StatePENDING).
		SetLastError(err.Error()).
		SetNextAttemptAt(next).
		Save(ctx)
	return updateErr
}

func (s *Server) finishLPKInspectionJob(ctx context.Context, job *entgo.LPKInspectionJob, state lpkinspectionjob.State, message string) error {
	now := s.now()
	update := job.Update().
		SetState(state).
		SetCompletedAt(now).
		ClearNextAttemptAt()
	if message != "" {
		update.SetLastError(message)
	} else {
		update.ClearLastError()
	}
	_, err := update.Save(ctx)
	return err
}

func (s *Server) applyLPKInspectionMetadata(ctx context.Context, job *entgo.LPKInspectionJob, inspected lpkInspection) error {
	record, err := s.db.App.Get(ctx, job.AppID)
	if err != nil {
		return err
	}
	meta := inspected.Metadata
	if packageID := strings.TrimSpace(meta.PackageID); packageID != "" && packageID != record.PackageID {
		return fmt.Errorf("LPK package %q does not match app packageId %q", packageID, record.PackageID)
	}
	overwrite := job.OverwriteExistingMetadata
	update := record.Update()
	changed := false
	if lpkTextShouldApply(record.Name, meta.Name, overwrite) {
		update.SetName(meta.Name)
		changed = true
	}
	if lpkTextShouldApply(record.Description, meta.Description, overwrite) {
		update.SetDescription(meta.Description)
		changed = true
	}
	if summary := packageSummary(meta.Description); lpkTextShouldApply(record.Summary, summary, overwrite) {
		update.SetSummary(summary)
		changed = true
	}
	nameI18n := catalogmeta.DecodeLocalizedText(record.NameI18nJSON)
	if !meta.NameI18n.IsZero() && (overwrite || nameI18n.IsZero()) {
		update.SetNameI18nJSON(catalogmeta.EncodeLocalizedText(meta.NameI18n))
		changed = true
	}
	descriptionI18n := catalogmeta.DecodeLocalizedText(record.DescriptionI18nJSON)
	if !meta.DescriptionI18n.IsZero() && (overwrite || descriptionI18n.IsZero()) {
		update.SetDescriptionI18nJSON(catalogmeta.EncodeLocalizedText(meta.DescriptionI18n))
		changed = true
	}
	summaryI18n := catalogmeta.DecodeLocalizedText(record.SummaryI18nJSON)
	if generated := packageSummaryI18n(meta.DescriptionI18n); !generated.IsZero() && (overwrite || summaryI18n.IsZero()) {
		update.SetSummaryI18nJSON(catalogmeta.EncodeLocalizedText(generated))
		changed = true
	}
	if lpkTextShouldApply(record.Author, meta.Author, overwrite) {
		update.SetAuthor(meta.Author)
		changed = true
	}
	if lpkTextShouldApply(record.Homepage, meta.Homepage, overwrite) {
		update.SetHomepage(meta.Homepage)
		changed = true
	}
	if lpkTextShouldApply(record.License, meta.License, overwrite) {
		update.SetLicense(meta.License)
		changed = true
	}
	if lpkTextShouldApply(record.MinOsVersion, meta.MinOSVersion, overwrite) {
		update.SetMinOsVersion(meta.MinOSVersion)
		changed = true
	}
	iconAssetID := 0
	if len(meta.IconData) > 0 && (overwrite || record.IconURL == nil || strings.TrimSpace(*record.IconURL) == "") {
		iconURL, assetID, iconErr := s.saveLPKIconAsset(ctx, meta)
		if iconErr != nil {
			return iconErr
		}
		if iconURL != "" {
			update.SetIconURL(iconURL)
			iconAssetID = assetID
			changed = true
		}
	}
	if changed {
		if _, err := update.Save(ctx); err != nil {
			if iconAssetID > 0 {
				_ = s.cleanupAssetIDs(ctx, iconAssetID)
			}
			return err
		}
		if iconAssetID > 0 {
			if err := s.replaceAssetLinks(ctx, assetOwnerApp, record.ID, assetRoleIcon, iconAssetID); err != nil {
				return err
			}
		}
	}
	if job.VersionID == nil {
		return nil
	}
	version, err := s.db.AppVersion.Get(ctx, *job.VersionID)
	if err != nil {
		return err
	}
	if version.AppID != record.ID {
		return errors.New("LPK inspection version does not belong to the application")
	}
	versionUpdate := version.Update()
	versionChanged := false
	if (overwrite || strings.TrimSpace(version.Sha256) == "") && strings.TrimSpace(inspected.SHA256) != "" {
		versionUpdate.SetSha256(inspected.SHA256)
		versionChanged = true
	}
	if (overwrite || version.FileSize <= 0) && inspected.Size > 0 {
		versionUpdate.SetFileSize(inspected.Size)
		versionChanged = true
	}
	if versionChanged {
		_, err = versionUpdate.Save(ctx)
	}
	return err
}

func lpkTextShouldApply(existing, incoming string, overwrite bool) bool {
	return strings.TrimSpace(incoming) != "" && (overwrite || strings.TrimSpace(existing) == "")
}

func lpkInspectionTerminal(state lpkinspectionjob.State) bool {
	switch state {
	case lpkinspectionjob.StateSUCCEEDED, lpkinspectionjob.StateFAILED, lpkinspectionjob.StateTIMED_OUT, lpkinspectionjob.StateCANCELLED:
		return true
	default:
		return false
	}
}

func lpkInspectionBackoff(attempts int) time.Duration {
	strategy := backoff.New(8*time.Second, time.Second)
	delay := time.Second
	for attempt := 0; attempt < max(attempts, 1); attempt++ {
		delay = strategy.Duration()
	}
	// Full jitter can yield zero. Keep a small floor so a temporarily missing
	// release asset cannot cause a tight retry loop.
	return max(delay, 100*time.Millisecond)
}

func temporaryLPKInspectionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "context deadline") || strings.Contains(message, "timeout") || strings.Contains(message, "connection refused") || strings.Contains(message, "no such host") || strings.Contains(message, "network is unreachable") {
		return true
	}
	for _, code := range []string{"404", "408", "425", "429", "500", "502", "503", "504"} {
		if strings.Contains(message, "status "+code) || strings.Contains(message, "http "+code) {
			return true
		}
	}
	return false
}

type lpkInspectionStatusDTO struct {
	ID          int        `json:"id"`
	State       string     `json:"state"`
	Trigger     string     `json:"trigger"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"lastError,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

func toLPKInspectionStatusDTO(job *entgo.LPKInspectionJob) lpkInspectionStatusDTO {
	dto := lpkInspectionStatusDTO{
		ID: job.ID, State: string(job.State), Trigger: string(job.Trigger), Attempts: job.Attempts, UpdatedAt: job.UpdatedAt, CompletedAt: job.CompletedAt,
	}
	if job.LastError != nil {
		dto.LastError = *job.LastError
	}
	return dto
}

func (s *Server) latestLPKInspectionStatus(ctx context.Context, appID int) (*lpkInspectionStatusDTO, error) {
	job, err := s.db.LPKInspectionJob.Query().
		Where(lpkinspectionjob.AppIDEQ(appID)).
		Order(entgo.Desc(lpkinspectionjob.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		return nil, err
	}
	dto := toLPKInspectionStatusDTO(job)
	return &dto, nil
}
