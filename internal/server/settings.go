package server

import (
	"context"
	"strconv"
	"time"

	"lazycat.community/appstore/ent/sitesetting"
)

func (s *Server) setting(ctx context.Context, key, fallback string) string {
	record, err := s.db.SiteSetting.Query().Where(sitesetting.KeyEQ(key)).Only(ctx)
	if err != nil {
		return fallback
	}
	return record.Value
}

func (s *Server) settingInt(ctx context.Context, key string, fallback int) int {
	raw := s.setting(ctx, key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func (s *Server) setSetting(ctx context.Context, key, value string) error {
	record, err := s.db.SiteSetting.Query().Where(sitesetting.KeyEQ(key)).Only(ctx)
	if err == nil {
		_, err = s.db.SiteSetting.UpdateOneID(record.ID).SetValue(value).Save(ctx)
		return err
	}
	_, err = s.db.SiteSetting.Create().SetKey(key).SetValue(value).Save(ctx)
	return err
}

func (s *Server) sourcePassword(ctx context.Context) string {
	password := s.setting(ctx, "source_password", s.cfg.SourcePassword)
	rotationDays := s.settingInt(ctx, "source_password_rotation", s.cfg.SourcePasswordRotation)
	if rotationDays <= 0 || password == "" {
		return password
	}

	rotatedAtRaw := s.setting(ctx, "source_password_rotated_at", "")
	rotatedAt, err := time.Parse(time.RFC3339, rotatedAtRaw)
	if err != nil {
		_ = s.setSetting(ctx, "source_password_rotated_at", time.Now().UTC().Format(time.RFC3339))
		return password
	}
	if time.Since(rotatedAt) < time.Duration(rotationDays)*24*time.Hour {
		return password
	}
	token, err := randomToken()
	if err != nil {
		return password
	}
	_ = s.setSetting(ctx, "source_password", token)
	_ = s.setSetting(ctx, "source_password_rotated_at", time.Now().UTC().Format(time.RFC3339))
	return token
}

func (s *Server) effectiveGitHubMirror(ctx context.Context) string {
	return s.setting(ctx, "github_mirror", s.cfg.GitHubMirror)
}

func (s *Server) effectiveMaxVersions(ctx context.Context) int {
	return s.settingInt(ctx, "max_versions", s.cfg.MaxVersions)
}

func (s *Server) effectiveMaxLPKSize(ctx context.Context) int64 {
	raw := s.setting(ctx, "max_lpk_size", "")
	if raw == "" {
		return s.cfg.MaxLPKSize
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return s.cfg.MaxLPKSize
	}
	return value
}

func (s *Server) effectiveRequireEmailVerify(ctx context.Context) bool {
	raw := s.setting(ctx, "require_email_verify", "")
	if raw == "" {
		return s.cfg.RequireEmailVerify
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return s.cfg.RequireEmailVerify
	}
	return value
}
