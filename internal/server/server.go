package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"entgo.io/ent/dialect"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/internal/buildinfo"
	"lazycat.community/appstore/internal/config"
	"lazycat.community/appstore/internal/storage"
	"lazycat.community/appstore/web"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib-x/entsqlite"
	_ "github.com/lib/pq"
)

type Server struct {
	cfg                     config.Config
	db                      *ent.Client
	storage                 storage.Backend
	mailer                  Mailer
	mux                     *http.ServeMux
	web                     http.Handler
	chatHub                 *chatHub
	allowPrivateLPKURLHosts bool
	adminLoginFailuresMu    sync.Mutex
	adminLoginFailures      map[string]int
	restartAfterImportOnce  sync.Once
	restartAfterImport      func()
}

func New(cfg config.Config) (*Server, error) {
	if err := ensureSQLiteDir(cfg); err != nil {
		return nil, err
	}

	client, err := openEnt(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	backend, err := newStorageBackend(cfg)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	s := &Server{
		cfg:                cfg,
		db:                 client,
		storage:            backend,
		mailer:             newSMTPMailer(cfg),
		mux:                http.NewServeMux(),
		chatHub:            newChatHub(),
		adminLoginFailures: map[string]int{},
	}
	s.web = embeddedWebHandler(cfg)
	if err := s.bootstrap(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	s.routes()
	return s, nil
}

func newStorageBackend(cfg config.Config) (storage.Backend, error) {
	switch strings.ToLower(cfg.StorageBackend) {
	case "", "local":
		return storage.NewLocalBackend(cfg.LocalStoragePath, "/files/"), nil
	case "webdav":
		if cfg.WebDAVURL == "" {
			return nil, fmt.Errorf("WEBDAV_URL is required when STORAGE_BACKEND=webdav")
		}
		return storage.NewWebDAVBackend(cfg.WebDAVURL, cfg.WebDAVUser, cfg.WebDAVPass, cfg.WebDAVPublicURL), nil
	case "s3":
		if cfg.S3Endpoint == "" || cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3_ENDPOINT and S3_BUCKET are required when STORAGE_BACKEND=s3")
		}
		return storage.NewS3Backend(storage.S3Options{
			Endpoint:  cfg.S3Endpoint,
			Bucket:    cfg.S3Bucket,
			Region:    "auto",
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			UseSSL:    cfg.S3UseSSL,
			PathStyle: true,
			PublicURL: cfg.S3PublicURL,
		})
	case "github":
		return storage.NewExternalLinkBackend("github"), nil
	default:
		return nil, fmt.Errorf("unsupported STORAGE_BACKEND %q", cfg.StorageBackend)
	}
}

func (s *Server) Close() error {
	return s.db.Close()
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.cors(s.mux))
}

func (s *Server) SetRestartAfterImport(fn func()) {
	s.restartAfterImport = fn
}

func openEnt(cfg config.Config) (*ent.Client, error) {
	switch cfg.DBDriver {
	case "sqlite3":
		return ent.Open(dialect.SQLite, sqliteDSN(cfg.DBDSN))
	case "postgres":
		return ent.Open(dialect.Postgres, cfg.DBDSN)
	case "mysql":
		return ent.Open(dialect.MySQL, cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DBDriver)
	}
}

func sqliteDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		dsn = config.DefaultSQLiteDSN
	}
	if dsn == ":memory:" {
		dsn = "file::memory:"
	}
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	for _, option := range []struct {
		key   string
		value string
	}{
		{key: "cache=", value: "cache=shared"},
		{key: "_pragma=foreign_keys", value: "_pragma=foreign_keys(1)"},
		{key: "_pragma=journal_mode", value: "_pragma=journal_mode(WAL)"},
		{key: "_pragma=synchronous", value: "_pragma=synchronous(NORMAL)"},
		{key: "_pragma=busy_timeout", value: "_pragma=busy_timeout(10000)"},
	} {
		if strings.Contains(dsn, option.key) {
			continue
		}
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		dsn += separator + option.value
	}
	return dsn
}

