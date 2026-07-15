package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/internal/auth"
	"lazycat.community/appstore/internal/catalogmeta"
	"lazycat.community/appstore/internal/lazycatpkg"
)

type lazycatInstaller interface {
	InstallLPK(context.Context, lazycatpkg.Identity, lazycatpkg.InstallRequest) (lazycatpkg.InstallResult, error)
}

type lazycatSDKInstaller struct{}

func (lazycatSDKInstaller) InstallLPK(ctx context.Context, identity lazycatpkg.Identity, req lazycatpkg.InstallRequest) (lazycatpkg.InstallResult, error) {
	return lazycatpkg.InstallLPK(ctx, identity, req)
}

type installVersionInput struct {
	InstallPassword string `json:"installPassword"`
}

const maxLazyCatIdentityHeaderBytes = 256

func (s *Server) handleRuntimeCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	_, available := s.lazycatInstallIdentity(r)
	writeJSON(w, http.StatusOK, map[string]bool{"lazycatInstall": available})
}

func (s *Server) handleInstallVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	identity, ok := s.lazycatInstallIdentity(r)
	if !ok {
		writeError(w, http.StatusForbidden, "LAZYCAT_INSTALL_UNAVAILABLE", "Installation is only available from a trusted LazyCat client", nil)
		return
	}
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
	var input installVersionInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body", nil)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, s.optionalUser(r)) {
		allowed, accessErr := s.requestHasGroupCodeForApp(r, record.ID)
		if accessErr != nil {
			writeError(w, http.StatusInternalServerError, "GROUP_CODE_CHECK_FAILED", "Could not validate group code", nil)
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "GROUP_CODE_REQUIRED", "A valid group code is required for this app", nil)
			return
		}
	}
	versionRecord, err := s.db.AppVersion.Get(r.Context(), versionID)
	if err != nil || versionRecord.AppID != appID || versionRecord.Status != appversion.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", "Version not found", nil)
		return
	}
	if strings.TrimSpace(versionRecord.DownloadURL) == "" {
		writeError(w, http.StatusConflict, "VERSION_NOT_INSTALLABLE", "Version has no installable LPK", nil)
		return
	}
	if record.InstallPasswordHash != "" && (len(input.InstallPassword) > 256 || !auth.CheckPassword(record.InstallPasswordHash, input.InstallPassword)) {
		writeError(w, http.StatusUnauthorized, "INSTALL_PASSWORD_REQUIRED", "A valid install password is required", nil)
		return
	}
	if !s.beginLazyCatInstall() {
		writeError(w, http.StatusConflict, "INSTALL_IN_PROGRESS", "Another application installation is already running", nil)
		return
	}
	defer s.endLazyCatInstall()
	result, err := s.lazycatInstaller.InstallLPK(r.Context(), identity, lazycatpkg.InstallRequest{
		DownloadURL: s.absoluteURL(r.Context(), versionRecord.DownloadURL),
		SHA256:      strings.TrimSpace(versionRecord.Sha256),
		PackageID:   strings.TrimSpace(record.PackageID),
		Name:        catalogmeta.DecodeLocalizedText(record.NameI18nJSON).Fallback(record.Name),
	})
	if err != nil {
		slog.Warn("LazyCat storefront installation failed", "app_id", appID, "version_id", versionID, "error", err)
		writeError(w, http.StatusBadGateway, "LAZYCAT_INSTALL_FAILED", "Could not install the application on this LazyCat device", nil)
		return
	}
	if err := s.recordAppDownload(r.Context(), appID, versionRecord.Version); err != nil {
		slog.Warn("Could not record successful LazyCat installation", "app_id", appID, "version_id", versionID, "error", err)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) beginLazyCatInstall() bool {
	if s.lazycatInstallSlots == nil {
		return true
	}
	select {
	case s.lazycatInstallSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) endLazyCatInstall() {
	if s.lazycatInstallSlots != nil {
		<-s.lazycatInstallSlots
	}
}

func (s *Server) lazycatInstallIdentity(r *http.Request) (lazycatpkg.Identity, bool) {
	if !s.cfg.TrustLazyCatClientInstall {
		return lazycatpkg.Identity{}, false
	}
	identity := lazycatpkg.Identity{
		UserID:   strings.TrimSpace(r.Header.Get("x-hc-user-id")),
		DeviceID: strings.TrimSpace(r.Header.Get("x-hc-device-id")),
	}
	valid := identity.UserID != "" && identity.DeviceID != "" &&
		len(identity.UserID) <= maxLazyCatIdentityHeaderBytes && len(identity.DeviceID) <= maxLazyCatIdentityHeaderBytes
	return identity, valid
}
