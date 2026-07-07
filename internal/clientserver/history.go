package clientserver

import (
	"context"
	"net/http"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientinstallhistory"
	"lazycat.community/appstore/internal/pagination"
)

func (s *Server) handleInstallHistory(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	page := pagination.FromRequest(r, s.clientDefaultPageSize(r.Context(), userID, pagination.DefaultPageSize, 500), 500)
	query := s.db.ClientInstallHistory.Query().
		Where(clientinstallhistory.UserIDEQ(userID))
	total, err := query.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_LIST_FAILED", "Could not list install history")
		return
	}
	rows, err := query.Order(ent.Desc(clientinstallhistory.FieldCreatedAt)).
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
	return create.Exec(ctx)
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
