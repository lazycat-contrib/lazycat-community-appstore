package migration

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/ad"
	"lazycat.community/appstore/ent/announcement"
	"lazycat.community/appstore/ent/apitoken"
	apppkg "lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appdownload"
	"lazycat.community/appstore/ent/appscreenshot"
	"lazycat.community/appstore/ent/apptag"
	versionpkg "lazycat.community/appstore/ent/appversion"
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
	"lazycat.community/appstore/internal/assetdata"
)

type Importer struct {
	db      *ent.Client
	storage StorageResolver
}

func NewImporter(db *ent.Client, storage StorageResolver) *Importer {
	return &Importer{db: db, storage: storage}
}

func (i *Importer) Preview(ctx context.Context, r io.ReaderAt, size int64) (*Preview, error) {
	_ = ctx
	if i == nil {
		return nil, fmt.Errorf("migration importer is not configured")
	}
	manifest, err := readManifestFromReaderAt(r, size)
	if err != nil {
		return nil, err
	}
	return previewFromManifest(manifest), nil
}

func (i *Importer) Import(ctx context.Context, r io.ReaderAt, size int64, options ImportOptions) (*ImportResult, error) {
	if i == nil || i.db == nil {
		return nil, fmt.Errorf("migration importer is not configured")
	}
	options.Options = NormalizeOptions(options.Options)
	if options.Mode == "" {
		options.Mode = ImportModeMerge
	}
	if options.Mode != ImportModeMerge && options.Mode != ImportModeReplace {
		return nil, fmt.Errorf("unsupported import mode")
	}
	if options.Mode == ImportModeReplace && options.ConfirmReplace != "OVERWRITE" && options.ConfirmReplace != "RESTORE" {
		return nil, fmt.Errorf("replace import requires confirmation")
	}
	zr, manifest, siteData, peopleData, appsData, err := readImportPackage(r, size)
	if err != nil {
		return nil, err
	}
	manifestOptions := OptionsFromModules(manifest.Modules)
	if !options.IncludeSite && !options.IncludePeople && !options.IncludeApps && !options.IncludeFiles {
		options.Options = manifestOptions
	}
	if options.IncludeFiles && !manifestOptions.IncludeFiles {
		options.IncludeFiles = false
	}

	pathMap := map[string]string{}
	warnings := append([]string{}, manifest.Warnings...)
	if options.IncludeFiles {
		var fileWarnings []string
		pathMap, fileWarnings, err = importFiles(ctx, zr, i.storage, manifest)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, fileWarnings...)
	}

	tx, err := i.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	result := &ImportResult{Mode: options.Mode, Warnings: warnings}
	if options.Mode == ImportModeReplace {
		if err := replaceSelectedData(ctx, client, options); err != nil {
			return nil, err
		}
	}
	maps := newImportMaps()
	if options.IncludeSite {
		if err := importSiteData(ctx, client, siteData, maps, result); err != nil {
			return nil, err
		}
	}
	if options.IncludePeople {
		if err := importPeopleData(ctx, client, peopleData, maps, result); err != nil {
			return nil, err
		}
	}
	if options.IncludeApps {
		if err := importAppsData(ctx, client, appsData, maps, pathMap, options.ActorUserID, result); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func readImportPackage(r io.ReaderAt, size int64) (*zip.Reader, Manifest, SiteData, PeopleData, AppsData, error) {
	zr, err := zipReaderFromReaderAt(r, size)
	if err != nil {
		return nil, Manifest{}, SiteData{}, PeopleData{}, AppsData{}, err
	}
	manifest, err := readManifest(zr)
	if err != nil {
		return nil, Manifest{}, SiteData{}, PeopleData{}, AppsData{}, err
	}
	options := OptionsFromModules(manifest.Modules)
	var siteData SiteData
	var peopleData PeopleData
	var appsData AppsData
	if options.IncludeSite {
		if err := readJSONEntry(zr, "data/site.json", &siteData); err != nil {
			return nil, Manifest{}, SiteData{}, PeopleData{}, AppsData{}, err
		}
	}
	if options.IncludePeople {
		if err := readJSONEntry(zr, "data/people.json", &peopleData); err != nil {
			return nil, Manifest{}, SiteData{}, PeopleData{}, AppsData{}, err
		}
	}
	if options.IncludeApps {
		if err := readJSONEntry(zr, "data/apps.json", &appsData); err != nil {
			return nil, Manifest{}, SiteData{}, PeopleData{}, AppsData{}, err
		}
	}
	return zr, manifest, siteData, peopleData, appsData, nil
}

type importMaps struct {
	users      map[int]int
	categories map[int]int
	tags       map[int]int
	groups     map[int]int
	apps       map[int]int
	versions   map[int]int
	assets     map[int]int
}

func newImportMaps() *importMaps {
	return &importMaps{
		users:      map[int]int{},
		categories: map[int]int{},
		tags:       map[int]int{},
		groups:     map[int]int{},
		apps:       map[int]int{},
		versions:   map[int]int{},
		assets:     map[int]int{},
	}
}

func replaceSelectedData(ctx context.Context, db *ent.Client, options ImportOptions) error {
	if options.IncludeApps {
		if _, err := db.AssetLink.Delete().Where(assetlink.OwnerTypeEQ(assetOwnerApp)).Exec(ctx); err != nil {
			return err
		}
		if _, err := db.AppVisibility.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.AppTag.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.AppScreenshot.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.AppDownload.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.AppVote.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.AppVersion.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.App.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.Tag.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.Category.Delete().Exec(ctx); err != nil {
			return err
		}
	}
	if options.IncludePeople {
		if _, err := db.AppVote.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.GroupMember.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.UserGroup.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.APIToken.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.MCPToken.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.User.Delete().Exec(ctx); err != nil {
			return err
		}
	}
	if options.IncludeSite {
		if _, err := db.AssetLink.Delete().Where(assetlink.OwnerTypeEQ(assetOwnerSite)).Exec(ctx); err != nil {
			return err
		}
		if _, err := db.Ad.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.Announcement.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.StorageConfig.Delete().Exec(ctx); err != nil {
			return err
		}
		if _, err := db.SiteSetting.Delete().Exec(ctx); err != nil {
			return err
		}
	}
	if options.IncludeApps || options.IncludeSite {
		if err := cleanupUnlinkedAssets(ctx, db); err != nil {
			return err
		}
	}
	return nil
}

func importSiteData(ctx context.Context, db *ent.Client, data SiteData, maps *importMaps, result *ImportResult) error {
	if err := importAssetRecords(ctx, db, data.Assets, maps, result); err != nil {
		return err
	}
	for _, record := range data.SiteSettings {
		if record.Key == siteIconSettingKey {
			value, err := remapServerAssetURL(record.Value, maps)
			if err != nil {
				return err
			}
			record.Value = value
		}
		existing, err := db.SiteSetting.Query().Where(sitesetting.KeyEQ(record.Key)).Only(ctx)
		if err == nil {
			if _, err := db.SiteSetting.UpdateOneID(existing.ID).SetValue(record.Value).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx); err != nil {
				return err
			}
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		if _, err := db.SiteSetting.Create().SetKey(record.Key).SetValue(record.Value).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.StorageConfigs {
		existing, err := db.StorageConfig.Query().Where(storageconfig.KeyEQ(record.Key)).Only(ctx)
		if err == nil {
			if _, err := applyStorageConfigUpdate(db.StorageConfig.UpdateOneID(existing.ID), record).Save(ctx); err != nil {
				return err
			}
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		if _, err := applyStorageConfigCreate(db.StorageConfig.Create(), record).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.Announcements {
		exists, err := db.Announcement.Query().Where(announcement.TitleEQ(record.Title), announcement.BodyEQ(record.Body), announcement.CreatedAtEQ(record.CreatedAt)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := applyAnnouncementCreate(db.Announcement.Create(), record).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.Ads {
		exists, err := db.Ad.Query().Where(ad.TitleEQ(record.Title), ad.BodyEQ(record.Body), ad.ImageURLEQ(record.ImageURL), ad.CreatedAtEQ(record.CreatedAt)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := applyAdCreate(db.Ad.Create(), record).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	if err := importAssetLinks(ctx, db, assetOwnerSite, data.AssetLinks, maps, func(ownerID int) (int, bool) {
		return 0, ownerID == 0
	}, result); err != nil {
		return err
	}
	return nil
}

func applyStorageConfigCreate(create *ent.StorageConfigCreate, record StorageConfigRecord) *ent.StorageConfigCreate {
	return create.SetKey(record.Key).SetName(record.Name).SetProvider(storageconfig.Provider(record.Provider)).SetDeliveryMode(storageconfig.DeliveryMode(record.DeliveryMode)).SetLocalPath(record.LocalPath).SetEndpointURL(record.EndpointURL).SetBucketName(record.BucketName).SetRegion(record.Region).SetPathStyle(record.PathStyle).SetAccountID(record.AccountID).SetRootPrefix(record.RootPrefix).SetAccessKeyID(record.AccessKeyID).SetSecretAccessKey(record.SecretAccessKey).SetWebdavUsername(record.WebDAVUsername).SetWebdavPassword(record.WebDAVPassword).SetPublicBaseURL(record.PublicBaseURL).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
}

func applyStorageConfigUpdate(update *ent.StorageConfigUpdateOne, record StorageConfigRecord) *ent.StorageConfigUpdateOne {
	return update.SetName(record.Name).SetProvider(storageconfig.Provider(record.Provider)).SetDeliveryMode(storageconfig.DeliveryMode(record.DeliveryMode)).SetLocalPath(record.LocalPath).SetEndpointURL(record.EndpointURL).SetBucketName(record.BucketName).SetRegion(record.Region).SetPathStyle(record.PathStyle).SetAccountID(record.AccountID).SetRootPrefix(record.RootPrefix).SetAccessKeyID(record.AccessKeyID).SetSecretAccessKey(record.SecretAccessKey).SetWebdavUsername(record.WebDAVUsername).SetWebdavPassword(record.WebDAVPassword).SetPublicBaseURL(record.PublicBaseURL).SetUpdatedAt(record.UpdatedAt)
}

func applyAnnouncementCreate(create *ent.AnnouncementCreate, record AnnouncementRecord) *ent.AnnouncementCreate {
	return create.SetEnabled(record.Enabled).SetLevel(announcement.Level(record.Level)).SetTitle(record.Title).SetBody(record.Body).SetLinkLabel(record.LinkLabel).SetLinkURL(record.LinkURL).SetNillableStartsAt(record.StartsAt).SetNillableEndsAt(record.EndsAt).SetSortOrder(record.SortOrder).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
}

func applyAdCreate(create *ent.AdCreate, record AdRecord) *ent.AdCreate {
	return create.SetEnabled(record.Enabled).SetTitle(record.Title).SetBody(record.Body).SetImageURL(record.ImageURL).SetLinkLabel(record.LinkLabel).SetLinkURL(record.LinkURL).SetNillableStartsAt(record.StartsAt).SetNillableEndsAt(record.EndsAt).SetSortOrder(record.SortOrder).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
}

func importPeopleData(ctx context.Context, db *ent.Client, data PeopleData, maps *importMaps, result *ImportResult) error {
	for _, record := range data.Users {
		existing, err := db.User.Query().Where(user.UsernameEQ(record.Username)).Only(ctx)
		if err == nil {
			update := db.User.UpdateOneID(existing.ID).SetNickname(record.Nickname).SetAvatarURL(record.AvatarURL).SetAvatarStorageKey(record.AvatarStorageKey).SetAvatarStoragePath(record.AvatarStoragePath).SetPasswordHash(record.PasswordHash).SetRole(user.Role(record.Role)).SetEmailVerified(record.EmailVerified).SetDisabled(record.Disabled).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
			if record.Email == nil {
				update.ClearEmail()
			} else {
				update.SetEmail(*record.Email)
			}
			if _, err := update.Save(ctx); err != nil {
				return err
			}
			maps.users[record.ID] = existing.ID
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		create := db.User.Create().SetUsername(record.Username).SetNickname(record.Nickname).SetAvatarURL(record.AvatarURL).SetAvatarStorageKey(record.AvatarStorageKey).SetAvatarStoragePath(record.AvatarStoragePath).SetPasswordHash(record.PasswordHash).SetRole(user.Role(record.Role)).SetEmailVerified(record.EmailVerified).SetDisabled(record.Disabled).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).SetNillableEmail(record.Email)
		created, err := create.Save(ctx)
		if err != nil {
			return err
		}
		maps.users[record.ID] = created.ID
		result.Created++
	}
	for _, record := range data.APITokens {
		userID, ok := maps.users[record.UserID]
		if !ok {
			result.Skipped++
			continue
		}
		existing, err := db.APIToken.Query().Where(apitoken.TokenHashEQ(record.TokenHash)).Only(ctx)
		if err == nil {
			if _, err := db.APIToken.UpdateOneID(existing.ID).SetUserID(userID).SetName(record.Name).SetPrefix(record.Prefix).SetTokenHash(record.TokenHash).SetNillableLastUsedAt(record.LastUsedAt).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
				return err
			}
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		if _, err := db.APIToken.Create().SetUserID(userID).SetName(record.Name).SetPrefix(record.Prefix).SetTokenHash(record.TokenHash).SetNillableLastUsedAt(record.LastUsedAt).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.MCPTokens {
		userID, ok := maps.users[record.UserID]
		if !ok {
			result.Skipped++
			continue
		}
		existing, err := db.MCPToken.Query().Where(mcptoken.TokenHashEQ(record.TokenHash)).Only(ctx)
		if err == nil {
			if _, err := db.MCPToken.UpdateOneID(existing.ID).SetUserID(userID).SetPrincipalType(mcptoken.PrincipalType(record.PrincipalType)).SetNote(record.Note).SetPrefix(record.Prefix).SetTokenHash(record.TokenHash).SetNillableExpiresAt(record.ExpiresAt).SetNillableLastUsedAt(record.LastUsedAt).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
				return err
			}
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		if _, err := db.MCPToken.Create().SetUserID(userID).SetPrincipalType(mcptoken.PrincipalType(record.PrincipalType)).SetNote(record.Note).SetPrefix(record.Prefix).SetTokenHash(record.TokenHash).SetNillableExpiresAt(record.ExpiresAt).SetNillableLastUsedAt(record.LastUsedAt).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.Groups {
		ownerID, ok := maps.users[record.OwnerID]
		if !ok {
			result.Skipped++
			continue
		}
		existing, err := db.UserGroup.Query().Where(usergroup.OwnerIDEQ(ownerID), usergroup.SlugEQ(record.Slug)).Only(ctx)
		if err == nil {
			if _, err := db.UserGroup.UpdateOneID(existing.ID).SetOwnerID(ownerID).SetName(record.Name).SetSlug(record.Slug).SetDescription(record.Description).SetCode(record.Code).SetCodeUpdatedAt(record.CodeUpdatedAt).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx); err != nil {
				return err
			}
			maps.groups[record.ID] = existing.ID
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		created, err := db.UserGroup.Create().SetOwnerID(ownerID).SetName(record.Name).SetSlug(record.Slug).SetDescription(record.Description).SetCode(record.Code).SetCodeUpdatedAt(record.CodeUpdatedAt).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx)
		if err != nil {
			return err
		}
		maps.groups[record.ID] = created.ID
		result.Created++
	}
	for _, record := range data.GroupMembers {
		groupID, groupOK := maps.groups[record.GroupID]
		userID, userOK := maps.users[record.UserID]
		if !groupOK || !userOK {
			result.Skipped++
			continue
		}
		exists, err := db.GroupMember.Query().Where(groupmember.GroupIDEQ(groupID), groupmember.UserIDEQ(userID)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := db.GroupMember.Create().SetGroupID(groupID).SetUserID(userID).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	return nil
}

func importAppsData(ctx context.Context, db *ent.Client, data AppsData, maps *importMaps, pathMap map[string]string, actorUserID int, result *ImportResult) error {
	if err := importAssetRecords(ctx, db, data.Assets, maps, result); err != nil {
		return err
	}
	for _, record := range data.Categories {
		existing, err := db.Category.Query().Where(category.SlugEQ(record.Slug)).Only(ctx)
		if err == nil {
			update := db.Category.UpdateOneID(existing.ID).SetName(record.Name).SetNameI18n(record.NameI18n).SetSlug(record.Slug).SetSortOrder(record.SortOrder).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
			if record.ParentID == nil {
				update.ClearParentID()
			} else if parentID, ok := maps.categories[*record.ParentID]; ok {
				update.SetParentID(parentID)
			}
			if _, err := update.Save(ctx); err != nil {
				return err
			}
			maps.categories[record.ID] = existing.ID
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		create := db.Category.Create().SetName(record.Name).SetNameI18n(record.NameI18n).SetSlug(record.Slug).SetSortOrder(record.SortOrder).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
		if record.ParentID != nil {
			if parentID, ok := maps.categories[*record.ParentID]; ok {
				create.SetParentID(parentID)
			}
		}
		created, err := create.Save(ctx)
		if err != nil {
			return err
		}
		maps.categories[record.ID] = created.ID
		result.Created++
	}
	for _, record := range data.Categories {
		if record.ParentID == nil {
			continue
		}
		categoryID, ok := maps.categories[record.ID]
		if !ok {
			continue
		}
		parentID, ok := maps.categories[*record.ParentID]
		if !ok {
			continue
		}
		if _, err := db.Category.UpdateOneID(categoryID).SetParentID(parentID).Save(ctx); err != nil {
			return err
		}
	}
	for _, record := range data.Tags {
		existing, err := db.Tag.Query().Where(tag.SlugEQ(record.Slug)).Only(ctx)
		if err == nil {
			if _, err := db.Tag.UpdateOneID(existing.ID).SetName(record.Name).SetNameI18n(record.NameI18n).SetSlug(record.Slug).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx); err != nil {
				return err
			}
			maps.tags[record.ID] = existing.ID
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		created, err := db.Tag.Create().SetName(record.Name).SetNameI18n(record.NameI18n).SetSlug(record.Slug).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx)
		if err != nil {
			return err
		}
		maps.tags[record.ID] = created.ID
		result.Created++
	}
	for _, record := range data.Apps {
		iconURL := record.IconURL
		if iconURL != nil {
			remapped, err := remapServerAssetURL(*iconURL, maps)
			if err != nil {
				return err
			}
			iconURL = &remapped
		}
		ownerID, ok := maps.users[record.OwnerID]
		if !ok {
			ownerID = actorUserID
		}
		if ownerID <= 0 {
			result.Skipped++
			continue
		}
		var categoryID *int
		if record.CategoryID != nil {
			if mapped, ok := maps.categories[*record.CategoryID]; ok {
				categoryID = &mapped
			}
		}
		existing, err := db.App.Query().Where(apppkg.PackageIDEQ(record.PackageID)).Only(ctx)
		if err == nil {
			update := db.App.UpdateOneID(existing.ID).SetOwnerID(ownerID).SetPackageID(record.PackageID).SetName(record.Name).SetNameI18nJSON(record.NameI18nJSON).SetSlug(record.Slug).SetSummary(record.Summary).SetSummaryI18nJSON(record.SummaryI18nJSON).SetDescription(record.Description).SetDescriptionI18nJSON(record.DescriptionI18nJSON).SetAuthor(record.Author).SetHomepage(record.Homepage).SetLicense(record.License).SetMinOsVersion(record.MinOSVersion).SetStatus(apppkg.Status(record.Status)).SetAllowUnreviewedUpdates(record.AllowUnreviewedUpdates).SetCommentsEnabled(record.CommentsEnabled).SetEmailNotificationsEnabled(record.EmailNotificationsEnabled).SetInstallPasswordHash(record.InstallPasswordHash).SetDownloadCount(record.DownloadCount).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
			if iconURL == nil {
				update.ClearIconURL()
			} else {
				update.SetIconURL(*iconURL)
			}
			if categoryID == nil {
				update.ClearCategoryID()
			} else {
				update.SetCategoryID(*categoryID)
			}
			if record.VersionRetentionCount == nil {
				update.ClearVersionRetentionCount()
			} else {
				update.SetVersionRetentionCount(*record.VersionRetentionCount)
			}
			if _, err := update.Save(ctx); err != nil {
				return err
			}
			maps.apps[record.ID] = existing.ID
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		create := db.App.Create().SetOwnerID(ownerID).SetPackageID(record.PackageID).SetName(record.Name).SetNameI18nJSON(record.NameI18nJSON).SetSlug(record.Slug).SetSummary(record.Summary).SetSummaryI18nJSON(record.SummaryI18nJSON).SetDescription(record.Description).SetDescriptionI18nJSON(record.DescriptionI18nJSON).SetAuthor(record.Author).SetHomepage(record.Homepage).SetLicense(record.License).SetMinOsVersion(record.MinOSVersion).SetNillableIconURL(iconURL).SetStatus(apppkg.Status(record.Status)).SetAllowUnreviewedUpdates(record.AllowUnreviewedUpdates).SetCommentsEnabled(record.CommentsEnabled).SetEmailNotificationsEnabled(record.EmailNotificationsEnabled).SetInstallPasswordHash(record.InstallPasswordHash).SetDownloadCount(record.DownloadCount).SetNillableVersionRetentionCount(record.VersionRetentionCount).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt)
		if categoryID != nil {
			create.SetCategoryID(*categoryID)
		}
		created, err := create.Save(ctx)
		if err != nil {
			return err
		}
		maps.apps[record.ID] = created.ID
		result.Created++
	}
	if err := importAssetLinks(ctx, db, assetOwnerApp, data.AssetLinks, maps, func(ownerID int) (int, bool) {
		mapped, ok := maps.apps[ownerID]
		return mapped, ok
	}, result); err != nil {
		return err
	}
	for _, record := range data.AppVersions {
		appID, appOK := maps.apps[record.AppID]
		uploaderID, userOK := maps.users[record.UploaderID]
		if !userOK {
			uploaderID = actorUserID
		}
		if !appOK || uploaderID <= 0 {
			result.Skipped++
			continue
		}
		storagePath := remapStoragePath(pathMap, record.StorageKey, record.StoragePath)
		existing, err := db.AppVersion.Query().Where(versionpkg.AppIDEQ(appID), versionpkg.VersionEQ(record.Version)).Only(ctx)
		if err == nil {
			if _, err := db.AppVersion.UpdateOneID(existing.ID).SetAppID(appID).SetUploaderID(uploaderID).SetVersion(record.Version).SetChangelog(record.Changelog).SetStatus(versionpkg.Status(record.Status)).SetSourceType(versionpkg.SourceType(record.SourceType)).SetDownloadURL(record.DownloadURL).SetStorageKey(record.StorageKey).SetStoragePath(storagePath).SetFileSize(record.FileSize).SetSha256(record.SHA256).SetNillablePublishedAt(record.PublishedAt).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx); err != nil {
				return err
			}
			maps.versions[record.ID] = existing.ID
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		created, err := db.AppVersion.Create().SetAppID(appID).SetUploaderID(uploaderID).SetVersion(record.Version).SetChangelog(record.Changelog).SetStatus(versionpkg.Status(record.Status)).SetSourceType(versionpkg.SourceType(record.SourceType)).SetDownloadURL(record.DownloadURL).SetStorageKey(record.StorageKey).SetStoragePath(storagePath).SetFileSize(record.FileSize).SetSha256(record.SHA256).SetNillablePublishedAt(record.PublishedAt).SetCreatedAt(record.CreatedAt).SetUpdatedAt(record.UpdatedAt).Save(ctx)
		if err != nil {
			return err
		}
		maps.versions[record.ID] = created.ID
		result.Created++
	}
	legacyVersionNames := make(map[int]string, len(data.AppVersions))
	for _, record := range data.AppVersions {
		legacyVersionNames[record.ID] = record.Version
	}
	for _, record := range data.AppDownloads {
		appID, appOK := maps.apps[record.AppID]
		if !appOK {
			result.Skipped++
			continue
		}
		version := strings.TrimSpace(record.Version)
		if version == "" && record.LegacyVersionID > 0 {
			version = legacyVersionNames[record.LegacyVersionID]
		}
		exists, err := db.AppDownload.Query().
			Where(appdownload.AppIDEQ(appID), appdownload.VersionEQ(version), appdownload.CreatedAtEQ(record.CreatedAt)).
			Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := db.AppDownload.Create().
			SetAppID(appID).
			SetVersion(version).
			SetCreatedAt(record.CreatedAt).
			Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.AppScreenshots {
		appID, appOK := maps.apps[record.AppID]
		uploaderID, userOK := maps.users[record.UploaderID]
		if !userOK {
			uploaderID = actorUserID
		}
		if !appOK || uploaderID <= 0 {
			result.Skipped++
			continue
		}
		storagePath := remapStoragePath(pathMap, record.StorageKey, record.StoragePath)
		exists, err := db.AppScreenshot.Query().Where(appscreenshot.AppIDEQ(appID), appscreenshot.ImageURLEQ(record.ImageURL), appscreenshot.StoragePathEQ(storagePath)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := db.AppScreenshot.Create().SetAppID(appID).SetUploaderID(uploaderID).SetImageURL(record.ImageURL).SetStorageKey(record.StorageKey).SetStoragePath(storagePath).SetCaption(record.Caption).SetDeviceType(appscreenshot.DeviceType(record.DeviceType)).SetSortOrder(record.SortOrder).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.AppTags {
		appID, appOK := maps.apps[record.AppID]
		tagID, tagOK := maps.tags[record.TagID]
		if !appOK || !tagOK {
			result.Skipped++
			continue
		}
		exists, err := db.AppTag.Query().Where(apptag.AppIDEQ(appID), apptag.TagIDEQ(tagID)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := db.AppTag.Create().SetAppID(appID).SetTagID(tagID).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.AppVisibilities {
		appID, appOK := maps.apps[record.AppID]
		groupID, groupOK := maps.groups[record.GroupID]
		if !appOK || !groupOK {
			result.Skipped++
			continue
		}
		exists, err := db.AppVisibility.Query().Where(appvisibility.AppIDEQ(appID), appvisibility.GroupIDEQ(groupID)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			result.Skipped++
			continue
		}
		if _, err := db.AppVisibility.Create().SetAppID(appID).SetGroupID(groupID).SetCreatedAt(record.CreatedAt).Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	for _, record := range data.AppVotes {
		appID, appOK := maps.apps[record.AppID]
		userID, userOK := maps.users[record.UserID]
		if !appOK || !userOK {
			result.Skipped++
			continue
		}
		value := record.Value
		if value == 0 {
			value = 1
		}
		existing, err := db.AppVote.Query().
			Where(appvote.AppIDEQ(appID), appvote.UserIDEQ(userID)).
			Only(ctx)
		if err == nil {
			if _, err := db.AppVote.UpdateOneID(existing.ID).
				SetValue(value).
				SetCreatedAt(record.CreatedAt).
				SetUpdatedAt(record.UpdatedAt).
				Save(ctx); err != nil {
				return err
			}
			result.Updated++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		if _, err := db.AppVote.Create().
			SetAppID(appID).
			SetUserID(userID).
			SetValue(value).
			SetCreatedAt(record.CreatedAt).
			SetUpdatedAt(record.UpdatedAt).
			Save(ctx); err != nil {
			return err
		}
		result.Created++
	}
	return nil
}

func importAssetRecords(ctx context.Context, db *ent.Client, records []AssetRecord, maps *importMaps, result *ImportResult) error {
	for _, record := range records {
		if record.ID <= 0 {
			return fmt.Errorf("asset record is missing id")
		}
		if len(record.Data) == 0 {
			return fmt.Errorf("asset %d is empty", record.ID)
		}
		if record.Size != int64(len(record.Data)) {
			return fmt.Errorf("asset %d size does not match data", record.ID)
		}
		sum := sha256.Sum256(record.Data)
		sha := hex.EncodeToString(sum[:])
		if !strings.EqualFold(record.SHA256, sha) {
			return fmt.Errorf("asset %d checksum mismatch", record.ID)
		}
		existing, err := db.Asset.Query().Where(asset.Sha256EQ(sha)).Only(ctx)
		if err == nil {
			maps.assets[record.ID] = existing.ID
			result.Skipped++
			continue
		}
		if !ent.IsNotFound(err) {
			return err
		}
		created, err := db.Asset.Create().
			SetSha256(sha).
			SetMediaType(record.MediaType).
			SetSize(record.Size).
			SetData(record.Data).
			SetCreatedAt(record.CreatedAt).
			SetUpdatedAt(record.UpdatedAt).
			Save(ctx)
		if err != nil {
			return err
		}
		maps.assets[record.ID] = created.ID
		result.Created++
	}
	return nil
}

func importAssetLinks(ctx context.Context, db *ent.Client, ownerType string, records []AssetLinkRecord, maps *importMaps, mapOwner func(int) (int, bool), result *ImportResult) error {
	deleted := map[string]struct{}{}
	for _, record := range records {
		if record.OwnerType != ownerType {
			return fmt.Errorf("asset link %d has unexpected owner type %q", record.ID, record.OwnerType)
		}
		assetID, ok := maps.assets[record.AssetID]
		if !ok {
			return fmt.Errorf("asset link %d references missing asset %d", record.ID, record.AssetID)
		}
		ownerID, ok := mapOwner(record.OwnerID)
		if !ok {
			result.Skipped++
			continue
		}
		key := fmt.Sprintf("%s:%d:%s", ownerType, ownerID, record.Role)
		if _, ok := deleted[key]; !ok {
			if _, err := db.AssetLink.Delete().
				Where(assetlink.OwnerTypeEQ(ownerType), assetlink.OwnerIDEQ(ownerID), assetlink.RoleEQ(record.Role)).
				Exec(ctx); err != nil {
				return err
			}
			deleted[key] = struct{}{}
		}
		if _, err := db.AssetLink.Create().
			SetAssetID(assetID).
			SetOwnerType(ownerType).
			SetOwnerID(ownerID).
			SetRole(record.Role).
			SetSortOrder(record.SortOrder).
			SetCreatedAt(record.CreatedAt).
			SetUpdatedAt(record.UpdatedAt).
			Save(ctx); err != nil {
			if ent.IsConstraintError(err) {
				result.Skipped++
				continue
			}
			return err
		}
		result.Created++
	}
	return cleanupUnlinkedAssets(ctx, db)
}

func remapServerAssetURL(value string, maps *importMaps) (string, error) {
	assetID, ok := assetdata.AssetIDFromURL(serverAssetURLPrefix, value)
	if !ok {
		return value, nil
	}
	mapped, ok := maps.assets[assetID]
	if !ok {
		return "", fmt.Errorf("asset URL references missing asset %d", assetID)
	}
	return assetdata.URL(serverAssetURLPrefix, mapped), nil
}

func cleanupUnlinkedAssets(ctx context.Context, db *ent.Client) error {
	records, err := db.Asset.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		linked, err := db.AssetLink.Query().Where(assetlink.AssetIDEQ(record.ID)).Exist(ctx)
		if err != nil {
			return err
		}
		if linked {
			continue
		}
		if err := db.Asset.DeleteOneID(record.ID).Exec(ctx); err != nil && !ent.IsNotFound(err) {
			return err
		}
	}
	return nil
}
