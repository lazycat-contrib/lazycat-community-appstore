package clientserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"lazycat.community/appstore/ent/clientsetting"
)

const settingCommentDisplayName = "comment_display_name"

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"settings": s.clientSettings(r)})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var input ClientSettingsDTO
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	displayName := sanitizeClientSetting(input.CommentDisplayName, 40)
	if err := s.setClientSetting(r, settingCommentDisplayName, displayName); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_SAVE_FAILED", "Could not save settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": ClientSettingsDTO{CommentDisplayName: displayName}})
}

func (s *Server) clientSettings(r *http.Request) ClientSettingsDTO {
	return ClientSettingsDTO{CommentDisplayName: strings.TrimSpace(s.clientSetting(r, settingCommentDisplayName))}
}

func (s *Server) clientCommentDisplayName(r *http.Request) string {
	value := strings.TrimSpace(s.clientSetting(r, settingCommentDisplayName))
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

func (s *Server) clientSetting(r *http.Request, key string) string {
	record, err := s.db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(currentUserID(r)), clientsetting.KeyEQ(key)).
		Only(r.Context())
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
