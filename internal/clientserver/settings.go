package clientserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsetting"
	"lazycat.community/appstore/ent/clientsyncsetting"
	"lazycat.community/appstore/internal/pagination"
)

const (
	settingClientTitle                  = "client_title"
	settingCommentDisplayName           = "comment_display_name"
	settingDefaultPageSize              = "default_page_size"
	settingInstallSuccessDismissSeconds = "install_success_dismiss_seconds"
)

const (
	defaultAutoSyncIntervalMinutes = 60
	minAutoSyncIntervalMinutes     = 5
	maxAutoSyncIntervalMinutes     = 24 * 60

	defaultInstallSuccessDismissSeconds = 3
	maxInstallSuccessDismissSeconds     = 60
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.clientSettings(r.Context(), currentUserID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_LOAD_FAILED", "Could not load settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var input ClientSettingsUpdateDTO
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	clientTitle := sanitizeClientSetting(input.ClientTitle, 80)
	displayName := sanitizeClientSetting(input.CommentDisplayName, 40)
	userID := currentUserID(r)
	if err := s.setClientSetting(r, settingClientTitle, clientTitle); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	if err := s.setClientSetting(r, settingCommentDisplayName, displayName); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	defaultPageSize := pagination.ClampPageSize(input.DefaultPageSize, pagination.DefaultPageSize, 100)
	if err := s.setClientSetting(r, settingDefaultPageSize, strconv.Itoa(defaultPageSize)); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	installSuccessDismissSeconds := s.clientInstallSuccessDismissSeconds(r.Context(), userID)
	if input.InstallSuccessDismissSeconds != nil {
		installSuccessDismissSeconds = sanitizeInstallSuccessDismissSeconds(*input.InstallSuccessDismissSeconds)
	}
	if err := s.setClientSetting(r, settingInstallSuccessDismissSeconds, strconv.Itoa(installSuccessDismissSeconds)); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	syncSetting, err := s.setClientSyncSetting(r.Context(), userID, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": s.clientSettingsDTO(clientTitle, displayName, defaultPageSize, installSuccessDismissSeconds, syncSetting)})
}

func (s *Server) clientSettings(ctx context.Context, userID string) (ClientSettingsDTO, error) {
	clientTitle := strings.TrimSpace(s.clientSetting(ctx, userID, settingClientTitle))
	commentDisplayName := strings.TrimSpace(s.clientSetting(ctx, userID, settingCommentDisplayName))
	defaultPageSize := s.clientDefaultPageSize(ctx, userID, pagination.DefaultPageSize, 100)
	installSuccessDismissSeconds := s.clientInstallSuccessDismissSeconds(ctx, userID)
	syncSetting, err := s.clientSyncSetting(ctx, userID)
	if err != nil {
		return ClientSettingsDTO{}, err
	}
	return s.clientSettingsDTO(clientTitle, commentDisplayName, defaultPageSize, installSuccessDismissSeconds, syncSetting), nil
}

func (s *Server) clientSettingsDTO(clientTitle string, commentDisplayName string, defaultPageSize int, installSuccessDismissSeconds int, syncSetting *ent.ClientSyncSetting) ClientSettingsDTO {
	dto := ClientSettingsDTO{
		ClientTitle:                  clientTitle,
		CommentDisplayName:           commentDisplayName,
		DefaultPageSize:              pagination.ClampPageSize(defaultPageSize, pagination.DefaultPageSize, 100),
		AutoSyncIntervalMinutes:      defaultAutoSyncIntervalMinutes,
		AutoUpdateIntervalMinutes:    defaultAutoSyncIntervalMinutes,
		InstallSuccessDismissSeconds: sanitizeInstallSuccessDismissSeconds(installSuccessDismissSeconds),
	}
	if syncSetting == nil {
		return dto
	}
	dto.AutoSyncEnabled = syncSetting.AutoSyncEnabled
	dto.AutoSyncIntervalMinutes = sanitizeAutoSyncInterval(syncSetting.AutoSyncIntervalMinutes)
	dto.SyncOnStartup = syncSetting.SyncOnStartup
	dto.LastAutoSyncAt = syncSetting.LastAutoSyncAt
	if syncSetting.LastAutoSyncStatus != nil {
		dto.LastAutoSyncStatus = syncSetting.LastAutoSyncStatus.String()
	}
	if syncSetting.LastAutoSyncError != nil {
		dto.LastAutoSyncError = *syncSetting.LastAutoSyncError
	}
	dto.AutoUpdateEnabled = syncSetting.AutoUpdateEnabled
	dto.AutoUpdateIntervalMinutes = sanitizeAutoSyncInterval(syncSetting.AutoUpdateIntervalMinutes)
	dto.LastAutoUpdateAt = syncSetting.LastAutoUpdateAt
	if syncSetting.LastAutoUpdateStatus != nil {
		dto.LastAutoUpdateStatus = syncSetting.LastAutoUpdateStatus.String()
	}
	if syncSetting.LastAutoUpdateError != nil {
		dto.LastAutoUpdateError = *syncSetting.LastAutoUpdateError
	}
	return dto
}

func (s *Server) clientInstallSuccessDismissSeconds(ctx context.Context, userID string) int {
	raw := strings.TrimSpace(s.clientSetting(ctx, userID, settingInstallSuccessDismissSeconds))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultInstallSuccessDismissSeconds
	}
	return sanitizeInstallSuccessDismissSeconds(value)
}

func (s *Server) clientDefaultPageSize(ctx context.Context, userID string, fallback, maxPageSize int) int {
	raw := strings.TrimSpace(s.clientSetting(ctx, userID, settingDefaultPageSize))
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = fallback
	}
	return pagination.ClampPageSize(value, fallback, maxPageSize)
}

