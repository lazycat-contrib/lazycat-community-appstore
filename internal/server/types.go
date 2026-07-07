package server

import (
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/internal/catalogmeta"
)

type publicUser struct {
	ID            int       `json:"id"`
	Username      string    `json:"username"`
	Nickname      string    `json:"nickname"`
	AvatarURL     string    `json:"avatarUrl,omitempty"`
	Email         *string   `json:"email,omitempty"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"emailVerified"`
	Disabled      bool      `json:"disabled"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toPublicUser(u *ent.User) publicUser {
	return publicUser{
		ID:            u.ID,
		Username:      u.Username,
		Nickname:      u.Nickname,
		AvatarURL:     u.AvatarURL,
		Email:         u.Email,
		Role:          string(u.Role),
		EmailVerified: u.EmailVerified,
		Disabled:      u.Disabled,
		CreatedAt:     u.CreatedAt,
	}
}

func userDisplayName(u *ent.User) string {
	if u == nil {
		return ""
	}
	if strings.TrimSpace(u.Nickname) != "" {
		return strings.TrimSpace(u.Nickname)
	}
	return u.Username
}

type siteProfile struct {
	Title        string           `json:"title"`
	IconURL      string           `json:"iconUrl,omitempty"`
	PublicURL    string           `json:"publicUrl"`
	SourceURL    string           `json:"sourceUrl"`
	Announcement siteAnnouncement `json:"announcement"`
	Registration siteRegistration `json:"registration"`
}

type siteRegistration struct {
	Mode string `json:"mode"`
}

type siteAnnouncement struct {
	Enabled   bool   `json:"enabled"`
	Level     string `json:"level"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
	LinkURL   string `json:"linkUrl,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type appSummary struct {
	ID                        int               `json:"id"`
	OwnerID                   int               `json:"ownerId"`
	Owner                     string            `json:"owner"`
	CategoryID                *int              `json:"categoryId,omitempty"`
	Category                  string            `json:"category,omitempty"`
	CategoryI18n              map[string]string `json:"categoryI18n,omitempty"`
	PackageID                 string            `json:"packageId"`
	Name                      string            `json:"name"`
	Slug                      string            `json:"slug"`
	Summary                   string            `json:"summary"`
	Description               string            `json:"description"`
	IconURL                   *string           `json:"iconUrl,omitempty"`
	Status                    string            `json:"status"`
	AllowUnreviewedUpdates    bool              `json:"allowUnreviewedUpdates"`
	CommentsEnabled           bool              `json:"commentsEnabled"`
	CommentsAllowed           bool              `json:"commentsAllowed"`
	EmailNotificationsEnabled bool              `json:"emailNotificationsEnabled"`
	InstallProtected          bool              `json:"installProtected"`
	DownloadCount             int               `json:"downloadCount"`
	Tags                      []string          `json:"tags"`
	VisibleGroupIDs           []int             `json:"visibleGroupIds"`
	LatestVersion             *version          `json:"latestVersion,omitempty"`
	CreatedAt                 time.Time         `json:"createdAt"`
	UpdatedAt                 time.Time         `json:"updatedAt"`
}

type categoryDTO struct {
	ID        int                       `json:"id"`
	Name      string                    `json:"name"`
	NameI18n  catalogmeta.LocalizedText `json:"nameI18n,omitempty"`
	Slug      string                    `json:"slug"`
	ParentID  *int                      `json:"parentId,omitempty"`
	SortOrder int                       `json:"sortOrder"`
	CreatedAt time.Time                 `json:"createdAt"`
	UpdatedAt time.Time                 `json:"updatedAt"`
}

type tagDTO struct {
	ID        int                       `json:"id"`
	Name      string                    `json:"name"`
	NameI18n  catalogmeta.LocalizedText `json:"nameI18n,omitempty"`
	Slug      string                    `json:"slug"`
	CreatedAt time.Time                 `json:"createdAt"`
	UpdatedAt time.Time                 `json:"updatedAt"`
}

type appDetail struct {
	appSummary
	Versions              []version    `json:"versions"`
	Screenshots           []screenshot `json:"screenshots"`
	Comments              []comment    `json:"comments"`
	Favorites             int          `json:"favorites"`
	OutdatedMarks         int          `json:"outdatedMarks"`
	OutdatedMarked        bool         `json:"outdatedMarked"`
	CanManageApp          bool         `json:"canManageApp"`
	CanUploadVersion      bool         `json:"canUploadVersion"`
	CanClearOutdatedMarks bool         `json:"canClearOutdatedMarks"`
}

type screenshot struct {
	ID         int       `json:"id"`
	AppID      int       `json:"appId"`
	ImageURL   string    `json:"imageUrl"`
	StorageKey string    `json:"storageKey,omitempty"`
	Caption    string    `json:"caption"`
	DeviceType string    `json:"deviceType"`
	SortOrder  int       `json:"sortOrder"`
	CreatedAt  time.Time `json:"createdAt"`
}

type collectionDTO struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Slug        string       `json:"slug"`
	Description string       `json:"description"`
	Kind        string       `json:"kind"`
	Apps        []appSummary `json:"apps"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type version struct {
	ID          int        `json:"id"`
	AppID       int        `json:"appId"`
	UploaderID  int        `json:"uploaderId"`
	Version     string     `json:"version"`
	Changelog   string     `json:"changelog"`
	Status      string     `json:"status"`
	SourceType  string     `json:"sourceType"`
	DownloadURL string     `json:"downloadUrl"`
	StorageKey  string     `json:"storageKey,omitempty"`
	StoragePath string     `json:"storagePath,omitempty"`
	FileSize    int64      `json:"fileSize"`
	SHA256      string     `json:"sha256"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type comment struct {
	ID           int       `json:"id"`
	AppID        int       `json:"appId"`
	UserID       int       `json:"userId"`
	ParentID     *int      `json:"parentId,omitempty"`
	AuthorType   string    `json:"authorType"`
	ClientUserID string    `json:"clientUserId,omitempty"`
	Username     string    `json:"username"`
	Body         string    `json:"body"`
	CanDelete    bool      `json:"canDelete"`
	Replies      []comment `json:"replies,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type commentNotificationDTO struct {
	ID        int       `json:"id"`
	AppID     int       `json:"appId"`
	CommentID int       `json:"commentId"`
	AppName   string    `json:"appName"`
	ActorName string    `json:"actorName"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

type reviewDTO struct {
	ID         int        `json:"id"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	AppID      *int       `json:"appId,omitempty"`
	VersionID  *int       `json:"versionId,omitempty"`
	Requester  int        `json:"requesterId"`
	ReviewerID *int       `json:"reviewerId,omitempty"`
	Note       string     `json:"note"`
	ReviewNote string     `json:"reviewNote"`
	ReviewedAt *time.Time `json:"reviewedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type collaboratorRequestDTO struct {
	ID        int       `json:"id"`
	AppID     int       `json:"appId"`
	UserID    int       `json:"userId"`
	UserIDRaw int       `json:"user_id"`
	Username  string    `json:"username"`
	Email     *string   `json:"email,omitempty"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
