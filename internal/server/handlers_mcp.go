package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/user"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpScope = "appstore:mcp"

var mcpPermanentExpiry = time.Date(5000, 1, 1, 0, 0, 0, 0, time.UTC)

const tokenLastUsedAtUpdateInterval = time.Minute

type mcpProfileResponse struct {
	Endpoint       string                   `json:"endpoint"`
	SourceURL      string                   `json:"sourceUrl"`
	PrincipalTypes []mcptoken.PrincipalType `json:"principalTypes"`
}

type mcpTokensResponse struct {
	Tokens []mcpTokenDTO `json:"tokens"`
}

type mcpTokenCreateResponse struct {
	Token  string      `json:"token"`
	Record mcpTokenDTO `json:"record"`
}

func (s *Server) handleMCPProfile(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	principalTypes := []mcptoken.PrincipalType{mcptoken.PrincipalTypeUSER}
	if isAdmin(u) {
		principalTypes = append(principalTypes, mcptoken.PrincipalTypeADMIN)
	}
	profile := s.siteProfile(r.Context())
	writeJSON(w, http.StatusOK, mcpProfileResponse{
		Endpoint:       profile.PublicURL + "/mcp",
		SourceURL:      profile.SourceURL,
		PrincipalTypes: principalTypes,
	})
}

func (s *Server) handleListMCPTokens(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	records, err := s.db.MCPToken.Query().
		Where(mcptoken.UserIDEQ(u.ID)).
		Order(entgo.Desc(mcptoken.FieldCreatedAt)).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MCP_TOKEN_LIST_FAILED", "Could not list MCP tokens", nil)
		return
	}
	out := make([]mcpTokenDTO, 0, len(records))
	for _, record := range records {
		out = append(out, toMCPTokenDTO(record))
	}
	writeJSON(w, http.StatusOK, mcpTokensResponse{Tokens: out})
}

