package clientserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientinstallhistory"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/mirror"
)

type updateCandidate struct {
	App              *ent.ClientSourceApp
	PackageID        string
	InstalledVersion string
	Version          VersionDTO
}

type installOperationKind string

const (
	installOperationManual installOperationKind = "manual"
	installOperationQueue  installOperationKind = "queue"
)

type installOperation struct {
	kind      installOperationKind
	taskID    string
	cancelled bool
	result    UpdateQueueResultDTO
}

// installCoordinator serializes all LazyCat installs for a client user. It keeps
// only live task metadata in memory; LazyCat remains the source of truth for the
// actual installation state.
type installCoordinator struct {
	mu         sync.Mutex
	operations map[string]*installOperation
}

func newInstallCoordinator() *installCoordinator {
	return &installCoordinator{operations: make(map[string]*installOperation)}
}

func (c *installCoordinator) begin(userID string, kind installOperationKind) (*installOperation, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.operations[userID]; exists {
		return nil, false
	}
	operation := &installOperation{kind: kind, result: UpdateQueueResultDTO{Status: "running"}}
	c.operations[userID] = operation
	return operation, true
}

func cloneUpdateQueueResult(result UpdateQueueResultDTO) UpdateQueueResultDTO {
	cloned := result
	cloned.Items = append([]UpdateQueueItemDTO(nil), result.Items...)
	return cloned
}

func (c *installCoordinator) publish(userID string, operation *installOperation, result UpdateQueueResultDTO) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operations[userID] == operation {
		operation.result = cloneUpdateQueueResult(result)
	}
}

func (c *installCoordinator) queueSnapshot(userID string) (UpdateQueueResultDTO, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	operation := c.operations[userID]
	if operation == nil || operation.kind != installOperationQueue {
		return UpdateQueueResultDTO{}, false
	}
	return cloneUpdateQueueResult(operation.result), true
}

func (c *installCoordinator) release(userID string, operation *installOperation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operations[userID] == operation {
		delete(c.operations, userID)
	}
}

func (c *installCoordinator) releaseTask(userID, taskID string, kind installOperationKind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	operation := c.operations[userID]
	if operation != nil && operation.kind == kind && operation.taskID == taskID {
		delete(c.operations, userID)
	}
}

func (c *installCoordinator) setTask(userID string, operation *installOperation, taskID string) (cancelled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operations[userID] != operation {
		return true
	}
	operation.taskID = taskID
	return operation.cancelled
}

func (c *installCoordinator) clearTask(userID string, operation *installOperation, taskID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operations[userID] == operation && operation.taskID == taskID {
		operation.taskID = ""
	}
}

func (c *installCoordinator) isCancelled(userID string, operation *installOperation) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.operations[userID] == operation && operation.cancelled
}

func (c *installCoordinator) cancelQueue(userID string) (taskID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	operation := c.operations[userID]
	if operation == nil || operation.kind != installOperationQueue {
		return "", errors.New("update queue is not running")
	}
	operation.cancelled = true
	return operation.taskID, nil
}

func eligibleUpdates(installed []InstalledApplicationDTO, apps []*ent.ClientSourceApp) []updateCandidate {
	installedByPackage := make(map[string]InstalledApplicationDTO, len(installed))
	for _, item := range installed {
		packageID := strings.ToLower(strings.TrimSpace(item.AppID))
		if packageID == "" || strings.TrimSpace(item.Version) == "" {
			continue
		}
		installedByPackage[packageID] = item
	}

	candidates := make([]updateCandidate, 0)
	for _, app := range apps {
		if app == nil || app.InstallProtected {
			continue
		}
		packageID := strings.TrimSpace(app.PackageID)
		installed, ok := installedByPackage[strings.ToLower(packageID)]
		if !ok {
			continue
		}
		latest, ok := cachedLatestVersion(app)
		if !ok || strings.TrimSpace(latest.Version) == "" || strings.TrimSpace(latest.DownloadURL) == "" {
			continue
		}
		if compareUpdateVersions(installed.Version, latest.Version) >= 0 {
			continue
		}
		candidates = append(candidates, updateCandidate{
			App:              app,
			PackageID:        packageID,
			InstalledVersion: installed.Version,
			Version:          latest,
		})
	}
	return candidates
}

func cachedLatestVersion(app *ent.ClientSourceApp) (VersionDTO, bool) {
	if app == nil || strings.TrimSpace(app.LatestVersionJSON) == "" {
		return VersionDTO{}, false
	}
	var version VersionDTO
	if err := json.Unmarshal([]byte(app.LatestVersionJSON), &version); err != nil {
		return VersionDTO{}, false
	}
	return version, true
}

