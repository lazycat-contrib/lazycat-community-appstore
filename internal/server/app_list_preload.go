package server

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/collaborator"
	favoritepkg "lazycat.community/appstore/ent/favorite"
	"lazycat.community/appstore/ent/groupmember"
	"lazycat.community/appstore/ent/predicate"
	"lazycat.community/appstore/ent/tag"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/catalogmeta"
)

type appSummaryPreload struct {
	owners             map[int]string
	categories         map[int]*entgo.Category
	tags               map[int][]string
	visibleGroupIDs    map[int][]int
	latestVersions     map[int]*entgo.AppVersion
	collaboratorAppIDs map[int]struct{}
	appFavorites       map[int]bool
	submitterFavorites map[int]bool
	downloadStats      map[int]downloadStats
	ratings            map[int]ratingSummary
	commentsEnabled    bool
}

func (s *Server) applyAppListVisibility(ctx context.Context, q *entgo.AppQuery, u *entgo.User, collaboratorAppIDs []int) error {
	if isAdmin(u) {
		return nil
	}
	if u == nil {
		q.Where(app.Not(appHasAnyVisibility()))
		return nil
	}

	groupIDs, err := s.userGroupIDs(ctx, u.ID)
	if err != nil {
		return err
	}

	visibilityPredicates := []predicate.App{
		app.Not(appHasAnyVisibility()),
		app.OwnerIDEQ(u.ID),
	}
	if len(collaboratorAppIDs) > 0 {
		visibilityPredicates = append(visibilityPredicates, app.IDIn(collaboratorAppIDs...))
	}
	if len(groupIDs) > 0 {
		visibilityPredicates = append(visibilityPredicates, appVisibleToGroups(groupIDs))
	}
	q.Where(app.Or(visibilityPredicates...))
	return nil
}

func appHasAnyVisibility() predicate.App {
	return func(selector *entsql.Selector) {
		visibility := entsql.Table(appvisibility.Table)
		selector.Where(entsql.Exists(
			entsql.Select().
				From(visibility).
				Where(entsql.ColumnsEQ(visibility.C(appvisibility.FieldAppID), selector.C(app.FieldID))),
		))
	}
}

func appVisibleToGroups(groupIDs []int) predicate.App {
	return func(selector *entsql.Selector) {
		visibility := entsql.Table(appvisibility.Table)
		selector.Where(entsql.Exists(
			entsql.Select().
				From(visibility).
				Where(
					entsql.And(
						entsql.ColumnsEQ(visibility.C(appvisibility.FieldAppID), selector.C(app.FieldID)),
						entsql.InInts(visibility.C(appvisibility.FieldGroupID), groupIDs...),
					),
				),
		))
	}
}

func (s *Server) userGroupIDs(ctx context.Context, userID int) ([]int, error) {
	records, err := s.db.GroupMember.Query().Where(groupmember.UserIDEQ(userID)).All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.GroupID)
	}
	return ids, nil
}

func (s *Server) preloadAppSummaries(ctx context.Context, apps []*entgo.App, u *entgo.User) (appSummaryPreload, error) {
	data := appSummaryPreload{
		owners:             map[int]string{},
		categories:         map[int]*entgo.Category{},
		tags:               map[int][]string{},
		visibleGroupIDs:    map[int][]int{},
		latestVersions:     map[int]*entgo.AppVersion{},
		collaboratorAppIDs: map[int]struct{}{},
		appFavorites:       map[int]bool{},
		submitterFavorites: map[int]bool{},
		downloadStats:      map[int]downloadStats{},
		ratings:            map[int]ratingSummary{},
	}
	if len(apps) == 0 {
		return data, nil
	}
	data.commentsEnabled = s.commentsEnabled(ctx)
	appIDs := make([]int, 0, len(apps))
	ownerIDs := map[int]struct{}{}
	categoryIDs := map[int]struct{}{}
	for _, record := range apps {
		appIDs = append(appIDs, record.ID)
		ownerIDs[record.OwnerID] = struct{}{}
		if record.CategoryID != nil {
			categoryIDs[*record.CategoryID] = struct{}{}
		}
	}
	if err := s.loadAppSummaryOwners(ctx, ownerIDs, data.owners); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryCategories(ctx, categoryIDs, data.categories); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryTags(ctx, appIDs, data.tags); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryVisibleGroups(ctx, appIDs, data.visibleGroupIDs); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryLatestVersions(ctx, appIDs, data.latestVersions); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryDownloadStats(ctx, appIDs, data.downloadStats); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryRatings(ctx, appIDs, u, data.ratings); err != nil {
		return data, err
	}
	if u != nil {
		if err := s.loadAppSummaryCollaborations(ctx, appIDs, u.ID, data.collaboratorAppIDs); err != nil {
			return data, err
		}
		if err := s.loadAppSummaryFavorites(ctx, appIDs, mapKeys(ownerIDs), u.ID, data.appFavorites, data.submitterFavorites); err != nil {
			return data, err
		}
	}
	return data, nil
}

func (s *Server) loadAppSummaryOwners(ctx context.Context, ownerIDs map[int]struct{}, out map[int]string) error {
	ids := mapKeys(ownerIDs)
	if len(ids) == 0 {
		return nil
	}
	records, err := s.db.User.Query().Where(user.IDIn(ids...)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.ID] = userDisplayName(record)
	}
	return nil
}

func (s *Server) loadAppSummaryCategories(ctx context.Context, categoryIDs map[int]struct{}, out map[int]*entgo.Category) error {
	ids := mapKeys(categoryIDs)
	if len(ids) == 0 {
		return nil
	}
	records, err := s.db.Category.Query().Where(category.IDIn(ids...)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.ID] = record
	}
	return nil
}

