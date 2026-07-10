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

const currentServerSchemaVersion = 2

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
	if version < 2 {
		if err := s.migrateDownloadVersionSnapshots(ctx); err != nil {
			return err
		}
		version = 2
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

func (s *Server) migrateDownloadVersionSnapshots(ctx context.Context) error {
	hasLegacyVersionID, err := s.appDownloadsHasLegacyVersionID(ctx)
	if err != nil {
		return err
	}
	if !hasLegacyVersionID {
		return nil
	}
	if _, err := s.sqlDB.ExecContext(ctx, `
		UPDATE app_downloads
		SET version = COALESCE(
			(SELECT version FROM app_versions WHERE app_versions.id = app_downloads.version_id),
			''
		)
		WHERE version = ''
	`); err != nil {
		return fmt.Errorf("backfill app download version snapshots: %w", err)
	}
	return nil
}

func (s *Server) appDownloadsHasLegacyVersionID(ctx context.Context) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s.cfg.DBDriver)) {
	case "sqlite", "sqlite3":
		rows, err := s.sqlDB.QueryContext(ctx, "PRAGMA table_info(app_downloads)")
		if err != nil {
			return false, fmt.Errorf("inspect SQLite app_downloads columns: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var (
				columnID     int
				name         string
				columnType   string
				notNull      int
				defaultValue any
				primaryKey   int
			)
			if err := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
				return false, fmt.Errorf("scan SQLite app_downloads column: %w", err)
			}
			if name == "version_id" {
				return true, nil
			}
		}
		if err := rows.Err(); err != nil {
			return false, fmt.Errorf("iterate SQLite app_downloads columns: %w", err)
		}
		return false, nil
	case "postgres", "postgresql":
		var count int
		if err := s.sqlDB.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'app_downloads'
			  AND column_name = 'version_id'
		`).Scan(&count); err != nil {
			return false, fmt.Errorf("inspect PostgreSQL app_downloads columns: %w", err)
		}
		return count > 0, nil
	case "mysql":
		var count int
		if err := s.sqlDB.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = DATABASE()
			  AND table_name = 'app_downloads'
			  AND column_name = 'version_id'
		`).Scan(&count); err != nil {
			return false, fmt.Errorf("inspect MySQL app_downloads columns: %w", err)
		}
		return count > 0, nil
	default:
		return false, fmt.Errorf("inspect app_downloads columns: unsupported database driver %q", s.cfg.DBDriver)
	}
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
