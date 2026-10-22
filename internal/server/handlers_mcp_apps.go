package server

import (
	"context"
	"errors"
	"math"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/internal/pagination"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpAppTargetInput struct {
	AppID     int    `json:"appId,omitzero" jsonschema:"app ID; provide either appId or packageId"`
	PackageID string `json:"packageId,omitempty" jsonschema:"package ID; provide either appId or packageId"`
}

type mcpManagedAppOutput struct {
	App              mcpAppSummary          `json:"app"`
	VersionRetention versionRetentionPolicy `json:"versionRetention"`
}

type mcpDeleteResult struct {
	OK bool `json:"ok"`
}

type mcpAdminUpdateAppInput struct {
	AppID                     int     `json:"appId,omitzero"`
	PackageID                 string  `json:"packageId,omitempty"`
	Name                      *string `json:"name,omitempty" jsonschema:"new display name; must not be blank"`
	Summary                   *string `json:"summary,omitempty"`
	Description               *string `json:"description,omitempty"`
	CommentsEnabled           *bool   `json:"commentsEnabled,omitempty"`
	EmailNotificationsEnabled *bool   `json:"emailNotificationsEnabled,omitempty"`
	AllowUnreviewedUpdates    *bool   `json:"allowUnreviewedUpdates,omitempty"`
	InstallPassword           *string `json:"installPassword,omitempty" jsonschema:"replacement install password; empty clears it; never returned"`
}

type mcpAdminDeleteAppInput struct {
	AppID     int    `json:"appId,omitzero"`
	PackageID string `json:"packageId,omitempty"`
	Confirm   bool   `json:"confirm" jsonschema:"must be true to permanently delete this app and its related data"`
}

type mcpAdminListVersionsInput struct {
	AppID     int    `json:"appId,omitzero"`
	PackageID string `json:"packageId,omitempty"`
	Page      int    `json:"page,omitzero"`
	Limit     int    `json:"limit,omitzero" jsonschema:"page size; default 50, maximum 200"`
}

type mcpVersionsOutput struct {
	Versions   []mcpVersionSummary `json:"versions"`
	Pagination pagination.Response `json:"pagination"`
	NextPage   int                 `json:"nextPage,omitzero"`
}

type mcpAdminUpdateVersionInput struct {
	AppID     int    `json:"appId,omitzero"`
	PackageID string `json:"packageId,omitempty"`
	VersionID int    `json:"versionId"`
	Changelog string `json:"changelog" jsonschema:"replacement release notes; empty clears them; package contents and version number are immutable"`
}

type mcpManagedVersionOutput struct {
	Version mcpVersionSummary `json:"version"`
}

type mcpAdminDeleteVersionInput struct {
	AppID     int    `json:"appId,omitzero"`
	PackageID string `json:"packageId,omitempty"`
	VersionID int    `json:"versionId"`
	Confirm   bool   `json:"confirm" jsonschema:"must be true to permanently delete this version"`
}

type mcpDeletedVersionOutput struct {
	DeletedVersion deletedVersionResult `json:"deletedVersion"`
}

type mcpAdminSetVersionRetentionInput struct {
	AppID       int    `json:"appId,omitzero"`
	PackageID   string `json:"packageId,omitempty"`
	Mode        string `json:"mode" jsonschema:"INHERIT uses site policy; CUSTOM requires maxVersions"`
	MaxVersions *int   `json:"maxVersions,omitempty" jsonschema:"CUSTOM only; zero keeps all versions, positive values prune old approved versions immediately"`
	Confirm     bool   `json:"confirm" jsonschema:"must be true; changing retention can permanently delete old approved versions"`
}

type mcpVersionRetentionOutput struct {
	VersionRetention versionRetentionPolicy `json:"versionRetention"`
	PrunedVersions   []deletedVersionResult `json:"prunedVersions"`
}

