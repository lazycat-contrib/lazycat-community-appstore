package clientserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	query := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.HasSourceWith(clientsource.UserIDEQ(userID))).
		WithSource().
		Order(ent.Asc(clientsourceapp.FieldName))
	if sourceID := strings.TrimSpace(r.URL.Query().Get("sourceId")); sourceID != "" {
		id, err := strconv.Atoi(sourceID)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_SOURCE_ID", "Invalid source id")
			return
		}
		query = query.Where(clientsourceapp.SourceIDEQ(id))
	}
	if search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))); search != "" {
		query = query.Where(
			clientsourceapp.Or(
				clientsourceapp.PackageIDContainsFold(search),
				clientsourceapp.NameContainsFold(search),
				clientsourceapp.SlugContainsFold(search),
				clientsourceapp.SummaryContainsFold(search),
			),
		)
	}
	items, err := query.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps")
		return
	}
	out := make([]SourceAppDTO, 0, len(items))
	for _, item := range items {
		dto, err := sourceAppDTO(item)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not read app cache")
			return
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out})
}

func (s *Server) handleGetApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid app id")
		return
	}
	item, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.IDEQ(id), clientsourceapp.HasSourceWith(clientsource.UserIDEQ(currentUserID(r)))).
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
	dto, err := sourceAppDTO(item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LOAD_FAILED", "Could not read app cache")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": dto})
}

func (s *Server) handleGetAppVersions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid app id")
		return
	}
	item, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.IDEQ(id), clientsourceapp.HasSourceWith(clientsource.UserIDEQ(currentUserID(r)))).
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
	dto, err := sourceAppDTO(item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LOAD_FAILED", "Could not read app cache")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": dto.Versions})
}

func sourceAppDTO(app *ent.ClientSourceApp) (SourceAppDTO, error) {
	var version *VersionDTO
	versions := []VersionDTO{}
	if app.LatestVersionJSON != "" {
		version = &VersionDTO{}
		if err := json.Unmarshal([]byte(app.LatestVersionJSON), version); err != nil {
			return SourceAppDTO{}, err
		}
	}
	if app.VersionsJSON != "" {
		if err := json.Unmarshal([]byte(app.VersionsJSON), &versions); err != nil {
			return SourceAppDTO{}, err
		}
	}
	sourceName := ""
	if source, err := app.Edges.SourceOrErr(); err == nil {
		sourceName = source.Name
	}
	return SourceAppDTO{
		ID:               app.ID,
		SourceID:         app.SourceID,
		SourceName:       sourceName,
		PackageID:        app.PackageID,
		Name:             app.Name,
		Slug:             app.Slug,
		Summary:          app.Summary,
		Category:         app.Category,
		InstallProtected: app.InstallProtected,
		LatestVersion:    version,
		Versions:         versions,
	}, nil
}