type createMCPTokenRequest struct {
	Note          string     `json:"note"`
	PrincipalType string     `json:"principalType"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	NeverExpires  bool       `json:"neverExpires"`
}

func (s *Server) handleCreateMCPToken(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input createMCPTokenRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	principalType := mcptoken.PrincipalTypeUSER
	switch strings.ToUpper(strings.TrimSpace(input.PrincipalType)) {
	case "", string(mcptoken.PrincipalTypeUSER):
		principalType = mcptoken.PrincipalTypeUSER
	case string(mcptoken.PrincipalTypeADMIN):
		if !isAdmin(u) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Only administrators can create admin MCP tokens", nil)
			return
		}
		principalType = mcptoken.PrincipalTypeADMIN
	default:
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Unsupported MCP token type", nil)
		return
	}
	if input.NeverExpires {
		input.ExpiresAt = nil
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Expiration must be in the future", nil)
		return
	}
	note := strings.TrimSpace(input.Note)
	if len([]rune(note)) > 160 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Note must be at most 160 characters", nil)
		return
	}
	token, err := randomMCPToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MCP_TOKEN_CREATE_FAILED", "Could not create MCP token", nil)
		return
	}
	record, err := s.db.MCPToken.Create().
		SetUserID(u.ID).
		SetPrincipalType(principalType).
		SetNote(note).
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SetNillableExpiresAt(input.ExpiresAt).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MCP_TOKEN_CREATE_FAILED", "Could not create MCP token", nil)
		return
	}
	writeJSON(w, http.StatusCreated, mcpTokenCreateResponse{Token: token, Record: toMCPTokenDTO(record)})
}

func (s *Server) handleDeleteMCPToken(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.MCPToken.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "MCP_TOKEN_NOT_FOUND", "MCP token not found", nil)
		return
	}
	if record.UserID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot delete this MCP token", nil)
		return
	}
	if err := s.db.MCPToken.DeleteOneID(record.ID).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "MCP_TOKEN_DELETE_FAILED", "Could not delete MCP token", nil)
		return
	}
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (s *Server) mcpHandler() http.Handler {
	stream := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		principalType := mcptoken.PrincipalTypeUSER
		if info := mcpauth.TokenInfoFromContext(r.Context()); info != nil {
			principalType = mcpPrincipalTypeFromInfo(info)
		}
		return s.newMCPServer(r.Context(), principalType)
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 30 * time.Minute,
	})
	return mcpauth.RequireBearerToken(s.verifyMCPBearerToken, &mcpauth.RequireBearerTokenOptions{
		Scopes: []string{mcpScope},
	})(stream)
}

func (s *Server) verifyMCPBearerToken(ctx context.Context, tokenValue string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	record, err := s.db.MCPToken.Query().Where(mcptoken.TokenHashEQ(tokenHash(tokenValue))).Only(ctx)
	if err != nil {
		return nil, mcpauth.ErrInvalidToken
	}
	now := time.Now()
	if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
		return nil, mcpauth.ErrInvalidToken
	}
	u, err := s.db.User.Get(ctx, record.UserID)
	if err != nil || u.Disabled {
		return nil, mcpauth.ErrInvalidToken
	}
	if s.emailVerificationRequiredForUser(ctx, u) {
		return nil, mcpauth.ErrInvalidToken
	}
	if record.PrincipalType == mcptoken.PrincipalTypeADMIN && !isAdmin(u) {
		return nil, mcpauth.ErrInvalidToken
	}
	s.touchMCPTokenLastUsedAt(ctx, record.ID, now)
	expiration := mcpPermanentExpiry
	if record.ExpiresAt != nil {
		expiration = *record.ExpiresAt
	}
	return &mcpauth.TokenInfo{
		Scopes:     []string{mcpScope},
		Expiration: expiration,
		UserID:     strconv.Itoa(u.ID),
		Extra: map[string]any{
			"tokenID":       record.ID,
			"userID":        u.ID,
			"principalType": string(record.PrincipalType),
		},
	}, nil
}

func (s *Server) touchMCPTokenLastUsedAt(ctx context.Context, tokenID int, now time.Time) {
	_, _ = s.db.MCPToken.Update().
		Where(
			mcptoken.IDEQ(tokenID),
			mcptoken.Or(
				mcptoken.LastUsedAtIsNil(),
				mcptoken.LastUsedAtLT(now.Add(-tokenLastUsedAtUpdateInterval)),
			),
		).
		SetLastUsedAt(now).
		Save(ctx)
}

func (s *Server) newMCPServer(ctx context.Context, principalType mcptoken.PrincipalType) *mcp.Server {
	server := mcp.NewServer(s.mcpImplementation(ctx), &mcp.ServerOptions{
		Instructions: "Use these tools to publish and update LazyCat LPK apps in this app store. Tool permissions are scoped by the MCP token type and the bound user.",
	})
	s.registerUserMCPTools(server)
	if principalType == mcptoken.PrincipalTypeADMIN {
		s.registerAdminMCPTools(server)
	}
	return server
}

func (s *Server) mcpImplementation(ctx context.Context) *mcp.Implementation {
	return &mcp.Implementation{
		Name:       "lazycat-community-appstore",
		Title:      "LazyCat Community App Store",
		Version:    appVersion(),
		WebsiteURL: s.sitePublicURL(ctx),
	}
}

func (s *Server) registerUserMCPTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "appstore_list_my_apps",
		Title:       "List my app-store apps",
		Description: "List apps owned by, or collaborated on by, the token user.",
	}, s.mcpListMyApps)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "appstore_create_app_from_url",
		Title:       "Create app from LPK URL",
		Description: "Create an app submission from a remote LPK URL. GitHub raw and release URLs may use configured mirrors.",
	}, s.mcpCreateAppFromURL)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "appstore_publish_version_from_url",
		Title:       "Publish app version from LPK URL",
		Description: "Publish a new version for an existing app from a remote LPK URL.",
	}, s.mcpPublishVersionFromURL)
}

func (s *Server) registerAdminMCPTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "appstore_admin_list_apps",
		Title:       "List all app-store apps",
		Description: "Admin-only list of apps across all submitters, with optional status, owner, and search filters.",
	}, s.mcpAdminListApps)
}

type mcpPrincipal struct {
	UserID        int
	TokenID       int
	PrincipalType mcptoken.PrincipalType
}

func mcpPrincipalTypeFromInfo(info *mcpauth.TokenInfo) mcptoken.PrincipalType {
	if info != nil && info.Extra != nil {
		if raw, ok := info.Extra["principalType"].(string); ok && raw == string(mcptoken.PrincipalTypeADMIN) {
			return mcptoken.PrincipalTypeADMIN
		}
	}
	return mcptoken.PrincipalTypeUSER
}

func mcpPrincipalFromInfo(info *mcpauth.TokenInfo) (mcpPrincipal, error) {
	if info == nil {
		return mcpPrincipal{}, errors.New("missing MCP token context")
	}
	principal := mcpPrincipal{PrincipalType: mcpPrincipalTypeFromInfo(info)}
	if info.UserID != "" {
		id, err := strconv.Atoi(info.UserID)
		if err != nil {
			return mcpPrincipal{}, errors.New("invalid MCP user context")
		}
		principal.UserID = id
	}
	if info.Extra != nil {
		if raw, ok := info.Extra["userID"].(int); ok {
			principal.UserID = raw
		}
		if raw, ok := info.Extra["tokenID"].(int); ok {
			principal.TokenID = raw
		}
	}
	if principal.UserID <= 0 {
		return mcpPrincipal{}, errors.New("missing MCP user context")
	}
	return principal, nil
}

func (s *Server) mcpEffectiveUser(ctx context.Context, req *mcp.CallToolRequest) (*entgo.User, mcptoken.PrincipalType, error) {
	principal, err := mcpPrincipalFromInfo(req.Extra.TokenInfo)
	if err != nil {
		return nil, mcptoken.PrincipalTypeUSER, err
	}
	u, err := s.db.User.Get(ctx, principal.UserID)
	if err != nil || u.Disabled {
		return nil, mcptoken.PrincipalTypeUSER, errors.New("MCP token user is not available")
	}
	if principal.PrincipalType == mcptoken.PrincipalTypeADMIN {
		if !isAdmin(u) {
			return nil, mcptoken.PrincipalTypeUSER, errors.New("MCP admin token is no longer valid for this user")
		}
		return u, principal.PrincipalType, nil
	}
	if isAdmin(u) {
		clone := *u
		clone.Role = user.RoleUSER
		return &clone, mcptoken.PrincipalTypeUSER, nil
	}
	return u, mcptoken.PrincipalTypeUSER, nil
}

func mcpRequest(ctx context.Context) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/mcp", nil)
	return req
}

type mcpListAppsInput struct {
	Query string `json:"query,omitempty" jsonschema:"optional text search"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of apps to return, default 50, maximum 100"`
}

