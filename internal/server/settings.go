package server

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent/sitesetting"
	"lazycat.community/appstore/internal/mirror"
)

const (
	settingMaxLPKSize               = "max_lpk_size"
	settingMaxVersions              = "max_versions"
	settingRequireEmailVerify       = "require_email_verify"
	settingSourcePassword           = "source_password"
	settingSourcePasswordRotation   = "source_password_rotation"
	settingCommentsEnabled          = "comments_enabled"
	settingAllowManualOutdatedClear = "allow_manual_outdated_clear"
	settingGitHubDownloadMirrors    = "github_download_mirrors"
	settingGitHubRawMirrors         = "github_raw_mirrors"
	settingSiteTitle                = "site_title"
	settingSiteSubtitle             = "site_subtitle"
	settingSiteIconURL              = "site_icon_url"
	settingSitePublicURL            = "site_public_url"
	settingDefaultStorageKey        = "default_storage_key"
	settingAnnouncementEnabled      = "announcement_enabled"
	settingAnnouncementLevel        = "announcement_level"
	settingAnnouncementTitle        = "announcement_title"
	settingAnnouncementBody         = "announcement_body"
	settingAnnouncementLinkLabel    = "announcement_link_label"
	settingAnnouncementLinkURL      = "announcement_link_url"
	settingAnnouncementUpdatedAt    = "announcement_updated_at"
	settingRegistrationMode         = "registration_mode"
	settingSMTPHost                 = "smtp_host"
	settingSMTPPort                 = "smtp_port"
	settingSMTPUser                 = "smtp_user"
	settingSMTPPass                 = "smtp_pass"
	settingSMTPFrom                 = "smtp_from"
)

const (
	registrationModeOpen   = "open"
	registrationModeInvite = "invite"
	registrationModeClosed = "closed"
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
	password := s.setting(ctx, settingSourcePassword, s.cfg.SourcePassword)
	rotationDays := s.settingInt(ctx, settingSourcePasswordRotation, s.cfg.SourcePasswordRotation)
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
	_ = s.setSetting(ctx, settingSourcePassword, token)
	_ = s.setSetting(ctx, "source_password_rotated_at", time.Now().UTC().Format(time.RFC3339))
	return token
}

func (s *Server) effectiveGitHubMirrors(ctx context.Context) []mirror.Entry {
	download, _ := mirror.Parse(s.setting(ctx, settingGitHubDownloadMirrors, s.cfg.GitHubDownloadMirrors), mirror.KindDownload)
	raw, _ := mirror.Parse(s.setting(ctx, settingGitHubRawMirrors, s.cfg.GitHubRawMirrors), mirror.KindRaw)
	return append(download, raw...)
}

func (s *Server) effectiveMaxVersions(ctx context.Context) int {
	return s.settingInt(ctx, settingMaxVersions, s.cfg.MaxVersions)
}

func (s *Server) effectiveMaxLPKSize(ctx context.Context) int64 {
	raw := s.setting(ctx, settingMaxLPKSize, "")
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
	raw := s.setting(ctx, settingRequireEmailVerify, "")
	if raw == "" {
		return s.cfg.RequireEmailVerify
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return s.cfg.RequireEmailVerify
	}
	return value
}

func (s *Server) commentsEnabled(ctx context.Context) bool {
	return s.settingBool(ctx, settingCommentsEnabled, true)
}

func (s *Server) commentsAllowed(ctx context.Context, appCommentsEnabled bool) bool {
	return appCommentsEnabled && s.commentsEnabled(ctx)
}

func (s *Server) manualOutdatedClearAllowed(ctx context.Context) bool {
	return s.settingBool(ctx, settingAllowManualOutdatedClear, false)
}

func (s *Server) registrationMode(ctx context.Context) string {
	mode := strings.ToLower(strings.TrimSpace(s.setting(ctx, settingRegistrationMode, registrationModeOpen)))
	if validRegistrationMode(mode) {
		return mode
	}
	return registrationModeOpen
}

func (s *Server) siteProfile(ctx context.Context) siteProfile {
	publicURL := s.sitePublicURL(ctx)
	title := strings.TrimSpace(s.setting(ctx, settingSiteTitle, ""))
	if title == "" {
		title = "懒猫私有商店服务端"
	}
	level := strings.TrimSpace(s.setting(ctx, settingAnnouncementLevel, "info"))
	if !validAnnouncementLevel(level) {
		level = "info"
	}
	announcement := siteAnnouncement{
		Enabled:   s.settingBool(ctx, settingAnnouncementEnabled, false),
		Level:     level,
		Title:     strings.TrimSpace(s.setting(ctx, settingAnnouncementTitle, "")),
		Body:      strings.TrimSpace(s.setting(ctx, settingAnnouncementBody, "")),
		LinkLabel: strings.TrimSpace(s.setting(ctx, settingAnnouncementLinkLabel, "")),
		LinkURL:   cleanURLSetting(s.setting(ctx, settingAnnouncementLinkURL, "")),
		UpdatedAt: strings.TrimSpace(s.setting(ctx, settingAnnouncementUpdatedAt, "")),
	}
	return siteProfile{
		Title:        title,
		Subtitle:     strings.TrimSpace(s.setting(ctx, settingSiteSubtitle, "")),
		IconURL:      cleanURLSetting(s.setting(ctx, settingSiteIconURL, "")),
		PublicURL:    publicURL,
		SourceURL:    publicURL + "/source/v1/index.json",
		Version:      appVersion(),
		Announcement: announcement,
		Registration: siteRegistration{Mode: s.registrationMode(ctx)},
	}
}

func (s *Server) sitePublicURL(ctx context.Context) string {
	value := cleanURLSetting(s.setting(ctx, settingSitePublicURL, ""))
	if value != "" {
		return value
	}
	if s.cfg.SitePublicURL != "" {
		return cleanURLSetting(s.cfg.SitePublicURL)
	}
	return cleanURLSetting(s.cfg.BaseURL)
}

func (s *Server) settingBool(ctx context.Context, key string, fallback bool) bool {
	raw := strings.TrimSpace(s.setting(ctx, key, ""))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func cleanURLSetting(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func isHTTPURLOrEmpty(value string) bool {
	value = cleanURLSetting(value)
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func validAnnouncementLevel(value string) bool {
	switch value {
	case "info", "warning", "success":
		return true
	default:
		return false
	}
}

func validRegistrationMode(value string) bool {
	switch value {
	case registrationModeOpen, registrationModeInvite, registrationModeClosed:
		return true
	default:
		return false
	}
}
