package clientserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/mirror"
)

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.ClientSource.Query().
		Where(clientsource.UserIDEQ(currentUserID(r))).
		Order(ent.Desc(clientsource.FieldUpdatedAt)).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_LIST_FAILED", "Could not list sources")
		return
	}
	out := make([]SourceDTO, 0, len(items))
	for _, item := range items {
		out = append(out, sourceDTO(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	input, ok := readSourceInput(w, r)
	if !ok {
		return
	}
	if input.DefaultDownloadMirrorID != "" || input.DefaultRawMirrorID != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Default mirror is not available until the source has been synced")
		return
	}
	chatEnabled := true
	if input.ChatEnabled != nil {
		chatEnabled = *input.ChatEnabled
	}
	created, err := s.db.ClientSource.Create().
		SetUserID(currentUserID(r)).
		SetName(input.Name).
		SetURL(input.URL).
		SetPassword(input.Password).
		SetGroupCodesJSON(encodeStringSlice(input.GroupCodes)).
		SetChatEnabled(chatEnabled).
		Save(r.Context())
	if err != nil {
		if ent.IsConstraintError(err) {
			writeError(w, http.StatusConflict, "SOURCE_EXISTS", "Source URL already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "SOURCE_CREATE_FAILED", "Could not create source")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"source": sourceDTO(created)})
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	input, ok := readSourceInput(w, r)
	if !ok {
		return
	}
	source, err := s.db.ClientSource.Query().
		Where(clientsource.IDEQ(id), clientsource.UserIDEQ(currentUserID(r))).
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "SOURCE_NOT_FOUND", "Source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "SOURCE_LOAD_FAILED", "Could not load source")
		return
	}
	if !sourceHasMirrorKind(source, input.DefaultDownloadMirrorID, mirror.KindDownload) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Default download mirror is not available from this source")
		return
	}
	if !sourceHasMirrorKind(source, input.DefaultRawMirrorID, mirror.KindRaw) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Default raw mirror is not available from this source")
		return
	}
	chatEnabled := source.ChatEnabled
	if input.ChatEnabled != nil {
		chatEnabled = *input.ChatEnabled
	}
	updated, err := s.db.ClientSource.UpdateOne(source).
		SetName(input.Name).
		SetURL(input.URL).
		SetPassword(input.Password).
		SetGroupCodesJSON(encodeStringSlice(input.GroupCodes)).
		SetGroupNamesJSON("").
		SetLastInvalidGroupCodesJSON("").
		SetDefaultDownloadMirrorID(input.DefaultDownloadMirrorID).
		SetDefaultRawMirrorID(input.DefaultRawMirrorID).
		SetChatEnabled(chatEnabled).
		Save(r.Context())
	if err != nil {
		if ent.IsConstraintError(err) {
			writeError(w, http.StatusConflict, "SOURCE_EXISTS", "Source URL already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "SOURCE_UPDATE_FAILED", "Could not update source")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": sourceDTO(updated)})
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	source, err := s.db.ClientSource.Query().
		Where(clientsource.IDEQ(id), clientsource.UserIDEQ(currentUserID(r))).
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "SOURCE_NOT_FOUND", "Source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "SOURCE_LOAD_FAILED", "Could not load source")
		return
	}
	if _, err := s.db.ClientSourceApp.Delete().Where(clientsourceapp.SourceIDEQ(source.ID)).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_DELETE_FAILED", "Could not delete source apps")
		return
	}
	if err := s.db.ClientSource.DeleteOne(source).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_DELETE_FAILED", "Could not delete source")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func readSourceInput(w http.ResponseWriter, r *http.Request) (SourceInput, bool) {
	var input SourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return SourceInput{}, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = normalizeSourceURL(input.URL)
	input.Password = strings.TrimSpace(input.Password)
	input.DefaultDownloadMirrorID = strings.TrimSpace(input.DefaultDownloadMirrorID)
	input.DefaultRawMirrorID = strings.TrimSpace(input.DefaultRawMirrorID)
	input.GroupCodes = normalizeGroupCodes(input.GroupCodes)
	if input.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Source name is required")
		return SourceInput{}, false
	}
	if input.URL == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Valid source URL is required")
		return SourceInput{}, false
	}
	return input, true
}

func normalizeSourceURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid source id")
		return 0, false
	}
	return id, true
}

func sourceDTO(source *ent.ClientSource) SourceDTO {
	mirrors := sourceMirrors(source)
	dto := SourceDTO{
		ID:                      source.ID,
		Name:                    source.Name,
		URL:                     source.URL,
		Password:                source.Password,
		DefaultDownloadMirrorID: source.DefaultDownloadMirrorID,
		DefaultRawMirrorID:      source.DefaultRawMirrorID,
		GroupCodes:              decodeStringSlice(source.GroupCodesJSON),
		Groups:                  decodeSourceGroups(source.GroupNamesJSON),
		Categories:              decodeSourceCategories(source.CategoriesJSON),
		Announcements:           decodeSourceAnnouncements(source.AnnouncementsJSON),
		ClientPolicy:            normalizeSourceClientPolicy(SourceClientPolicyDTO{MinVersion: source.MinClientVersion, Message: source.MinClientVersionMessage}),
		LastInvalidGroupCodes:   decodeStringSlice(source.LastInvalidGroupCodesJSON),
		GitHubMirrors:           mirrors,
		ChatAvailable:           source.ChatAvailable,
		ChatEnabled:             source.ChatEnabled,
		LastSync:                source.LastSync,
		LastAppCount:            source.LastAppCount,
		LastInstallableCount:    source.LastInstallableCount,
	}
	if source.LastError != nil {
		dto.LastError = *source.LastError
	}
	if source.LastErrorCode != nil {
		dto.LastErrorCode = string(*source.LastErrorCode)
	}
	return dto
}

func sourceHasMirrorKind(source *ent.ClientSource, id string, kind string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return true
	}
	entry, ok := mirror.Find(sourceMirrors(source), id)
	return ok && entry.Kind == kind
}

func sourceMirrors(source *ent.ClientSource) []mirror.Entry {
	mirrors := []mirror.Entry{}
	if source.MirrorsJSON != "" {
		_ = json.Unmarshal([]byte(source.MirrorsJSON), &mirrors)
	}
	return mirrors
}