type mcpAdminListAppsInput struct {
	Query   string `json:"query,omitempty" jsonschema:"optional text search"`
	Status  string `json:"status,omitempty" jsonschema:"optional app status filter such as APPROVED, PENDING, REJECTED, or UNLISTED"`
	OwnerID int    `json:"ownerId,omitempty" jsonschema:"optional owner user ID filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of apps to return, default 100, maximum 200"`
}

type mcpAppSummary struct {
	ID               int    `json:"id"`
	OwnerID          int    `json:"ownerId"`
	Owner            string `json:"owner"`
	PackageID        string `json:"packageId"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	LatestVersion    string `json:"latestVersion,omitempty"`
	CanManageApp     bool   `json:"canManageApp"`
	CanUploadVersion bool   `json:"canUploadVersion"`
	UpdatedAt        string `json:"updatedAt"`
}

type mcpVersionSummary struct {
	ID        int    `json:"id"`
	AppID     int    `json:"appId"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	Source    string `json:"sourceType"`
	SHA256    string `json:"sha256"`
	FileSize  int64  `json:"fileSize,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type mcpAppsOutput struct {
	Apps []mcpAppSummary `json:"apps"`
}

func (s *Server) mcpListMyApps(ctx context.Context, req *mcp.CallToolRequest, input mcpListAppsInput) (*mcp.CallToolResult, mcpAppsOutput, error) {
	u, _, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpAppsOutput{}, err
	}
	httpReq := mcpRequest(ctx)
	ids := s.collaboratorAppIDs(ctx, u.ID)
	q := s.db.App.Query().Order(entgo.Desc(app.FieldUpdatedAt)).Limit(clampLimit(input.Limit, 50, 100))
	if len(ids) > 0 {
		q.Where(app.Or(app.OwnerIDEQ(u.ID), app.IDIn(ids...)))
	} else {
		q.Where(app.OwnerIDEQ(u.ID))
	}
	if search := strings.TrimSpace(input.Query); search != "" {
		q.Where(app.Or(app.NameContainsFold(search), app.PackageIDContainsFold(search), app.SummaryContainsFold(search), app.DescriptionContainsFold(search)))
	}
	records, err := q.All(ctx)
	if err != nil {
		return nil, mcpAppsOutput{}, err
	}
	return nil, mcpAppsOutput{Apps: s.mcpAppSummaries(httpReq, records, u)}, nil
}

func (s *Server) mcpAdminListApps(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminListAppsInput) (*mcp.CallToolResult, mcpAppsOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpAppsOutput{}, err
	}
	if principalType != mcptoken.PrincipalTypeADMIN {
		return nil, mcpAppsOutput{}, errors.New("admin MCP token is required")
	}
	httpReq := mcpRequest(ctx)
	q := s.db.App.Query().Order(entgo.Desc(app.FieldUpdatedAt)).Limit(clampLimit(input.Limit, 100, 200))
	if search := strings.TrimSpace(input.Query); search != "" {
		q.Where(app.Or(app.NameContainsFold(search), app.PackageIDContainsFold(search), app.SummaryContainsFold(search), app.DescriptionContainsFold(search)))
	}
	if status := strings.ToUpper(strings.TrimSpace(input.Status)); status != "" {
		q.Where(app.StatusEQ(app.Status(status)))
	}
	if input.OwnerID > 0 {
		q.Where(app.OwnerIDEQ(input.OwnerID))
	}
	records, err := q.All(ctx)
	if err != nil {
		return nil, mcpAppsOutput{}, err
	}
	return nil, mcpAppsOutput{Apps: s.mcpAppSummaries(httpReq, records, u)}, nil
}

type mcpCreateAppFromURLInput struct {
	Name                      string   `json:"name,omitempty" jsonschema:"app display name; inferred from LPK metadata when omitted"`
	PackageID                 string   `json:"packageId,omitempty" jsonschema:"LazyCat package ID; inferred from LPK metadata when omitted"`
	Slug                      string   `json:"slug,omitempty" jsonschema:"optional app slug"`
	Summary                   string   `json:"summary,omitempty" jsonschema:"short summary; inferred from LPK metadata when omitted"`
	Description               string   `json:"description,omitempty" jsonschema:"full description; inferred from LPK metadata when omitted"`
	IconURL                   string   `json:"iconUrl,omitempty" jsonschema:"optional icon URL or data URL; inferred from LPK metadata when omitted"`
	CategoryID                *int     `json:"categoryId,omitempty" jsonschema:"optional category ID"`
	Tags                      []string `json:"tags,omitempty" jsonschema:"optional tags"`
	AllowUnreviewedUpdates    bool     `json:"allowUnreviewedUpdates,omitempty" jsonschema:"whether future versions can skip review"`
	CommentsEnabled           *bool    `json:"commentsEnabled,omitempty" jsonschema:"whether comments are enabled for this app"`
	EmailNotificationsEnabled *bool    `json:"emailNotificationsEnabled,omitempty" jsonschema:"whether comment/update notifications are enabled"`
	InstallPassword           string   `json:"installPassword,omitempty" jsonschema:"optional install password"`
	Version                   string   `json:"version,omitempty" jsonschema:"version; inferred from LPK metadata when omitted"`
	Changelog                 string   `json:"changelog,omitempty" jsonschema:"optional changelog"`
	DownloadURL               string   `json:"downloadUrl" jsonschema:"remote LPK URL; GitHub raw and release URLs are supported"`
	SourceType                string   `json:"sourceType,omitempty" jsonschema:"source type, default GITHUB"`
	SHA256                    string   `json:"sha256,omitempty" jsonschema:"optional SHA256; computed from LPK when omitted"`
	UseMirrorDownload         *bool    `json:"useMirrorDownload,omitempty" jsonschema:"use configured GitHub mirrors when applicable, default true"`
}

type mcpCreateAppOutput struct {
	App     mcpAppSummary      `json:"app"`
	Version *mcpVersionSummary `json:"version,omitempty"`
}

func (s *Server) mcpCreateAppFromURL(ctx context.Context, req *mcp.CallToolRequest, input mcpCreateAppFromURLInput) (*mcp.CallToolResult, mcpCreateAppOutput, error) {
	u, _, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpCreateAppOutput{}, err
	}
	if strings.TrimSpace(input.DownloadURL) == "" {
		return nil, mcpCreateAppOutput{}, errors.New("downloadUrl is required")
	}
	httpReq := mcpRequest(ctx)
	createInput := createAppJSON{
		Name:                      input.Name,
		PackageID:                 input.PackageID,
		Slug:                      input.Slug,
		Summary:                   input.Summary,
		Description:               input.Description,
		IconURL:                   input.IconURL,
		CategoryID:                input.CategoryID,
		Tags:                      input.Tags,
		AllowUnreviewedUpdates:    input.AllowUnreviewedUpdates,
		CommentsEnabled:           input.CommentsEnabled,
		EmailNotificationsEnabled: input.EmailNotificationsEnabled,
		InstallPassword:           input.InstallPassword,
		Version:                   input.Version,
		Changelog:                 input.Changelog,
		DownloadURL:               normalizeGitHubRawURL(input.DownloadURL),
		SourceType:                input.SourceType,
		SHA256:                    input.SHA256,
		UseMirrorDownload:         boolDefault(input.UseMirrorDownload, true),
	}
	var inspected lpkInspection
	if createInput.DownloadURL != "" && appInputNeedsLPKInspection(createInput) {
		inspected, err = s.inspectLPKURL(ctx, createInput.DownloadURL, s.effectiveMaxLPKSize(ctx), createInput.UseMirrorDownload)
		if err != nil {
			return nil, mcpCreateAppOutput{}, err
		}
		if err := applyAppMetadata(&createInput, inspected.Metadata); err != nil {
			return nil, mcpCreateAppOutput{}, err
		}
		if createInput.SHA256 == "" {
			createInput.SHA256 = inspected.SHA256
		}
	}
	record, err := s.createAppRecord(httpReq, u, createInput)
	if err != nil {
		return nil, mcpCreateAppOutput{}, err
	}
	var versionOut *mcpVersionSummary
	if createInput.DownloadURL != "" {
		created, err := s.createExternalVersion(httpReq, u, record, createInput.Version, createInput.Changelog, createInput.DownloadURL, createInput.SourceType, createInput.SHA256, inspected.Size)
		if err != nil {
			return nil, mcpCreateAppOutput{}, err
		}
		summary := toMCPVersionSummary(created)
		versionOut = &summary
	}
	return nil, mcpCreateAppOutput{
		App:     toMCPAppSummary(s.appSummaryDTO(httpReq, record, u)),
		Version: versionOut,
	}, nil
}

type mcpPublishVersionFromURLInput struct {
	AppID             int    `json:"appId,omitempty" jsonschema:"app database ID; either appId or packageId is required"`
	PackageID         string `json:"packageId,omitempty" jsonschema:"LazyCat package ID; either appId or packageId is required"`
	Version           string `json:"version,omitempty" jsonschema:"version; inferred from LPK metadata when omitted"`
	Changelog         string `json:"changelog,omitempty" jsonschema:"optional changelog"`
	DownloadURL       string `json:"downloadUrl" jsonschema:"remote LPK URL; GitHub raw and release URLs are supported"`
	SourceType        string `json:"sourceType,omitempty" jsonschema:"source type, default GITHUB"`
	SHA256            string `json:"sha256,omitempty" jsonschema:"optional SHA256; computed from LPK when omitted"`
	UseMirrorDownload *bool  `json:"useMirrorDownload,omitempty" jsonschema:"use configured GitHub mirrors when applicable, default true"`
}

type mcpPublishVersionOutput struct {
	App     mcpAppSummary     `json:"app"`
	Version mcpVersionSummary `json:"version"`
}

func (s *Server) mcpPublishVersionFromURL(ctx context.Context, req *mcp.CallToolRequest, input mcpPublishVersionFromURLInput) (*mcp.CallToolResult, mcpPublishVersionOutput, error) {
	u, _, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpPublishVersionOutput{}, err
	}
	if strings.TrimSpace(input.DownloadURL) == "" {
		return nil, mcpPublishVersionOutput{}, errors.New("downloadUrl is required")
	}
	httpReq := mcpRequest(ctx)
	record, err := s.mcpFindApp(ctx, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpPublishVersionOutput{}, err
	}
	if !s.canUploadVersion(httpReq, record, u) {
		return nil, mcpPublishVersionOutput{}, errors.New("you cannot upload versions for this app")
	}
	createInput := createAppJSON{
		Version:           input.Version,
		Changelog:         input.Changelog,
		DownloadURL:       normalizeGitHubRawURL(input.DownloadURL),
		SourceType:        input.SourceType,
		SHA256:            input.SHA256,
		UseMirrorDownload: boolDefault(input.UseMirrorDownload, true),
	}
	var inspected lpkInspection
	if createInput.DownloadURL != "" && versionInputNeedsLPKInspection(createInput) {
		inspected, err = s.inspectLPKURL(ctx, createInput.DownloadURL, s.effectiveMaxLPKSize(ctx), createInput.UseMirrorDownload)
		if err != nil {
			return nil, mcpPublishVersionOutput{}, err
		}
		if inspected.Metadata.PackageID != "" && inspected.Metadata.PackageID != record.PackageID {
			return nil, mcpPublishVersionOutput{}, fmt.Errorf("LPK package %q does not match app packageId %q", inspected.Metadata.PackageID, record.PackageID)
		}
		if createInput.Version == "" {
			createInput.Version = inspected.Metadata.Version
		}
		if createInput.SHA256 == "" {
			createInput.SHA256 = inspected.SHA256
		}
	}
	created, err := s.createExternalVersion(httpReq, u, record, createInput.Version, createInput.Changelog, createInput.DownloadURL, createInput.SourceType, createInput.SHA256, inspected.Size)
	if err != nil {
		return nil, mcpPublishVersionOutput{}, err
	}
	return nil, mcpPublishVersionOutput{
		App:     toMCPAppSummary(s.appSummaryDTO(httpReq, record, u)),
		Version: toMCPVersionSummary(created),
	}, nil
}

func (s *Server) mcpFindApp(ctx context.Context, appID int, packageID string) (*entgo.App, error) {
	if appID > 0 {
		record, err := s.db.App.Get(ctx, appID)
		if err != nil {
			return nil, errors.New("app not found")
		}
		return record, nil
	}
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return nil, errors.New("appId or packageId is required")
	}
	record, err := s.db.App.Query().Where(app.PackageIDEQ(packageID)).Only(ctx)
	if err != nil {
		return nil, errors.New("app not found")
	}
	return record, nil
}

func (s *Server) mcpAppSummaries(r *http.Request, records []*entgo.App, u *entgo.User) []mcpAppSummary {
	out := make([]mcpAppSummary, 0, len(records))
	for _, record := range records {
		out = append(out, toMCPAppSummary(s.appSummaryDTO(r, record, u)))
	}
	return out
}

func toMCPAppSummary(dto appSummary) mcpAppSummary {
	latestVersion := ""
	if dto.LatestVersion != nil {
		latestVersion = dto.LatestVersion.Version
	}
	return mcpAppSummary{
		ID:               dto.ID,
		OwnerID:          dto.OwnerID,
		Owner:            dto.Owner,
		PackageID:        dto.PackageID,
		Name:             dto.Name,
		Status:           dto.Status,
		LatestVersion:    latestVersion,
		CanManageApp:     dto.CanManageApp,
		CanUploadVersion: dto.CanUploadVersion,
		UpdatedAt:        dto.UpdatedAt.Format(time.RFC3339),
	}
}

func toMCPVersionSummary(v *entgo.AppVersion) mcpVersionSummary {
	return mcpVersionSummary{
		ID:        v.ID,
		AppID:     v.AppID,
		Version:   v.Version,
		Status:    string(v.Status),
		Source:    string(v.SourceType),
		SHA256:    v.Sha256,
		FileSize:  v.FileSize,
		CreatedAt: v.CreatedAt.Format(time.RFC3339),
	}
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func clampLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
