package clientserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsetting"
	"lazycat.community/appstore/ent/clientsyncsetting"
)

const settingCommentDisplayName = "comment_display_name"

const (
	defaultAutoSyncIntervalMinutes = 60
	minAutoSyncIntervalMinutes     = 5
	maxAutoSyncIntervalMinutes     = 24 * 60
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
	var input ClientSettingsDTO
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	displayName := sanitizeClientSetting(input.CommentDisplayName, 40)
	userID := currentUserID(r)
	if err := s.setClientSetting(r, settingCommentDisplayName, displayName); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	syncSetting, err := s.setClientSyncSetting(r.Context(), userID, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": s.clientSettingsDTO(displayName, syncSetting)})
}

func (s *Server) clientSettings(ctx context.Context, userID string) (ClientSettingsDTO, error) {
	commentDisplayName := strings.TrimSpace(s.clientSetting(ctx, userID, settingCommentDisplayName))
	syncSetting, err := s.clientSyncSetting(ctx, userID)
	if err != nil {
		return ClientSettingsDTO{}, err
	}
	return s.clientSettingsDTO(commentDisplayName, syncSetting), nil
}

func (s *Server) clientSettingsDTO(commentDisplayName string, syncSetting *ent.ClientSyncSetting) ClientSettingsDTO {
	dto := ClientSettingsDTO{
		CommentDisplayName:      commentDisplayName,
		AutoSyncIntervalMinutes: defaultAutoSyncIntervalMinutes,
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
	return dto
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
		return "懒猫用户"
	}
	return "LazyCat user"
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

func (s *Server) setClientSyncSetting(ctx context.Context, userID string, input ClientSettingsDTO) (*ent.ClientSyncSetting, error) {
	interval := sanitizeAutoSyncInterval(input.AutoSyncIntervalMinutes)
	record, err := s.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.UserIDEQ(userID)).
		Only(ctx)
	if err == nil {
		return s.db.ClientSyncSetting.UpdateOneID(record.ID).
			SetAutoSyncEnabled(input.AutoSyncEnabled).
			SetAutoSyncIntervalMinutes(interval).
			SetSyncOnStartup(input.SyncOnStartup).
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
