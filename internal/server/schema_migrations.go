package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/sitesetting"
)

const currentServerSchemaVersion = 1

func (s *Server) migrateSchema(ctx context.Context) error {
	version := s.storedSchemaVersion(ctx)
	if version >= currentServerSchemaVersion {
		return nil
	}
	if version < 1 {
		if err := s.migrateImageDataURLs(ctx); err != nil {
			return err
		}
		version = 1
		if err := s.setSetting(ctx, settingSchemaVersion, strconv.Itoa(version)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) storedSchemaVersion(ctx context.Context) int {
	raw := strings.TrimSpace(s.setting(ctx, settingSchemaVersion, "0"))
	version, err := strconv.Atoi(raw)
	if err != nil || version < 0 {
		return 0
	}
	return version
}

func (s *Server) migrateImageDataURLs(ctx context.Context) error {
	if err := s.migrateAppIconDataURLs(ctx); err != nil {
		return err
	}
	return s.migrateSiteLogoDataURL(ctx)
}

func (s *Server) migrateAppIconDataURLs(ctx context.Context) error {
	records, err := s.db.App.Query().
		Where(app.IconURLNotNil(), app.IconURLHasPrefix("data:")).
		All(ctx)
	if err != nil {
		return fmt.Errorf("load app icon data URLs: %w", err)
	}
	for _, record := range records {
		if record.IconURL == nil || strings.TrimSpace(*record.IconURL) == "" {
			continue
		}
		nextURL, assetID, err := s.saveAppIconDataURLAsset(ctx, *record.IconURL)
		if err != nil {
			return fmt.Errorf("save app %d icon asset: %w", record.ID, err)
		}
		if _, err := s.db.App.UpdateOneID(record.ID).SetIconURL(nextURL).Save(ctx); err != nil {
			_ = s.cleanupAssetIDs(ctx, assetID)
			return fmt.Errorf("update app %d icon URL: %w", record.ID, err)
		}
		if err := s.replaceAssetLinks(ctx, assetOwnerApp, record.ID, assetRoleIcon, assetID); err != nil {
			return fmt.Errorf("link app %d icon asset: %w", record.ID, err)
		}
	}
	return nil
}

func (s *Server) migrateSiteLogoDataURL(ctx context.Context) error {
	record, err := s.db.SiteSetting.Query().Where(sitesetting.KeyEQ(settingSiteIconURL)).Only(ctx)
	if entgo.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load site logo setting: %w", err)
	}
	if !isDataURL(record.Value) {
		if assetID, ok := s.assetIDFromURL(record.Value); ok {
			return s.replaceAssetLinks(ctx, assetOwnerSite, 0, assetRoleIcon, assetID)
		}
		return nil
	}
	nextURL, assetID, err := s.saveDataURLAsset(ctx, record.Value)
	if err != nil {
		return fmt.Errorf("save site logo asset: %w", err)
	}
	if _, err := s.db.SiteSetting.UpdateOneID(record.ID).SetValue(nextURL).Save(ctx); err != nil {
		_ = s.cleanupAssetIDs(ctx, assetID)
		return fmt.Errorf("update site logo setting: %w", err)
	}
	return s.replaceAssetLinks(ctx, assetOwnerSite, 0, assetRoleIcon, assetID)
}
