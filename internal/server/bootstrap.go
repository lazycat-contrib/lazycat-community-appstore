package server

import (
	"context"
	"fmt"

	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
	"lazycat.community/appstore/internal/catalogmeta"
)

var (
	userRoleSoftwareAdmin = user.RoleSOFTWARE_ADMIN
	userRoleSiteAdmin     = user.RoleSITE_ADMIN
)

func (s *Server) bootstrap(ctx context.Context) error {
	count, err := s.db.User.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count == 0 && s.cfg.AdminBootstrap {
		hash, err := auth.HashPassword(s.cfg.AdminPassword)
		if err != nil {
			return err
		}
		if _, err := s.db.User.Create().
			SetUsername(s.cfg.AdminUsername).
			SetPasswordHash(hash).
			SetRole(user.RoleSITE_ADMIN).
			SetEmailVerified(true).
			Save(ctx); err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
	}

	defaultCategories := []struct {
		name     string
		slug     string
		nameI18n catalogmeta.LocalizedText
	}{
		{name: "Productivity", slug: "productivity", nameI18n: catalogmeta.LocalizedText{"en": "Productivity", "zh-CN": "效率"}},
		{name: "Developer Tools", slug: "developer-tools", nameI18n: catalogmeta.LocalizedText{"en": "Developer Tools", "zh-CN": "开发工具"}},
		{name: "Media", slug: "media", nameI18n: catalogmeta.LocalizedText{"en": "Media", "zh-CN": "媒体"}},
		{name: "Utilities", slug: "utilities", nameI18n: catalogmeta.LocalizedText{"en": "Utilities", "zh-CN": "工具"}},
	}
	for _, item := range defaultCategories {
		exists, err := s.db.Category.Query().Where(category.SlugEQ(item.slug)).Exist(ctx)
		if err != nil {
			return err
		}
		if !exists {
			_, err = s.db.Category.Create().
				SetName(item.name).
				SetNameI18n(catalogmeta.EncodeLocalizedText(item.nameI18n)).
				SetSlug(item.slug).
				Save(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