func compareUpdateVersions(left, right string) int {
	leftParts, leftOK := numericVersionParts(left)
	rightParts, rightOK := numericVersionParts(right)
	if !leftOK || !rightOK {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	length := len(leftParts)
	if len(rightParts) > length {
		length = len(rightParts)
	}
	for index := 0; index < length; index++ {
		var leftPart, rightPart int
		if index < len(leftParts) {
			leftPart = leftParts[index]
		}
		if index < len(rightParts) {
			rightPart = rightParts[index]
		}
		if leftPart < rightPart {
			return -1
		}
		if leftPart > rightPart {
			return 1
		}
	}
	return 0
}

func numericVersionParts(value string) ([]int, bool) {
	value = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "v"), "V")
	if value == "" {
		return nil, false
	}
	main, _, _ := strings.Cut(value, "-")
	main, _, _ = strings.Cut(main, "+")
	parts := strings.Split(main, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return nil, false
		}
		out = append(out, parsed)
	}
	return out, true
}

func (s *Server) RunUpdateQueue(ctx context.Context, userID string) UpdateQueueResultDTO {
	return s.RunUpdateQueueWithOptions(ctx, userID, UpdateQueueRequestDTO{})
}

func (s *Server) RunUpdateQueueWithOptions(ctx context.Context, userID string, options UpdateQueueRequestDTO) UpdateQueueResultDTO {
	operation, started := s.installCoordinator.begin(userID, installOperationQueue)
	if !started {
		return UpdateQueueResultDTO{Status: "already_running"}
	}
	defer s.installCoordinator.release(userID, operation)

	installed, err := s.pkg.QueryInstalled(ctx, userID)
	if err != nil {
		return UpdateQueueResultDTO{Status: "failed", Error: err.Error()}
	}
	apps, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.HasSourceWith(clientsource.UserIDEQ(userID))).
		WithSource().
		Order(ent.Asc(clientsourceapp.FieldName)).
		All(ctx)
	if err != nil {
		return UpdateQueueResultDTO{Status: "failed", Error: err.Error()}
	}
	candidates := eligibleUpdates(installed, apps)
	if len(candidates) == 0 {
		return UpdateQueueResultDTO{Status: "no_updates"}
	}

	result := UpdateQueueResultDTO{Status: "running", Items: make([]UpdateQueueItemDTO, len(candidates))}
	for index, candidate := range candidates {
		result.Items[index] = updateQueueItem(candidate, "queued", "")
	}
	s.installCoordinator.publish(userID, operation, result)
	overrides := make(map[int]UpdateQueueMirrorOverrideDTO, len(options.MirrorOverrides))
	for _, override := range options.MirrorOverrides {
		overrides[override.SourceID] = override
	}
	for index, candidate := range candidates {
		if s.installCoordinator.isCancelled(userID, operation) {
			result.Items[index] = updateQueueItem(candidate, "cancelled", "")
			continue
		}
		result.Items[index] = updateQueueItem(candidate, "running", "")
		s.installCoordinator.publish(userID, operation, result)
		result.Items[index] = s.installUpdateCandidate(ctx, userID, operation, candidate, overrides[candidate.App.SourceID], func(item UpdateQueueItemDTO) {
			result.Items[index] = item
			s.installCoordinator.publish(userID, operation, result)
		})
		s.installCoordinator.publish(userID, operation, result)
	}
	result.Status = updateQueueResultStatus(result.Items)
	s.installCoordinator.publish(userID, operation, result)
	return result
}

func (s *Server) CancelUpdateQueue(ctx context.Context, userID string) error {
	taskID, err := s.installCoordinator.cancelQueue(userID)
	if err != nil || taskID == "" {
		return err
	}
	return s.pkg.CancelInstall(ctx, userID, taskID)
}

func (s *Server) handleRunUpdateQueue(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	var input UpdateQueueRequestDTO
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
			return
		}
	}
	syncResult, err := s.syncAllSources(r.Context(), userID)
	if err != nil {
		writeError(w, 502, "SOURCE_SYNC_FAILED", "Could not sync application sources before updating")
		return
	}
	if syncResult.Success == 0 && syncResult.Failed > 0 {
		writeError(w, 502, "SOURCE_SYNC_FAILED", "All application source syncs failed")
		return
	}
	writeJSON(w, 200, s.RunUpdateQueueWithOptions(r.Context(), userID, input))
}

func (s *Server) handleGetUpdateQueue(w http.ResponseWriter, r *http.Request) {
	result, ok := s.installCoordinator.queueSnapshot(currentUserID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "UPDATE_QUEUE_NOT_RUNNING", "No application update queue is running")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCancelUpdateQueue(w http.ResponseWriter, r *http.Request) {
	if err := s.CancelUpdateQueue(r.Context(), currentUserID(r)); err != nil {
		writeError(w, 409, "UPDATE_QUEUE_NOT_RUNNING", "No application update queue is running")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "cancelling"})
}

