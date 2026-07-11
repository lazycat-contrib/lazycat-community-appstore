package clientserver

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientinstallhistory"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/mirror"
)

func (s *Server) handleInstalled(w http.ResponseWriter, r *http.Request) {
	apps, err := s.pkg.QueryInstalled(r.Context(), currentUserID(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, "LAZYCAT_SDK_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	var input InstallRequestDTO
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	app, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.IDEQ(input.AppID), clientsourceapp.HasSourceWith(clientsource.UserIDEQ(currentUserID(r)))).
		WithSource().
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "APP_LOAD_FAILED", "Could not load app")
		return
	}
	dto, err := sourceAppDTO(app)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LOAD_FAILED", "Could not read app cache")
		return
	}
	selected, err := selectInstallVersion(dto, input.Version)
	if err != nil {
		_ = s.recordInstallHistory(r.Context(), currentUserID(r), app, dto, nil, clientinstallhistory.ResultFAILED, err.Error())
		writeError(w, http.StatusUnprocessableEntity, "VERSION_NOT_FOUND", err.Error())
		return
	}
	if selected == nil || selected.DownloadURL == "" {
		_ = s.recordInstallHistory(r.Context(), currentUserID(r), app, dto, selected, clientinstallhistory.ResultFAILED, "App has no installable version")
		writeError(w, http.StatusUnprocessableEntity, "NO_INSTALLABLE_VERSION", "App has no installable version")
		return
	}
	downloadURL, err := s.installDownloadURL(app, selected, input)
	if err != nil {
		_ = s.recordInstallHistory(r.Context(), currentUserID(r), app, dto, selected, clientinstallhistory.ResultFAILED, err.Error())
		writeError(w, http.StatusUnprocessableEntity, "MIRROR_NOT_AVAILABLE", err.Error())
		return
	}
	installReq := InstallRequestDTO{
		AppID:       dto.ID,
		Version:     selected.Version,
		Name:        dto.Name,
		PackageID:   dto.PackageID,
		DownloadURL: withInstallPassword(downloadURL, input.InstallPassword),
		SHA256:      selected.SHA256,
	}
	operation, started := s.installCoordinator.begin(currentUserID(r), installOperationManual)
	if !started {
		writeError(w, http.StatusConflict, "INSTALL_IN_PROGRESS", "An installation is already running")
		return
	}
	taskStarted := false
	defer func() {
		if !taskStarted {
			s.installCoordinator.release(currentUserID(r), operation)
		}
	}()
	result, err := s.pkg.InstallLPK(r.Context(), currentUserID(r), installReq)
	if err != nil {
		_ = s.recordInstallHistory(r.Context(), currentUserID(r), app, dto, selected, clientinstallhistory.ResultFAILED, err.Error())
		writeError(w, http.StatusBadGateway, "INSTALL_FAILED", err.Error())
		return
	}
	if result.TaskID == "" {
		writeError(w, http.StatusBadGateway, "INSTALL_TASK_MISSING", "LazyCat did not return an install task")
		return
	}
	s.installCoordinator.setTask(currentUserID(r), operation, result.TaskID)
	taskStarted = true
	writeJSON(w, http.StatusAccepted, map[string]any{"task": InstallTaskDTO{TaskID: result.TaskID, Status: result.Status, Detail: result.Detail}})
}

func (s *Server) handleGetInstallTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.PathValue("taskId"))
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_TASK_ID", "Task ID is required")
		return
	}
	task, err := s.pkg.GetInstallTask(r.Context(), currentUserID(r), taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "INSTALL_TASK_NOT_FOUND", "Install task not found")
		return
	}
	if installTaskTerminal(task.Status) {
		s.installCoordinator.releaseTask(currentUserID(r), taskID, installOperationManual)
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task})
}

func (s *Server) handleCancelInstallTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.PathValue("taskId"))
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_TASK_ID", "Task ID is required")
		return
	}
	if err := s.pkg.CancelInstall(r.Context(), currentUserID(r), taskID); err != nil {
		writeError(w, http.StatusNotFound, "INSTALL_TASK_NOT_FOUND", "Install task not found")
		return
	}
	s.installCoordinator.releaseTask(currentUserID(r), taskID, installOperationManual)
	writeJSON(w, http.StatusOK, map[string]any{"taskId": taskID, "status": "CANCELLED"})
}

func (s *Server) installDownloadURL(app *ent.ClientSourceApp, version *VersionDTO, input InstallRequestDTO) (string, error) {
	source, err := app.Edges.SourceOrErr()
	if err != nil {
		return "", errors.New("source was not loaded")
	}
	mirrorID := strings.TrimSpace(input.MirrorID)
	if mirrorID == "" {
		return withGroupCodes(version.DownloadURL, decodeStringSlice(source.GroupCodesJSON)), nil
	}
	upstream := strings.TrimSpace(version.UpstreamDownloadURL)
	if upstream == "" {
		upstream = strings.TrimSpace(version.DownloadURL)
	}
	if !mirror.IsGitHubURL(upstream) {
		return "", errors.New("selected mirror can only be used with GitHub downloads")
	}
	entry, ok := mirror.FindApplicable(sourceMirrors(source), mirrorID, upstream)
	if !ok {
		return "", errors.New("selected mirror is not available for this download")
	}
	return withGroupCodes(mirror.RewriteGitHub(upstream, entry), decodeStringSlice(source.GroupCodesJSON)), nil
}

func selectInstallVersion(app SourceAppDTO, wanted string) (*VersionDTO, error) {
	wanted = strings.TrimSpace(wanted)
	if wanted == "" {
		return app.LatestVersion, nil
	}
	for i := range app.Versions {
		if app.Versions[i].Version == wanted {
			return &app.Versions[i], nil
		}
	}
	if app.LatestVersion != nil && app.LatestVersion.Version == wanted {
		return app.LatestVersion, nil
	}
	return nil, errors.New("requested version is not available from this source")
}

func withInstallPassword(rawURL string, password string) string {
	if password == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		return rawURL + separator + "installPassword=" + url.QueryEscape(password)
	}
	q := parsed.Query()
	q.Set("installPassword", password)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
