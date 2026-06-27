package server

import (
	"context"
	"fmt"

	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
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
		name string
		slug string
	}{
		{name: "Productivity", slug: "productivity"},
		{name: "Developer Tools", slug: "developer-tools"},
		{name: "Media", slug: "media"},
		{name: "Utilities", slug: "utilities"},
	}
	for _, item := range defaultCategories {
		exists, err := s.db.Category.Query().Where(category.SlugEQ(item.slug)).Exist(ctx)
		if err != nil {
			return err
		}
		if !exists {
			_, err = s.db.Category.Create().SetName(item.name).SetSlug(item.slug).Save(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