func (s *Server) loadAppSummaryTags(ctx context.Context, appIDs []int, out map[int][]string) error {
	links, err := s.db.AppTag.Query().Where(apptag.AppIDIn(appIDs...)).All(ctx)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	tagIDs := map[int]struct{}{}
	for _, link := range links {
		tagIDs[link.TagID] = struct{}{}
	}
	records, err := s.db.Tag.Query().Where(tag.IDIn(mapKeys(tagIDs)...)).All(ctx)
	if err != nil {
		return err
	}
	tagNames := make(map[int]string, len(records))
	for _, record := range records {
		tagNames[record.ID] = record.Name
	}
	for _, link := range links {
		if name := tagNames[link.TagID]; name != "" {
			out[link.AppID] = append(out[link.AppID], name)
		}
	}
	return nil
}

func (s *Server) loadAppSummaryVisibleGroups(ctx context.Context, appIDs []int, out map[int][]int) error {
	records, err := s.db.AppVisibility.Query().Where(appvisibility.AppIDIn(appIDs...)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.AppID] = append(out[record.AppID], record.GroupID)
	}
	return nil
}

func (s *Server) loadAppSummaryLatestVersions(ctx context.Context, appIDs []int, out map[int]*entgo.AppVersion) error {
	records, err := s.db.AppVersion.Query().
		Where(appversion.AppIDIn(appIDs...), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Asc(appversion.FieldAppID), entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, exists := out[record.AppID]; !exists {
			out[record.AppID] = record
		}
	}
	return nil
}

func (s *Server) loadAppSummaryCollaborations(ctx context.Context, appIDs []int, userID int, out map[int]struct{}) error {
	records, err := s.db.Collaborator.Query().Where(collaborator.AppIDIn(appIDs...), collaborator.UserIDEQ(userID)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.AppID] = struct{}{}
	}
	return nil
}

func (s *Server) loadAppSummaryFavorites(ctx context.Context, appIDs, ownerIDs []int, userID int, appOut, submitterOut map[int]bool) error {
	if len(appIDs) > 0 {
		records, err := s.db.Favorite.Query().
			Where(favoritepkg.UserIDEQ(userID), favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeAPP), favoritepkg.TargetIDIn(appIDs...)).
			All(ctx)
		if err != nil {
			return err
		}
		for _, record := range records {
			appOut[record.TargetID] = true
		}
	}
	if len(ownerIDs) > 0 {
		records, err := s.db.Favorite.Query().
			Where(favoritepkg.UserIDEQ(userID), favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeSUBMITTER), favoritepkg.TargetIDIn(ownerIDs...)).
			All(ctx)
		if err != nil {
			return err
		}
		for _, record := range records {
			submitterOut[record.TargetID] = true
		}
	}
	return nil
}

func (s *Server) appSummaryDTOFromPreload(ctx context.Context, record *entgo.App, u *entgo.User, preload appSummaryPreload) appSummary {
	dto := appSummary{
		ID:                        record.ID,
		OwnerID:                   record.OwnerID,
		CategoryID:                record.CategoryID,
		PackageID:                 record.PackageID,
		Name:                      record.Name,
		NameI18n:                  catalogmeta.DecodeLocalizedText(record.NameI18nJSON),
		Slug:                      record.Slug,
		Summary:                   record.Summary,
		SummaryI18n:               catalogmeta.DecodeLocalizedText(record.SummaryI18nJSON),
		Description:               record.Description,
		DescriptionI18n:           catalogmeta.DecodeLocalizedText(record.DescriptionI18nJSON),
		IconURL:                   record.IconURL,
		Status:                    string(record.Status),
		AllowUnreviewedUpdates:    record.AllowUnreviewedUpdates,
		CommentsEnabled:           record.CommentsEnabled,
		CommentsAllowed:           record.CommentsEnabled && preload.commentsEnabled,
		EmailNotificationsEnabled: record.EmailNotificationsEnabled,
		InstallProtected:          record.InstallPasswordHash != "",
		DownloadCount:             record.DownloadCount,
		DownloadStats:             preload.downloadStats[record.ID],
		Rating:                    preload.ratings[record.ID],
		CreatedAt:                 record.CreatedAt,
		UpdatedAt:                 record.UpdatedAt,
		Tags:                      preload.tags[record.ID],
		VisibleGroupIDs:           preload.visibleGroupIDs[record.ID],
		Owner:                     preload.owners[record.OwnerID],
	}
	if dto.Tags == nil {
		dto.Tags = []string{}
	}
	dto.DownloadStats.Total = record.DownloadCount
	if u != nil {
		_, isCollaborator := preload.collaboratorAppIDs[record.ID]
		dto.CanManageApp = isAdmin(u) || record.OwnerID == u.ID
		dto.CanUploadVersion = dto.CanManageApp || isCollaborator
		dto.AppFavorited = preload.appFavorites[record.ID]
		dto.SubmitterFavorited = preload.submitterFavorites[record.OwnerID]
	}
	if record.CategoryID != nil {
		if cat := preload.categories[*record.CategoryID]; cat != nil {
			dto.Category = cat.Name
			dto.CategoryI18n = catalogmeta.DecodeLocalizedText(cat.NameI18n)
		}
	}
	if latest := preload.latestVersions[record.ID]; latest != nil {
		v := toVersionDTO(latest)
		dto.LatestVersion = &v
	}
	return dto
}