func (s *Server) installUpdateCandidate(ctx context.Context, userID string, operation *installOperation, candidate updateCandidate, override UpdateQueueMirrorOverrideDTO, onProgress func(UpdateQueueItemDTO)) UpdateQueueItemDTO {
	if s.installCoordinator.isCancelled(userID, operation) {
		return updateQueueItem(candidate, "cancelled", "")
	}
	dto, err := sourceAppDTO(candidate.App)
	if err != nil {
		return s.recordUpdateFailure(ctx, userID, candidate, SourceAppDTO{}, err)
	}
	source, err := candidate.App.Edges.SourceOrErr()
	if err != nil {
		return s.recordUpdateFailure(ctx, userID, candidate, dto, err)
	}
	mirrorID := defaultUpdateMirrorID(source, candidate.Version)
	if override.SourceID == source.ID {
		upstream := strings.TrimSpace(candidate.Version.UpstreamDownloadURL)
		if upstream == "" {
			upstream = strings.TrimSpace(candidate.Version.DownloadURL)
		}
		if mirror.KindForURL(upstream) == mirror.KindRaw {
			mirrorID = strings.TrimSpace(override.RawMirrorID)
		} else if mirror.KindForURL(upstream) == mirror.KindDownload {
			mirrorID = strings.TrimSpace(override.DownloadMirrorID)
		}
	}
	downloadURL, err := s.installDownloadURL(candidate.App, &candidate.Version, InstallRequestDTO{MirrorID: mirrorID})
	if err != nil {
		return s.recordUpdateFailure(ctx, userID, candidate, dto, err)
	}
	installRequest := InstallRequestDTO{
		AppID:       candidate.App.ID,
		Version:     candidate.Version.Version,
		Name:        dto.Name,
		PackageID:   dto.PackageID,
		DownloadURL: downloadURL,
		SHA256:      candidate.Version.SHA256,
	}
	item := updateQueueItem(candidate, "running", "")
	onProgress(item)
	install, err := s.pkg.InstallLPK(ctx, userID, installRequest)
	if err != nil {
		return s.recordUpdateFailure(ctx, userID, candidate, dto, err)
	}
	item.Detail = install.Detail
	item.Status = "success"
	_ = s.recordInstallHistory(ctx, userID, candidate.App, dto, &candidate.Version, clientinstallhistory.ResultSUCCESS, "")
	return item
}

func defaultUpdateMirrorID(source *ent.ClientSource, version VersionDTO) string {
	upstream := strings.TrimSpace(version.UpstreamDownloadURL)
	if upstream == "" {
		upstream = strings.TrimSpace(version.DownloadURL)
	}
	switch mirror.KindForURL(upstream) {
	case mirror.KindRaw:
		return source.DefaultRawMirrorID
	case mirror.KindDownload:
		return source.DefaultDownloadMirrorID
	default:
		return ""
	}
}

func updateQueueItem(candidate updateCandidate, status, detail string) UpdateQueueItemDTO {
	source, _ := candidate.App.Edges.SourceOrErr()
	var sourceName string
	if source != nil {
		sourceName = source.Name
	}
	return UpdateQueueItemDTO{
		AppID:            candidate.App.ID,
		SourceID:         candidate.App.SourceID,
		SourceName:       sourceName,
		PackageID:        candidate.PackageID,
		AppName:          candidate.App.Name,
		InstalledVersion: candidate.InstalledVersion,
		Version:          candidate.Version.Version,
		Status:           status,
		Detail:           detail,
	}
}

func (s *Server) recordUpdateFailure(ctx context.Context, userID string, candidate updateCandidate, dto SourceAppDTO, err error) UpdateQueueItemDTO {
	message := err.Error()
	if dto.Name != "" {
		_ = s.recordInstallHistory(ctx, userID, candidate.App, dto, &candidate.Version, clientinstallhistory.ResultFAILED, message)
	}
	return updateQueueItem(candidate, "failed", message)
}

func updateQueueResultStatus(items []UpdateQueueItemDTO) string {
	var success, failed, cancelled int
	for _, item := range items {
		switch item.Status {
		case "success":
			success++
		case "failed":
			failed++
		case "cancelled":
			cancelled++
		}
	}
	switch {
	case cancelled > 0:
		return "cancelled"
	case failed > 0 && success > 0:
		return "partial"
	case failed > 0:
		return "failed"
	default:
		return "success"
	}
}

func installTaskSucceeded(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "INSTALL_OK", "SUCCEEDED", "SUCCESS":
		return true
	default:
		return false
	}
}

func installTaskCancelled(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CANCELLED", "CANCELED":
		return true
	default:
		return false
	}
}

func installTaskTerminal(status string) bool {
	status = strings.ToUpper(strings.TrimSpace(status))
	return installTaskSucceeded(status) || installTaskCancelled(status) || strings.HasSuffix(status, "_ERR") || status == "FAILED" || status == "ERROR"
}

func taskFailureDetail(task InstallTaskDTO) string {
	if detail := strings.TrimSpace(task.Detail); detail != "" {
		return detail
	}
	return "LazyCat installation failed"
}
