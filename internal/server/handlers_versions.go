package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
)

type updateVersionRetentionRequest struct {
	Mode           string `json:"mode"`
	MaxVersions    *int   `json:"maxVersions"`
	maxVersionsSet bool
}

func (s *Server) handleDeleteVersion(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	versionID, err := strconv.Atoi(r.PathValue("versionId"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.canUploadVersion(r, record, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to delete this version", nil)
		return
	}
	deleted, err := s.deleteAppVersion(r.Context(), appID, versionID)
	if err != nil {
		if entgo.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", "Version not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "VERSION_DELETE_FAILED", "Could not delete version", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deletedVersion": deleted})
}

func (input *updateVersionRetentionRequest) UnmarshalJSON(data []byte) error {
	type requestAlias updateVersionRetentionRequest
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var decoded requestAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*input = updateVersionRetentionRequest(decoded)
	_, input.maxVersionsSet = raw["maxVersions"]
	return nil
}

func (s *Server) handleUpdateVersionRetention(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && record.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the app owner or an administrator can change version retention", nil)
		return
	}
	var input updateVersionRetentionRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.Mode = strings.ToUpper(strings.TrimSpace(input.Mode))
	var maxVersions *int
	switch input.Mode {
	case "INHERIT":
		if input.maxVersionsSet {
			badRequest(w, errors.New("maxVersions must be omitted when mode is INHERIT"))
			return
		}
	case "CUSTOM":
		if !input.maxVersionsSet || input.MaxVersions == nil || *input.MaxVersions < 0 {
			badRequest(w, errors.New("maxVersions must be a non-negative integer when mode is CUSTOM"))
			return
		}
		maxVersions = input.MaxVersions
	default:
		badRequest(w, errors.New("mode must be INHERIT or CUSTOM"))
		return
	}
	policy, prunedVersions, err := s.updateAppVersionRetention(r.Context(), appID, maxVersions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VERSION_RETENTION_UPDATE_FAILED", "Could not update version retention", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"versionRetention": policy,
		"prunedVersions":   prunedVersions,
	})
}