func ensureSQLiteDir(cfg config.Config) error {
	if cfg.DBDriver != "sqlite3" {
		return nil
	}
	dsn := strings.TrimPrefix(strings.TrimSpace(cfg.DBDSN), "file:")
	if idx := strings.IndexByte(dsn, '?'); idx >= 0 {
		dsn = dsn[:idx]
	}
	if dsn == "" || dsn == ":memory:" {
		return nil
	}
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (s *Server) routes() {
	s.mux.Handle("GET /files/", http.StripPrefix("/files/", http.HandlerFunc(s.handleLocalFile)))
	s.mux.HandleFunc("GET /api/v1/files/{storageKey}/{path...}", s.handleProxyFile)
	s.mux.HandleFunc("GET /favicon.ico", s.handleFavicon)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "lazycat-appstore-server"})
	})

	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/verify-email", s.handleVerifyEmail)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/auth/password-reset/request", s.handleRequestPasswordReset)
	s.mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", s.handleConfirmPasswordReset)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/setup/status", s.handleSetupStatus)
	s.mux.HandleFunc("POST /api/v1/setup", s.handleSetup)
	s.mux.HandleFunc("GET /api/v1/site/profile", s.handleSiteProfile)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.withAuth(s.handleMe))
	s.mux.HandleFunc("PATCH /api/v1/me/profile", s.withAuth(s.handleUpdateMyProfile))
	s.mux.HandleFunc("POST /api/v1/me/avatar", s.withAuth(s.handleUploadMyAvatar))
	s.mux.HandleFunc("POST /api/v1/me/2fa/setup", s.withAuth(s.handleTwoFactorSetup))
	s.mux.HandleFunc("POST /api/v1/me/2fa/enable", s.withAuth(s.handleTwoFactorEnable))
	s.mux.HandleFunc("POST /api/v1/me/2fa/disable", s.withAuth(s.handleTwoFactorDisable))
	s.mux.HandleFunc("GET /api/v1/me/tokens", s.withAuth(s.handleListTokens))
	s.mux.HandleFunc("POST /api/v1/me/tokens", s.withAuth(s.handleCreateToken))
	s.mux.HandleFunc("DELETE /api/v1/me/tokens/{id}", s.withAuth(s.handleDeleteToken))
	s.mux.HandleFunc("GET /api/v1/me/mcp", s.withAuth(s.handleMCPProfile))
	s.mux.HandleFunc("GET /api/v1/me/mcp/tokens", s.withAuth(s.handleListMCPTokens))
	s.mux.HandleFunc("POST /api/v1/me/mcp/tokens", s.withAuth(s.handleCreateMCPToken))
	s.mux.HandleFunc("DELETE /api/v1/me/mcp/tokens/{id}", s.withAuth(s.handleDeleteMCPToken))
	s.mux.HandleFunc("GET /api/v1/storage-options", s.withAuth(s.handleListStorageOptions))
	s.mux.HandleFunc("GET /api/v1/me/favorites", s.withAuth(s.handleListFavorites))
	s.mux.HandleFunc("GET /api/v1/me/collaboration", s.withAuth(s.handleMyCollaboration))
	s.mux.HandleFunc("GET /api/v1/me/comment-notifications", s.withAuth(s.handleListCommentNotifications))
	s.mux.HandleFunc("POST /api/v1/me/comment-notifications/read", s.withAuth(s.handleReadAllCommentNotifications))
	s.mux.HandleFunc("POST /api/v1/me/comment-notifications/{id}/read", s.withAuth(s.handleReadCommentNotification))

	s.mux.HandleFunc("GET /api/v1/apps", s.handleListApps)
	s.mux.HandleFunc("POST /api/v1/apps", s.withAuth(s.handleCreateApp))
	s.mux.HandleFunc("GET /api/v1/apps/{id}", s.handleGetApp)
	s.mux.HandleFunc("PATCH /api/v1/apps/{id}", s.withAuth(s.handleUpdateApp))
	s.mux.HandleFunc("DELETE /api/v1/apps/{id}", s.withAuth(s.handleDeleteApp))
	s.mux.HandleFunc("POST /api/v1/apps/{id}/unlist", s.withAuth(s.handleUnlistApp))
	s.mux.HandleFunc("POST /api/v1/apps/{id}/versions", s.withAuth(s.handleCreateVersion))
	s.mux.HandleFunc("GET /api/v1/apps/{id}/versions/{versionId}/download", s.handleDownloadVersion)
	s.mux.HandleFunc("POST /api/v1/apps/{id}/screenshots", s.withAuth(s.handleUploadScreenshot))
	s.mux.HandleFunc("PATCH /api/v1/apps/{id}/screenshots/reorder", s.withAuth(s.handleReorderScreenshots))
	s.mux.HandleFunc("PATCH /api/v1/apps/{id}/screenshots/{screenshotId}", s.withAuth(s.handleUpdateScreenshot))
	s.mux.HandleFunc("DELETE /api/v1/apps/{id}/screenshots/{screenshotId}", s.withAuth(s.handleDeleteScreenshot))
	s.mux.HandleFunc("GET /api/v1/apps/{id}/comments", s.handleListComments)
	s.mux.HandleFunc("POST /api/v1/apps/{id}/comments", s.handleCreateComment)
	s.mux.HandleFunc("DELETE /api/v1/comments/{id}", s.handleDeleteComment)
	s.mux.HandleFunc("POST /api/v1/apps/{id}/chat", s.handleCreateAppChatConversation)
	s.mux.HandleFunc("POST /api/v1/apps/{id}/favorites", s.withAuth(s.handleToggleFavorite))
	s.mux.HandleFunc("POST /api/v1/submitters/{id}/favorites", s.withAuth(s.handleToggleSubmitterFavorite))
	s.mux.HandleFunc("POST /api/v1/apps/{id}/outdated-marks", s.handleMarkOutdated)
	s.mux.HandleFunc("DELETE /api/v1/apps/{id}/outdated-marks", s.handleClearOutdated)
	s.mux.HandleFunc("POST /api/v1/apps/{id}/collaborator-requests", s.withAuth(s.handleCreateCollaboratorRequest))
	s.mux.HandleFunc("GET /api/v1/apps/{id}/collaborator-requests", s.withAuth(s.handleListCollaboratorRequests))
	s.mux.HandleFunc("POST /api/v1/apps/{id}/collaborators", s.withAuth(s.handleAddCollaborator))
	s.mux.HandleFunc("DELETE /api/v1/apps/{id}/collaborators/{userId}", s.withAuth(s.handleDeleteCollaborator))
	s.mux.HandleFunc("POST /api/v1/apps/{id}/collaborator-invites", s.withAuth(s.handleCreateCollaboratorInvite))
	s.mux.HandleFunc("POST /api/v1/collaborator-invites/accept", s.withAuth(s.handleAcceptCollaboratorInvite))
	s.mux.HandleFunc("POST /api/v1/collaborator-requests/{id}/approve", s.withAuth(s.handleApproveCollaboratorRequest))
	s.mux.HandleFunc("POST /api/v1/collaborator-requests/{id}/reject", s.withAuth(s.handleRejectCollaboratorRequest))
	s.mux.HandleFunc("PATCH /api/v1/apps/{id}/visibility", s.withAuth(s.handleSetAppVisibility))
	s.mux.HandleFunc("GET /api/v1/groups", s.withAuth(s.handleListGroups))
	s.mux.HandleFunc("POST /api/v1/groups", s.withAuth(s.handleCreateGroup))
	s.mux.HandleFunc("POST /api/v1/groups/client-config", s.withAuth(s.handleGroupClientConfig))
	s.mux.HandleFunc("POST /api/v1/groups/{id}/code:rotate", s.withAuth(s.handleRotateGroupCode))
	s.mux.HandleFunc("DELETE /api/v1/groups/{id}", s.withAuth(s.handleDeleteGroup))
	s.mux.HandleFunc("POST /api/v1/groups/{id}/members/{userId}", s.withAuth(s.handleAddGroupMember))
	s.mux.HandleFunc("DELETE /api/v1/groups/{id}/members/{userId}", s.withAuth(s.handleRemoveGroupMember))
	s.mux.HandleFunc("GET /api/v1/categories", s.handlePublicListCategories)
	s.mux.HandleFunc("GET /api/v1/tags", s.handlePublicListTags)
	s.mux.HandleFunc("GET /api/v1/collections", s.handlePublicListCollections)
	s.mux.HandleFunc("GET /api/v1/chat/users", s.withAuth(s.handleListChatUsers))
	s.mux.HandleFunc("GET /api/v1/chat/conversations", s.handleListChatConversations)
	s.mux.HandleFunc("POST /api/v1/chat/conversations", s.handleCreateChatConversation)
	s.mux.HandleFunc("GET /api/v1/chat/conversations/{id}/messages", s.handleListChatMessages)
	s.mux.HandleFunc("POST /api/v1/chat/conversations/{id}/messages", s.handleCreateChatMessage)
	s.mux.HandleFunc("POST /api/v1/chat/conversations/{id}/read", s.handleReadChatConversation)
	s.mux.HandleFunc("DELETE /api/v1/chat/conversations/{id}", s.handleDeleteChatConversation)
	s.mux.HandleFunc("GET /api/v1/chat/events", s.handleChatEvents)

	admin := s.withRole(userRoleSoftwareAdmin, userRoleSiteAdmin)
	s.mux.HandleFunc("GET /api/v1/admin/reviews", admin(s.handleListReviews))
	s.mux.HandleFunc("POST /api/v1/admin/reviews/{id}/approve", admin(s.handleApproveReview))
	s.mux.HandleFunc("POST /api/v1/admin/reviews/{id}/reject", admin(s.handleRejectReview))
	s.mux.HandleFunc("GET /api/v1/admin/categories", admin(s.handleListCategories))
	s.mux.HandleFunc("POST /api/v1/admin/categories", admin(s.handleCreateCategory))
	s.mux.HandleFunc("PATCH /api/v1/admin/categories/{id}", admin(s.handleUpdateCategory))
	s.mux.HandleFunc("DELETE /api/v1/admin/categories/{id}", admin(s.handleDeleteCategory))
	s.mux.HandleFunc("GET /api/v1/admin/tags", admin(s.handleListTags))
	s.mux.HandleFunc("POST /api/v1/admin/tags", admin(s.handleCreateTag))
	s.mux.HandleFunc("PATCH /api/v1/admin/tags/{id}", admin(s.handleUpdateTag))
	s.mux.HandleFunc("DELETE /api/v1/admin/tags/{id}", admin(s.handleDeleteTag))
	s.mux.HandleFunc("GET /api/v1/admin/collections", admin(s.handleAdminListCollections))
	s.mux.HandleFunc("POST /api/v1/admin/collections", admin(s.handleCreateCollection))
	s.mux.HandleFunc("PATCH /api/v1/admin/collections/{id}", admin(s.handleUpdateCollection))
	s.mux.HandleFunc("DELETE /api/v1/admin/collections/{id}", admin(s.handleDeleteCollection))
	s.mux.HandleFunc("GET /api/v1/admin/users", s.withRole(userRoleSiteAdmin)(s.handleAdminListUsers))
	s.mux.HandleFunc("POST /api/v1/admin/users", s.withRole(userRoleSiteAdmin)(s.handleAdminCreateUser))
	s.mux.HandleFunc("PATCH /api/v1/admin/users/{id}", s.withRole(userRoleSiteAdmin)(s.handleAdminUpdateUser))
	s.mux.HandleFunc("POST /api/v1/admin/users/{id}/2fa/reset", s.withRole(userRoleSiteAdmin)(s.handleAdminResetUserTwoFactor))
	s.mux.HandleFunc("DELETE /api/v1/admin/users/{id}", s.withRole(userRoleSiteAdmin)(s.handleAdminDeleteUser))
	s.mux.HandleFunc("GET /api/v1/admin/settings", s.withRole(userRoleSiteAdmin)(s.handleGetSettings))
	s.mux.HandleFunc("PATCH /api/v1/admin/settings", s.withRole(userRoleSiteAdmin)(s.handleUpdateSettings))
	s.mux.HandleFunc("POST /api/v1/admin/settings/site-icon", s.withRole(userRoleSiteAdmin)(s.handleUploadSiteIcon))
	s.mux.HandleFunc("POST /api/v1/admin/settings/test-email", s.withRole(userRoleSiteAdmin)(s.handleSendTestEmail))
	s.mux.HandleFunc("GET /api/v1/admin/announcements", s.withRole(userRoleSiteAdmin)(s.handleListAnnouncements))
	s.mux.HandleFunc("POST /api/v1/admin/announcements", s.withRole(userRoleSiteAdmin)(s.handleCreateAnnouncement))
	s.mux.HandleFunc("PATCH /api/v1/admin/announcements/{id}", s.withRole(userRoleSiteAdmin)(s.handleUpdateAnnouncement))
	s.mux.HandleFunc("DELETE /api/v1/admin/announcements/{id}", s.withRole(userRoleSiteAdmin)(s.handleDeleteAnnouncement))
	s.mux.HandleFunc("GET /api/v1/admin/registration-invites", s.withRole(userRoleSiteAdmin)(s.handleListRegistrationInvites))
	s.mux.HandleFunc("POST /api/v1/admin/registration-invites", s.withRole(userRoleSiteAdmin)(s.handleCreateRegistrationInvite))
	s.mux.HandleFunc("DELETE /api/v1/admin/registration-invites/{id}", s.withRole(userRoleSiteAdmin)(s.handleDeleteRegistrationInvite))
	s.mux.HandleFunc("GET /api/v1/admin/storage", s.withRole(userRoleSiteAdmin)(s.handleListStorageConfigs))
	s.mux.HandleFunc("POST /api/v1/admin/storage", s.withRole(userRoleSiteAdmin)(s.handleCreateStorageConfig))
	s.mux.HandleFunc("PATCH /api/v1/admin/storage", s.withRole(userRoleSiteAdmin)(s.handleUpdateDefaultStorageConfig))
	s.mux.HandleFunc("PATCH /api/v1/admin/storage/{key}", s.withRole(userRoleSiteAdmin)(s.handleUpdateStorageConfig))
	s.mux.HandleFunc("DELETE /api/v1/admin/storage/{key}", s.withRole(userRoleSiteAdmin)(s.handleDeleteStorageConfig))
	s.mux.HandleFunc("POST /api/v1/admin/storage/{key}/default", s.withRole(userRoleSiteAdmin)(s.handleSetDefaultStorageConfig))
	s.mux.HandleFunc("POST /api/v1/admin/storage/{key}/test", s.withRole(userRoleSiteAdmin)(s.handleTestSavedStorageConfig))
	s.mux.HandleFunc("POST /api/v1/admin/storage/test", s.withRole(userRoleSiteAdmin)(s.handleTestStorageConfig))
	s.mux.HandleFunc("POST /api/v1/admin/migration/export", s.withRole(userRoleSiteAdmin)(s.handleMigrationExport))
	s.mux.HandleFunc("POST /api/v1/admin/migration/import/preview", s.withRole(userRoleSiteAdmin)(s.handleMigrationImportPreview))
	s.mux.HandleFunc("POST /api/v1/admin/migration/import", s.withRole(userRoleSiteAdmin)(s.handleMigrationImport))

	s.mux.HandleFunc("GET /source/v1/index.json", s.handleSourceIndexV1)
	s.mux.HandleFunc("GET /source/v2/index.json", s.handleSourceIndexV2)
	s.mux.Handle("/mcp", s.mcpHandler())

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isAPINamespace(r.URL.Path) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", nil)
			return
		}
		if s.web != nil {
			s.web.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "lazycat-community-appstore-server",
			"source":  s.siteProfile(r.Context()).SourceURL,
		})
	})
}

