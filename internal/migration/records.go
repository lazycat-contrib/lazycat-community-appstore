package migration

import (
	"context"
	"sort"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/ad"
	"lazycat.community/appstore/ent/announcement"
	"lazycat.community/appstore/ent/apitoken"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appdownload"
	"lazycat.community/appstore/ent/appscreenshot"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/appvote"
	"lazycat.community/appstore/ent/asset"
	"lazycat.community/appstore/ent/assetlink"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/groupmember"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/sitesetting"
	"lazycat.community/appstore/ent/storageconfig"
	"lazycat.community/appstore/ent/tag"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/ent/usergroup"
)

const (
	assetOwnerApp        = "app"
	assetOwnerSite       = "site"
	serverAssetURLPrefix = "/api/v1/assets"
	siteIconSettingKey   = "site_icon_url"
)

type SiteData struct {
	SiteSettings   []SiteSettingRecord   `json:"siteSettings,omitempty"`
	StorageConfigs []StorageConfigRecord `json:"storageConfigs,omitempty"`
	Announcements  []AnnouncementRecord  `json:"announcements,omitempty"`
	Ads            []AdRecord            `json:"ads,omitempty"`
	Assets         []AssetRecord         `json:"assets,omitempty"`
	AssetLinks     []AssetLinkRecord     `json:"assetLinks,omitempty"`
}

type PeopleData struct {
	Users        []UserRecord        `json:"users,omitempty"`
	APITokens    []APITokenRecord    `json:"apiTokens,omitempty"`
	MCPTokens    []MCPTokenRecord    `json:"mcpTokens,omitempty"`
	Groups       []UserGroupRecord   `json:"groups,omitempty"`
	GroupMembers []GroupMemberRecord `json:"groupMembers,omitempty"`
}

type AppsData struct {
	Categories      []CategoryRecord      `json:"categories,omitempty"`
	Tags            []TagRecord           `json:"tags,omitempty"`
	Apps            []AppRecord           `json:"apps,omitempty"`
	AppVersions     []AppVersionRecord    `json:"appVersions,omitempty"`
	AppDownloads    []AppDownloadRecord   `json:"appDownloads,omitempty"`
	AppScreenshots  []AppScreenshotRecord `json:"appScreenshots,omitempty"`
	AppTags         []AppTagRecord        `json:"appTags,omitempty"`
	AppVisibilities []AppVisibilityRecord `json:"appVisibilities,omitempty"`
	AppVotes        []AppVoteRecord       `json:"appVotes,omitempty"`
	Assets          []AssetRecord         `json:"assets,omitempty"`
	AssetLinks      []AssetLinkRecord     `json:"assetLinks,omitempty"`
}

