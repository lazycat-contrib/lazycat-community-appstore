package clientserver

import (
	"context"
	"fmt"
	"lazycat.community/appstore/ent/clientsetting"
	"net/http"
	"strconv"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientinstallhistory"
	"lazycat.community/appstore/internal/pagination"
)

func (s *Server) handleInstallHistory(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	if _, err := s.pruneInstallHistory(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_PRUNE_FAILED", "Could not apply history retention")
		return
	}
	page := pagination.FromRequest(r, s.clientDefaultPageSize(r.Context(), userID, pagination.DefaultPageSize, 500), 500)
	query := s.db.ClientInstallHistory.Query().
		Where(clientinstallhistory.UserIDEQ(userID))
	total, err := query.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_LIST_FAILED", "Could not list install history")
		return
	}
	rows, err := query.Order(ent.Desc(clientinstallhistory.FieldCreatedAt), ent.Desc(clientinstallhistory.FieldID)).
		Offset(page.Offset()).
		Limit(page.PageSize).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_LIST_FAILED", "Could not list install history")
		return
	}
	out := make([]InstallHistoryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, installHistoryDTO(row))
	}
	writeJSON(w, http.StatusOK, pagination.NewHistoryPage(out, page, total))
}

func (s *Server) recordInstallHistory(ctx context.Context, userID string, app *ent.ClientSourceApp, dto SourceAppDTO, version *VersionDTO, result clientinstallhistory.Result, errorMessage string) error {
	create := s.db.ClientInstallHistory.Create().
		SetUserID(userID).
		SetSourceID(app.SourceID).
		SetSourceAppID(app.ID).
		SetSourceName(dto.SourceName).
		SetPackageID(dto.PackageID).
		SetAppName(dto.Name).
		SetResult(result)
	if version != nil {
		create.
			SetVersion(version.Version).
			SetDownloadURL(version.DownloadURL).
			SetSha256(version.SHA256)
	}
	if errorMessage != "" {
		create.SetError(errorMessage)
	}
	if err := create.Exec(ctx); err != nil {
		return err
	}
	_, err := s.pruneInstallHistory(ctx, userID)
	return err
}

func installHistoryDTO(row *ent.ClientInstallHistory) InstallHistoryDTO {
	return InstallHistoryDTO{
		ID:          row.ID,
		SourceID:    row.SourceID,
		SourceAppID: row.SourceAppID,
		SourceName:  row.SourceName,
		PackageID:   row.PackageID,
		AppName:     row.AppName,
		Version:     row.Version,
		Result:      string(row.Result),
		DownloadURL: row.DownloadURL,
		SHA256:      row.Sha256,
		Error:       row.Error,
		CreatedAt:   row.CreatedAt,
	}
}

const (
	settingHistoryMaxEntries    = "history_max_entries"
	settingHistoryRetentionDays = "history_retention_days"
)

func (s *Server) applyHistoryRetentionSettings(ctx context.Context, userID string, dto *ClientSettingsDTO) error {
	var err error
	dto.HistoryMaxEntries, err = s.historyRetentionValue(ctx, userID, settingHistoryMaxEntries, 500, 10000)
	if err != nil {
		return err
	}
	dto.HistoryRetentionDays, err = s.historyRetentionValue(ctx, userID, settingHistoryRetentionDays, 90, 3650)
	return err
}

func (s *Server) historyRetentionValue(ctx context.Context, userID, key string, fallback, maximum int) (int, error) {
	record, err := s.db.ClientSetting.Query().Where(clientsetting.UserIDEQ(userID), clientsetting.KeyEQ(key)).Only(ctx)
	if ent.IsNotFound(err) {
		return fallback, nil
	}
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(record.Value)
	if err != nil || value < 0 || value > maximum {
		return 0, fmt.Errorf("invalid history retention setting %s", key)
	}
	return value, nil
}

func (s *Server) handlePruneInstallHistory(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.pruneInstallHistory(r.Context(), currentUserID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_PRUNE_FAILED", "Could not clean install history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}

func (s *Server) pruneInstallHistory(ctx context.Context, userID string) (int, error) {
	var settings ClientSettingsDTO
	if err := s.applyHistoryRetentionSettings(ctx, userID, &settings); err != nil {
		return 0, err
	}
	deleted := 0
	if settings.HistoryRetentionDays > 0 {
		count, err := s.db.ClientInstallHistory.Delete().Where(clientinstallhistory.UserIDEQ(userID), clientinstallhistory.CreatedAtLT(time.Now().AddDate(0, 0, -settings.HistoryRetentionDays))).Exec(ctx)
		if err != nil {
			return deleted, err
		}
		deleted += count
	}
	if settings.HistoryMaxEntries == 0 {
		return deleted, nil
	}
	for {
		ids, err := s.db.ClientInstallHistory.Query().Where(clientinstallhistory.UserIDEQ(userID)).Order(ent.Desc(clientinstallhistory.FieldCreatedAt), ent.Desc(clientinstallhistory.FieldID)).Offset(settings.HistoryMaxEntries).Limit(500).IDs(ctx)
		if err != nil || len(ids) == 0 {
			return deleted, err
		}
		count, err := s.db.ClientInstallHistory.Delete().Where(clientinstallhistory.UserIDEQ(userID), clientinstallhistory.IDIn(ids...)).Exec(ctx)
		if err != nil {
			return deleted, err
		}
		deleted += count
	}
}
