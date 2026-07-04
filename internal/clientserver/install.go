package clientserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	app, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.IDEQ(input.AppID), clientsourceapp.HasSourceWith(clientsource.UserIDEQ(currentUserID(r)))).
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
	if dto.LatestVersion == nil || dto.LatestVersion.DownloadURL == "" {
		writeError(w, http.StatusUnprocessableEntity, "NO_INSTALLABLE_VERSION", "App has no installable version")
		return
	}
	installReq := InstallRequestDTO{
		AppID:       dto.ID,
		Name:        dto.Name,
		PackageID:   dto.Slug,
		DownloadURL: withInstallPassword(dto.LatestVersion.DownloadURL, input.InstallPassword),
		SHA256:      dto.LatestVersion.SHA256,
	}
	result, err := s.pkg.InstallLPK(r.Context(), currentUserID(r), installReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "INSTALL_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
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