type AssetRecord struct {
	ID        int       `json:"id,omitempty"`
	SHA256    string    `json:"sha256"`
	MediaType string    `json:"media_type"`
	Size      int64     `json:"size"`
	Data      []byte    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AssetLinkRecord struct {
	ID        int       `json:"id,omitempty"`
	AssetID   int       `json:"asset_id"`
	OwnerType string    `json:"owner_type"`
	OwnerID   int       `json:"owner_id"`
	Role      string    `json:"role"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SiteSettingRecord struct {
	ID        int       `json:"id,omitempty"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StorageConfigRecord struct {
	ID              int       `json:"id,omitempty"`
	Key             string    `json:"key"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	DeliveryMode    string    `json:"delivery_mode"`
	LocalPath       string    `json:"local_path"`
	EndpointURL     string    `json:"endpoint_url"`
	BucketName      string    `json:"bucket_name"`
	Region          string    `json:"region"`
	PathStyle       bool      `json:"path_style"`
	AccountID       string    `json:"account_id"`
	RootPrefix      string    `json:"root_prefix"`
	AccessKeyID     string    `json:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key"`
	WebDAVUsername  string    `json:"webdav_username"`
	WebDAVPassword  string    `json:"webdav_password"`
	PublicBaseURL   string    `json:"public_base_url"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AnnouncementRecord struct {
	ID        int        `json:"id,omitempty"`
	Enabled   bool       `json:"enabled"`
	Level     string     `json:"level"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	LinkLabel string     `json:"link_label"`
	LinkURL   string     `json:"link_url"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type AdRecord struct {
	ID        int        `json:"id,omitempty"`
	Enabled   bool       `json:"enabled"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ImageURL  string     `json:"image_url"`
	LinkLabel string     `json:"link_label"`
	LinkURL   string     `json:"link_url"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type UserRecord struct {
	ID                int       `json:"id,omitempty"`
	Username          string    `json:"username"`
	Nickname          string    `json:"nickname"`
	AvatarURL         string    `json:"avatar_url"`
	AvatarStorageKey  string    `json:"avatar_storage_key"`
	AvatarStoragePath string    `json:"avatar_storage_path"`
	Email             *string   `json:"email,omitempty"`
	PasswordHash      string    `json:"password_hash"`
	Role              string    `json:"role"`
	EmailVerified     bool      `json:"email_verified"`
	Disabled          bool      `json:"disabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type APITokenRecord struct {
	ID         int        `json:"id,omitempty"`
	UserID     int        `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	TokenHash  string     `json:"token_hash"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type MCPTokenRecord struct {
	ID            int        `json:"id,omitempty"`
	UserID        int        `json:"user_id"`
	PrincipalType string     `json:"principal_type"`
	Note          string     `json:"note"`
	Prefix        string     `json:"prefix"`
	TokenHash     string     `json:"token_hash"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type UserGroupRecord struct {
	ID            int       `json:"id,omitempty"`
	OwnerID       int       `json:"owner_id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	Code          string    `json:"code"`
	CodeUpdatedAt time.Time `json:"code_updated_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GroupMemberRecord struct {
	ID        int       `json:"id,omitempty"`
	GroupID   int       `json:"group_id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CategoryRecord struct {
	ID        int       `json:"id,omitempty"`
	Name      string    `json:"name"`
	NameI18n  string    `json:"name_i18n"`
	Slug      string    `json:"slug"`
	ParentID  *int      `json:"parent_id,omitempty"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TagRecord struct {
	ID        int       `json:"id,omitempty"`
	Name      string    `json:"name"`
	NameI18n  string    `json:"name_i18n"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AppRecord struct {
	ID                        int       `json:"id,omitempty"`
	OwnerID                   int       `json:"owner_id"`
	CategoryID                *int      `json:"category_id,omitempty"`
	PackageID                 string    `json:"package_id"`
	Name                      string    `json:"name"`
	NameI18nJSON              string    `json:"name_i18n_json"`
	Slug                      string    `json:"slug"`
	Summary                   string    `json:"summary"`
	SummaryI18nJSON           string    `json:"summary_i18n_json"`
	Description               string    `json:"description"`
	DescriptionI18nJSON       string    `json:"description_i18n_json"`
	Author                    string    `json:"author,omitempty"`
	Homepage                  string    `json:"homepage,omitempty"`
	License                   string    `json:"license,omitempty"`
	MinOSVersion              string    `json:"min_os_version,omitempty"`
	IconURL                   *string   `json:"icon_url,omitempty"`
	Status                    string    `json:"status"`
	AllowUnreviewedUpdates    bool      `json:"allow_unreviewed_updates"`
	CommentsEnabled           bool      `json:"comments_enabled"`
	EmailNotificationsEnabled bool      `json:"email_notifications_enabled"`
	InstallPasswordHash       string    `json:"install_password_hash"`
	DownloadCount             int       `json:"download_count"`
	VersionRetentionCount     *int      `json:"version_retention_count,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

type AppVersionRecord struct {
	ID          int        `json:"id,omitempty"`
	AppID       int        `json:"app_id"`
	UploaderID  int        `json:"uploader_id"`
	Version     string     `json:"version"`
	Changelog   string     `json:"changelog"`
	Status      string     `json:"status"`
	SourceType  string     `json:"source_type"`
	DownloadURL string     `json:"download_url"`
	StorageKey  string     `json:"storage_key"`
	StoragePath string     `json:"storage_path"`
	FileSize    int64      `json:"file_size"`
	SHA256      string     `json:"sha256"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type AppDownloadRecord struct {
	ID              int       `json:"id,omitempty"`
	AppID           int       `json:"app_id"`
	Version         string    `json:"version,omitempty"`
	LegacyVersionID int       `json:"version_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type AppScreenshotRecord struct {
	ID          int       `json:"id,omitempty"`
	AppID       int       `json:"app_id"`
	UploaderID  int       `json:"uploader_id"`
	ImageURL    string    `json:"image_url"`
	StorageKey  string    `json:"storage_key"`
	StoragePath string    `json:"storage_path"`
	Caption     string    `json:"caption"`
	DeviceType  string    `json:"device_type"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

type AppTagRecord struct {
	ID        int       `json:"id,omitempty"`
	AppID     int       `json:"app_id"`
	TagID     int       `json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

type AppVisibilityRecord struct {
	ID        int       `json:"id,omitempty"`
	AppID     int       `json:"app_id"`
	GroupID   int       `json:"group_id"`
	CreatedAt time.Time `json:"created_at"`
}

type AppVoteRecord struct {
	ID        int       `json:"id,omitempty"`
	AppID     int       `json:"app_id"`
	UserID    int       `json:"user_id"`
	Value     int       `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func collectSiteData(ctx context.Context, db *ent.Client) (SiteData, error) {
	var data SiteData
	if err := db.SiteSetting.Query().Select(sitesetting.FieldID, sitesetting.FieldKey, sitesetting.FieldValue, sitesetting.FieldCreatedAt, sitesetting.FieldUpdatedAt).Scan(ctx, &data.SiteSettings); err != nil {
		return data, err
	}
	if err := db.StorageConfig.Query().Select(storageconfig.FieldID, storageconfig.FieldKey, storageconfig.FieldName, storageconfig.FieldProvider, storageconfig.FieldDeliveryMode, storageconfig.FieldLocalPath, storageconfig.FieldEndpointURL, storageconfig.FieldBucketName, storageconfig.FieldRegion, storageconfig.FieldPathStyle, storageconfig.FieldAccountID, storageconfig.FieldRootPrefix, storageconfig.FieldAccessKeyID, storageconfig.FieldSecretAccessKey, storageconfig.FieldWebdavUsername, storageconfig.FieldWebdavPassword, storageconfig.FieldPublicBaseURL, storageconfig.FieldCreatedAt, storageconfig.FieldUpdatedAt).Scan(ctx, &data.StorageConfigs); err != nil {
		return data, err
	}
	if err := db.Announcement.Query().Select(announcement.FieldID, announcement.FieldEnabled, announcement.FieldLevel, announcement.FieldTitle, announcement.FieldBody, announcement.FieldLinkLabel, announcement.FieldLinkURL, announcement.FieldStartsAt, announcement.FieldEndsAt, announcement.FieldSortOrder, announcement.FieldCreatedAt, announcement.FieldUpdatedAt).Scan(ctx, &data.Announcements); err != nil {
		return data, err
	}
	if err := db.Ad.Query().Select(ad.FieldID, ad.FieldEnabled, ad.FieldTitle, ad.FieldBody, ad.FieldImageURL, ad.FieldLinkLabel, ad.FieldLinkURL, ad.FieldStartsAt, ad.FieldEndsAt, ad.FieldSortOrder, ad.FieldCreatedAt, ad.FieldUpdatedAt).Scan(ctx, &data.Ads); err != nil {
		return data, err
	}
	links, assets, err := collectAssetRecords(ctx, db, assetOwnerSite)
	if err != nil {
		return data, err
	}
	data.AssetLinks = links
	data.Assets = assets
	return data, nil
}

func collectPeopleData(ctx context.Context, db *ent.Client) (PeopleData, error) {
	var data PeopleData
	if err := db.User.Query().Select(user.FieldID, user.FieldUsername, user.FieldNickname, user.FieldAvatarURL, user.FieldAvatarStorageKey, user.FieldAvatarStoragePath, user.FieldEmail, user.FieldPasswordHash, user.FieldRole, user.FieldEmailVerified, user.FieldDisabled, user.FieldCreatedAt, user.FieldUpdatedAt).Scan(ctx, &data.Users); err != nil {
		return data, err
	}
	if err := db.APIToken.Query().Select(apitoken.FieldID, apitoken.FieldUserID, apitoken.FieldName, apitoken.FieldPrefix, apitoken.FieldTokenHash, apitoken.FieldLastUsedAt, apitoken.FieldCreatedAt).Scan(ctx, &data.APITokens); err != nil {
		return data, err
	}
	if err := db.MCPToken.Query().Select(mcptoken.FieldID, mcptoken.FieldUserID, mcptoken.FieldPrincipalType, mcptoken.FieldNote, mcptoken.FieldPrefix, mcptoken.FieldTokenHash, mcptoken.FieldExpiresAt, mcptoken.FieldLastUsedAt, mcptoken.FieldCreatedAt).Scan(ctx, &data.MCPTokens); err != nil {
		return data, err
	}
	if err := db.UserGroup.Query().Select(usergroup.FieldID, usergroup.FieldOwnerID, usergroup.FieldName, usergroup.FieldSlug, usergroup.FieldDescription, usergroup.FieldCode, usergroup.FieldCodeUpdatedAt, usergroup.FieldCreatedAt, usergroup.FieldUpdatedAt).Scan(ctx, &data.Groups); err != nil {
		return data, err
	}
	if err := db.GroupMember.Query().Select(groupmember.FieldID, groupmember.FieldGroupID, groupmember.FieldUserID, groupmember.FieldCreatedAt).Scan(ctx, &data.GroupMembers); err != nil {
		return data, err
	}
	return data, nil
}

func collectAppsData(ctx context.Context, db *ent.Client) (AppsData, error) {
	var data AppsData
	if err := db.Category.Query().Select(category.FieldID, category.FieldName, category.FieldNameI18n, category.FieldSlug, category.FieldParentID, category.FieldSortOrder, category.FieldCreatedAt, category.FieldUpdatedAt).Scan(ctx, &data.Categories); err != nil {
		return data, err
	}
	if err := db.Tag.Query().Select(tag.FieldID, tag.FieldName, tag.FieldNameI18n, tag.FieldSlug, tag.FieldCreatedAt, tag.FieldUpdatedAt).Scan(ctx, &data.Tags); err != nil {
		return data, err
	}
	if err := db.App.Query().Select(app.FieldID, app.FieldOwnerID, app.FieldCategoryID, app.FieldPackageID, app.FieldName, app.FieldNameI18nJSON, app.FieldSlug, app.FieldSummary, app.FieldSummaryI18nJSON, app.FieldDescription, app.FieldDescriptionI18nJSON, app.FieldAuthor, app.FieldHomepage, app.FieldLicense, app.FieldMinOsVersion, app.FieldIconURL, app.FieldStatus, app.FieldAllowUnreviewedUpdates, app.FieldCommentsEnabled, app.FieldEmailNotificationsEnabled, app.FieldInstallPasswordHash, app.FieldDownloadCount, app.FieldVersionRetentionCount, app.FieldCreatedAt, app.FieldUpdatedAt).Scan(ctx, &data.Apps); err != nil {
		return data, err
	}
	if err := db.AppVersion.Query().Select(appversion.FieldID, appversion.FieldAppID, appversion.FieldUploaderID, appversion.FieldVersion, appversion.FieldChangelog, appversion.FieldStatus, appversion.FieldSourceType, appversion.FieldDownloadURL, appversion.FieldStorageKey, appversion.FieldStoragePath, appversion.FieldFileSize, appversion.FieldSha256, appversion.FieldPublishedAt, appversion.FieldCreatedAt, appversion.FieldUpdatedAt).Scan(ctx, &data.AppVersions); err != nil {
		return data, err
	}
	if err := db.AppDownload.Query().Select(appdownload.FieldID, appdownload.FieldAppID, appdownload.FieldVersion, appdownload.FieldCreatedAt).Scan(ctx, &data.AppDownloads); err != nil {
		return data, err
	}
	if err := db.AppScreenshot.Query().Select(appscreenshot.FieldID, appscreenshot.FieldAppID, appscreenshot.FieldUploaderID, appscreenshot.FieldImageURL, appscreenshot.FieldStorageKey, appscreenshot.FieldStoragePath, appscreenshot.FieldCaption, appscreenshot.FieldDeviceType, appscreenshot.FieldSortOrder, appscreenshot.FieldCreatedAt).Scan(ctx, &data.AppScreenshots); err != nil {
		return data, err
	}
	if err := db.AppTag.Query().Select(apptag.FieldID, apptag.FieldAppID, apptag.FieldTagID, apptag.FieldCreatedAt).Scan(ctx, &data.AppTags); err != nil {
		return data, err
	}
	if err := db.AppVisibility.Query().Select(appvisibility.FieldID, appvisibility.FieldAppID, appvisibility.FieldGroupID, appvisibility.FieldCreatedAt).Scan(ctx, &data.AppVisibilities); err != nil {
		return data, err
	}
	if err := db.AppVote.Query().Select(appvote.FieldID, appvote.FieldAppID, appvote.FieldUserID, appvote.FieldValue, appvote.FieldCreatedAt, appvote.FieldUpdatedAt).Scan(ctx, &data.AppVotes); err != nil {
		return data, err
	}
	links, assets, err := collectAssetRecords(ctx, db, assetOwnerApp)
	if err != nil {
		return data, err
	}
	data.AssetLinks = links
	data.Assets = assets
	return data, nil
}

func collectAssetRecords(ctx context.Context, db *ent.Client, ownerType string) ([]AssetLinkRecord, []AssetRecord, error) {
	var links []AssetLinkRecord
	if err := db.AssetLink.Query().
		Where(assetlink.OwnerTypeEQ(ownerType)).
		Order(ent.Asc(assetlink.FieldOwnerID), ent.Asc(assetlink.FieldRole), ent.Asc(assetlink.FieldSortOrder), ent.Asc(assetlink.FieldID)).
		Select(assetlink.FieldID, assetlink.FieldAssetID, assetlink.FieldOwnerType, assetlink.FieldOwnerID, assetlink.FieldRole, assetlink.FieldSortOrder, assetlink.FieldCreatedAt, assetlink.FieldUpdatedAt).
		Scan(ctx, &links); err != nil {
		return nil, nil, err
	}
	if len(links) == 0 {
		return links, nil, nil
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(links))
	for _, link := range links {
		if link.AssetID <= 0 {
			continue
		}
		if _, ok := seen[link.AssetID]; ok {
			continue
		}
		seen[link.AssetID] = struct{}{}
		ids = append(ids, link.AssetID)
	}
	sort.Ints(ids)
	var assets []AssetRecord
	if len(ids) > 0 {
		if err := db.Asset.Query().
			Where(asset.IDIn(ids...)).
			Order(ent.Asc(asset.FieldID)).
			Select(asset.FieldID, asset.FieldSha256, asset.FieldMediaType, asset.FieldSize, asset.FieldData, asset.FieldCreatedAt, asset.FieldUpdatedAt).
			Scan(ctx, &assets); err != nil {
			return nil, nil, err
		}
	}
	return links, assets, nil
}

func siteCounts(data SiteData) map[string]int {
	return map[string]int{
		"siteSettings":   len(data.SiteSettings),
		"storageConfigs": len(data.StorageConfigs),
		"announcements":  len(data.Announcements),
		"ads":            len(data.Ads),
		"siteAssets":     len(data.Assets),
		"siteAssetLinks": len(data.AssetLinks),
	}
}

func peopleCounts(data PeopleData) map[string]int {
	return map[string]int{
		"users":        len(data.Users),
		"apiTokens":    len(data.APITokens),
		"mcpTokens":    len(data.MCPTokens),
		"groups":       len(data.Groups),
		"groupMembers": len(data.GroupMembers),
	}
}

func appsCounts(data AppsData) map[string]int {
	return map[string]int{
		"categories":      len(data.Categories),
		"tags":            len(data.Tags),
		"apps":            len(data.Apps),
		"versions":        len(data.AppVersions),
		"downloads":       len(data.AppDownloads),
		"screenshots":     len(data.AppScreenshots),
		"appTags":         len(data.AppTags),
		"appVisibilities": len(data.AppVisibilities),
		"appVotes":        len(data.AppVotes),
		"appAssets":       len(data.Assets),
		"appAssetLinks":   len(data.AssetLinks),
	}
}

func mergeCounts(target map[string]int, source map[string]int) {
	for key, value := range source {
		target[key] = value
	}
}