func isAPINamespace(rawPath string) bool {
	return strings.HasPrefix(rawPath, "/api/") || strings.HasPrefix(rawPath, "/source/") || strings.HasPrefix(rawPath, "/files/")
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	iconURL := s.siteProfile(r.Context()).IconURL
	if iconURL != "" {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, iconURL, http.StatusFound)
		return
	}
	if s.web != nil {
		s.web.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func embeddedWebHandler(cfg config.Config) http.Handler {
	dist, err := web.Dist()
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return nil
	}
	return spaFileServer{assets: dist, cfg: cfg}
}

type spaFileServer struct {
	assets fs.FS
	cfg    config.Config
}

func (h spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	if name == "app-config.js" {
		h.serveAppConfig(w, r)
		return
	}
	if info, err := fs.Stat(h.assets, name); err == nil && !info.IsDir() {
		h.serveName(w, r, name)
		return
	}
	if strings.Contains(path.Base(name), ".") {
		http.NotFound(w, r)
		return
	}
	h.serveName(w, r, "index.html")
}

func (h spaFileServer) serveName(w http.ResponseWriter, r *http.Request, name string) {
	file, err := h.assets.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read embedded asset", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, path.Base(name), info.ModTime(), bytes.NewReader(data))
}

func (h spaFileServer) serveAppConfig(w http.ResponseWriter, r *http.Request) {
	configJSON, err := json.Marshal(map[string]string{
		"apiBaseURL":        "",
		"defaultSourceURL":  strings.TrimRight(h.cfg.BaseURL, "/") + "/source/v2/index.json",
		"defaultSourceName": "喵喵私有商店",
		"appVersion":        buildinfo.Version,
	})
	if err != nil {
		http.Error(w, "build app config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprintf(w, "window.LAZYCAT_APPSTORE_CONFIG = Object.assign(%s, { apiBaseURL: window.location.origin, defaultSourceURL: window.location.origin + \"/source/v2/index.json\" });\n", configJSON)
}

func (s *Server) handleLocalFile(w http.ResponseWriter, r *http.Request) {
	full, err := safeLocalFilePath(s.cfg.LocalStoragePath, r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found", nil)
		return
	}
	file, err := os.Open(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found", nil)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found", nil)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func safeLocalFilePath(root, rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" || rawPath == "/" {
		return "", fmt.Errorf("empty path")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(strings.TrimLeft(filepath.ToSlash(rawPath), "/"))
	full, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(clean)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, full)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file path escapes root")
	}
	return full, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