func (s *Server) clientCommentDisplayName(r *http.Request) string {
	value := strings.TrimSpace(s.clientSetting(r.Context(), currentUserID(r), settingCommentDisplayName))
	if value == "" {
		return defaultClientCommentDisplayName(r)
	}
	return value
}

func defaultClientCommentDisplayName(r *http.Request) string {
	language := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.HasPrefix(language, "zh") || strings.Contains(language, "zh-") || strings.Contains(language, "zh_") {
		return "喵喵用户"
	}
	return "MiaoMiao user"
}

func (s *Server) clientSetting(ctx context.Context, userID, key string) string {
	record, err := s.db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(userID), clientsetting.KeyEQ(key)).
		Only(ctx)
	if err != nil {
		return ""
	}
	return record.Value
}

func (s *Server) setClientSetting(r *http.Request, key, value string) error {
	userID := currentUserID(r)
	record, err := s.db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(userID), clientsetting.KeyEQ(key)).
		Only(r.Context())
	if err == nil {
		_, err = s.db.ClientSetting.UpdateOneID(record.ID).SetValue(value).Save(r.Context())
		return err
	}
	_, err = s.db.ClientSetting.Create().SetUserID(userID).SetKey(key).SetValue(value).Save(r.Context())
	return err
}

func (s *Server) clientSyncSetting(ctx context.Context, userID string) (*ent.ClientSyncSetting, error) {
	record, err := s.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.UserIDEQ(userID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return record, err
}

func (s *Server) setClientSyncSetting(ctx context.Context, userID string, input ClientSettingsUpdateDTO) (*ent.ClientSyncSetting, error) {
	interval := sanitizeAutoSyncInterval(input.AutoSyncIntervalMinutes)
	record, err := s.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.UserIDEQ(userID)).
		Only(ctx)
	if err == nil {
		return s.db.ClientSyncSetting.UpdateOneID(record.ID).
			SetAutoSyncEnabled(input.AutoSyncEnabled).
			SetAutoSyncIntervalMinutes(interval).
			SetSyncOnStartup(input.SyncOnStartup).
			SetAutoUpdateEnabled(input.AutoUpdateEnabled).
			SetAutoUpdateIntervalMinutes(sanitizeAutoSyncInterval(input.AutoUpdateIntervalMinutes)).
			Save(ctx)
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	return s.db.ClientSyncSetting.Create().
		SetUserID(userID).
		SetAutoSyncEnabled(input.AutoSyncEnabled).
		SetAutoSyncIntervalMinutes(interval).
		SetSyncOnStartup(input.SyncOnStartup).
		SetAutoUpdateEnabled(input.AutoUpdateEnabled).
		SetAutoUpdateIntervalMinutes(sanitizeAutoSyncInterval(input.AutoUpdateIntervalMinutes)).
		Save(ctx)
}

func sanitizeAutoSyncInterval(value int) int {
	if value <= 0 {
		return defaultAutoSyncIntervalMinutes
	}
	if value < minAutoSyncIntervalMinutes {
		return minAutoSyncIntervalMinutes
	}
	if value > maxAutoSyncIntervalMinutes {
		return maxAutoSyncIntervalMinutes
	}
	return value
}

func sanitizeInstallSuccessDismissSeconds(value int) int {
	if value < 0 {
		return defaultInstallSuccessDismissSeconds
	}
	if value > maxInstallSuccessDismissSeconds {
		return maxInstallSuccessDismissSeconds
	}
	return value
}

func sanitizeClientSetting(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