func (s *Server) registerAdminAppMCPTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_get_app", Title: "Get app management details", Description: "Admin-only app details, including unpublished apps and version retention. Provide appId or packageId, not both.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, s.mcpAdminGetApp)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_update_app", Title: "Edit app information", Description: "Admin-only partial update of name, summary, description, comments, email notifications, update review policy or install password. Omitted fields and review status are preserved. Use existing taxonomy tools for category and tags."}, s.mcpAdminUpdateApp)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_delete_app", Title: "Permanently delete app", Description: "Admin-only permanent deletion of an app, its versions, screenshots and related records. Requires confirm=true. Use unlist instead when you only want to hide it.", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)}}, s.mcpAdminDeleteApp)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_unlist_app", Title: "Unlist app", Description: "Admin-only removal from the public store without deleting the app or versions."}, s.mcpAdminUnlistApp)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_list_versions", Title: "List all app versions", Description: "Admin-only paginated versions including pending and rejected versions. Follow nextPage until absent.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, s.mcpAdminListVersions)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_update_version", Title: "Edit version release notes", Description: "Admin-only update of version changelog. Version numbers, package bytes, URLs and checksums remain immutable; publish a new version to replace a package."}, s.mcpAdminUpdateVersion)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_delete_version", Title: "Permanently delete version", Description: "Admin-only permanent deletion of one version belonging to the specified app. Requires confirm=true. Returns storage cleanup warnings if any.", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)}}, s.mcpAdminDeleteVersion)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_set_version_retention", Title: "Set app version retention", Description: "Admin-only INHERIT or CUSTOM retention policy. Requires confirm=true because old approved versions may be deleted immediately. CUSTOM maxVersions=0 keeps all versions.", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)}}, s.mcpAdminSetVersionRetention)
}

func (s *Server) mcpAdminApp(ctx context.Context, req *mcp.CallToolRequest, appID int, packageID string) (*entgo.App, *entgo.User, error) {
	u, principal, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	if principal != mcptoken.PrincipalTypeADMIN {
		return nil, nil, errors.New("admin MCP token is required")
	}
	if appID < 0 {
		return nil, nil, errors.New("appId must be positive")
	}
	record, err := s.mcpTaxonomyApp(ctx, appID, packageID)
	return record, u, err
}

func (s *Server) mcpManagedApp(ctx context.Context, record *entgo.App, u *entgo.User) mcpManagedAppOutput {
	return mcpManagedAppOutput{App: toMCPAppSummary(s.appSummaryDTO(mcpRequest(ctx), record, u)), VersionRetention: s.versionRetentionPolicyForApp(ctx, record)}
}

func (s *Server) mcpAdminGetApp(ctx context.Context, req *mcp.CallToolRequest, input mcpAppTargetInput) (*mcp.CallToolResult, mcpManagedAppOutput, error) {
	record, u, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpManagedAppOutput{}, err
	}
	return nil, s.mcpManagedApp(ctx, record, u), nil
}

func (s *Server) mcpAdminUpdateApp(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminUpdateAppInput) (*mcp.CallToolResult, mcpManagedAppOutput, error) {
	record, u, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpManagedAppOutput{}, err
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return nil, mcpManagedAppOutput{}, errors.New("name must not be blank")
	}
	updated, err := s.applyAppInfoUpdate(mcpRequest(ctx), record.ID, updateAppJSON{
		Name: input.Name, Summary: input.Summary, Description: input.Description, CommentsEnabled: input.CommentsEnabled,
		EmailNotificationsEnabled: input.EmailNotificationsEnabled, AllowUnreviewedUpdates: input.AllowUnreviewedUpdates, InstallPassword: input.InstallPassword,
	})
	if err != nil {
		return nil, mcpManagedAppOutput{}, err
	}
	s.invalidateSourceFeed()
	return nil, s.mcpManagedApp(ctx, updated, u), nil
}

func (s *Server) mcpAdminUnlistApp(ctx context.Context, req *mcp.CallToolRequest, input mcpAppTargetInput) (*mcp.CallToolResult, mcpManagedAppOutput, error) {
	record, u, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpManagedAppOutput{}, err
	}
	updated, err := s.db.App.UpdateOneID(record.ID).SetStatus(app.StatusUNLISTED).Save(ctx)
	if err != nil {
		return nil, mcpManagedAppOutput{}, err
	}
	s.invalidateSourceFeed()
	return nil, s.mcpManagedApp(ctx, updated, u), nil
}

func (s *Server) mcpAdminDeleteApp(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminDeleteAppInput) (*mcp.CallToolResult, mcpDeleteResult, error) {
	record, _, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpDeleteResult{}, err
	}
	if !input.Confirm {
		return nil, mcpDeleteResult{}, errors.New("confirm must be true to delete an app")
	}
	if err := s.deleteAppData(mcpRequest(ctx), record.ID); err != nil {
		return nil, mcpDeleteResult{}, err
	}
	s.invalidateSourceFeed()
	return nil, mcpDeleteResult{OK: true}, nil
}

func (s *Server) mcpAdminListVersions(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminListVersionsInput) (*mcp.CallToolResult, mcpVersionsOutput, error) {
	record, _, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpVersionsOutput{}, err
	}
	if input.Page < 0 {
		return nil, mcpVersionsOutput{}, errors.New("page must be at least 1")
	}
	page := max(1, input.Page)
	limit := clampLimit(input.Limit, 50, 200)
	if page-1 > math.MaxInt/limit {
		return nil, mcpVersionsOutput{}, errors.New("page is too large")
	}
	query := s.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID))
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, mcpVersionsOutput{}, err
	}
	versions, err := query.Order(orderVersionsBySoftwareUpdate(), entgo.Desc(appversion.FieldID)).Offset((page - 1) * limit).Limit(limit).All(ctx)
	if err != nil {
		return nil, mcpVersionsOutput{}, err
	}
	out := mcpVersionsOutput{Versions: make([]mcpVersionSummary, 0, len(versions)), Pagination: (pagination.Params{Page: page, PageSize: limit}).Response(total)}
	for _, version := range versions {
		out.Versions = append(out.Versions, toMCPVersionSummary(version))
	}
	if page < out.Pagination.TotalPages {
		out.NextPage = page + 1
	}
	return nil, out, nil
}

func (s *Server) mcpAdminUpdateVersion(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminUpdateVersionInput) (*mcp.CallToolResult, mcpManagedVersionOutput, error) {
	record, _, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpManagedVersionOutput{}, err
	}
	version, err := s.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID), appversion.IDEQ(input.VersionID)).Only(ctx)
	if err != nil {
		return nil, mcpManagedVersionOutput{}, err
	}
	updated, err := s.db.AppVersion.UpdateOne(version).SetChangelog(input.Changelog).Save(ctx)
	if err != nil {
		return nil, mcpManagedVersionOutput{}, err
	}
	s.invalidateSourceFeed()
	return nil, mcpManagedVersionOutput{Version: toMCPVersionSummary(updated)}, nil
}

func (s *Server) mcpAdminDeleteVersion(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminDeleteVersionInput) (*mcp.CallToolResult, mcpDeletedVersionOutput, error) {
	record, _, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpDeletedVersionOutput{}, err
	}
	if !input.Confirm {
		return nil, mcpDeletedVersionOutput{}, errors.New("confirm must be true to delete a version")
	}
	deleted, err := s.deleteAppVersion(ctx, record.ID, input.VersionID)
	if err != nil {
		return nil, mcpDeletedVersionOutput{}, err
	}
	s.invalidateSourceFeed()
	return nil, mcpDeletedVersionOutput{DeletedVersion: deleted}, nil
}

func (s *Server) mcpAdminSetVersionRetention(ctx context.Context, req *mcp.CallToolRequest, input mcpAdminSetVersionRetentionInput) (*mcp.CallToolResult, mcpVersionRetentionOutput, error) {
	record, _, err := s.mcpAdminApp(ctx, req, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpVersionRetentionOutput{}, err
	}
	if !input.Confirm {
		return nil, mcpVersionRetentionOutput{}, errors.New("confirm must be true because retention can delete versions")
	}
	switch strings.ToUpper(strings.TrimSpace(input.Mode)) {
	case "INHERIT":
		if input.MaxVersions != nil {
			return nil, mcpVersionRetentionOutput{}, errors.New("maxVersions must be omitted when mode is INHERIT")
		}
	case "CUSTOM":
		if input.MaxVersions == nil || *input.MaxVersions < 0 {
			return nil, mcpVersionRetentionOutput{}, errors.New("CUSTOM requires a non-negative maxVersions")
		}
	default:
		return nil, mcpVersionRetentionOutput{}, errors.New("mode must be INHERIT or CUSTOM")
	}
	policy, pruned, err := s.updateAppVersionRetention(ctx, record.ID, input.MaxVersions)
	if err != nil {
		return nil, mcpVersionRetentionOutput{}, err
	}
	s.invalidateSourceFeed()
	return nil, mcpVersionRetentionOutput{VersionRetention: policy, PrunedVersions: pruned}, nil
}
