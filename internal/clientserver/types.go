package clientserver

import (
	"context"
	"time"

	"lazycat.community/appstore/internal/catalogmeta"
	"lazycat.community/appstore/internal/mirror"
)

type SourceDTO struct {
	ID                      int                     `json:"id"`
	Name                    string                  `json:"name"`
	URL                     string                  `json:"url"`
	Password                string                  `json:"password"`
	DefaultDownloadMirrorID string                  `json:"defaultDownloadMirrorId"`
	DefaultRawMirrorID      string                  `json:"defaultRawMirrorId"`
	GroupCodes              []string                `json:"groupCodes,omitempty"`
	Groups                  []SourceGroupDTO        `json:"groups,omitempty"`
	Categories              []SourceCategoryDTO     `json:"categories,omitempty"`
	Announcements           []SourceAnnouncementDTO `json:"announcements,omitempty"`
	Ads                     []SourceAdDTO           `json:"ads,omitempty"`
	ClientPolicy            SourceClientPolicyDTO   `json:"clientPolicy,omitempty"`
	LastInvalidGroupCodes   []string                `json:"lastInvalidGroupCodes,omitempty"`
	GitHubMirrors           []mirror.Entry          `json:"githubMirrors"`
	ChatAvailable           bool                    `json:"chatAvailable"`
	ChatEnabled             bool                    `json:"chatEnabled"`
	AdsPreference           string                  `json:"adsPreference"`
	LastSync                *time.Time              `json:"lastSync,omitempty"`
	LastError               string                  `json:"lastError,omitempty"`
	LastErrorCode           string                  `json:"lastErrorCode,omitempty"`
	LastAppCount            int                     `json:"lastAppCount"`
	LastInstallableCount    int                     `json:"lastInstallableCount"`
}

type SourceCategoryDTO struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	NameI18n  map[string]string `json:"nameI18n,omitempty"`
	Slug      string            `json:"slug"`
	ParentID  *int              `json:"parentId,omitempty"`
	SortOrder int               `json:"sortOrder,omitempty"`
}

type SourceAnnouncementDTO struct {
	ID        int    `json:"id,omitempty"`
	Enabled   bool   `json:"enabled"`
	Level     string `json:"level"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
	LinkURL   string `json:"linkUrl,omitempty"`
	StartsAt  string `json:"startsAt,omitempty"`
	EndsAt    string `json:"endsAt,omitempty"`
	SortOrder int    `json:"sortOrder,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type SourceAdDTO struct {
	ID        int    `json:"id,omitempty"`
	Enabled   bool   `json:"enabled"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	ImageURL  string `json:"imageUrl,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
	LinkURL   string `json:"linkUrl,omitempty"`
	StartsAt  string `json:"startsAt,omitempty"`
	EndsAt    string `json:"endsAt,omitempty"`
	SortOrder int    `json:"sortOrder,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type SourceClientPolicyDTO struct {
	MinVersion string `json:"minVersion,omitempty"`
	Message    string `json:"message,omitempty"`
}

type SourceInput struct {
	Name                    string   `json:"name"`
	URL                     string   `json:"url"`
	Password                string   `json:"password"`
	DefaultDownloadMirrorID string   `json:"defaultDownloadMirrorId"`
	DefaultRawMirrorID      string   `json:"defaultRawMirrorId"`
	GroupCodes              []string `json:"groupCodes"`
	ChatEnabled             *bool    `json:"chatEnabled"`
	AdsPreference           *string  `json:"adsPreference"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type ClientIdentityDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Source      string `json:"source"`
}

type ClientAuthStatusDTO struct {
	Authenticated bool               `json:"authenticated"`
	OIDCEnabled   bool               `json:"oidcEnabled"`
	User          *ClientIdentityDTO `json:"user,omitempty"`
}

type VersionDTO struct {
	Version             string `json:"version"`
	Changelog           string `json:"changelog,omitempty"`
	DownloadURL         string `json:"downloadUrl"`
	UpstreamDownloadURL string `json:"upstreamDownloadUrl,omitempty"`
	SourceType          string `json:"sourceType,omitempty"`
	SHA256              string `json:"sha256"`
	Size                int64  `json:"size"`
}

type SourceAppDTO struct {
	ID               int                      `json:"id"`
	SourceID         int                      `json:"sourceId"`
	SourceName       string                   `json:"sourceName"`
	ExternalID       string                   `json:"externalId"`
	PackageID        string                   `json:"packageId"`
	Name             string                   `json:"name"`
	NameI18n         map[string]string        `json:"nameI18n,omitempty"`
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	SummaryI18n      map[string]string        `json:"summaryI18n,omitempty"`
	DescriptionI18n  map[string]string        `json:"descriptionI18n,omitempty"`
	Author           string                   `json:"author,omitempty"`
	Homepage         string                   `json:"homepage,omitempty"`
	License          string                   `json:"license,omitempty"`
	MinOSVersion     string                   `json:"minOSVersion,omitempty"`
	CategoryID       *int                     `json:"categoryId,omitempty"`
	Category         string                   `json:"category,omitempty"`
	CategoryI18n     map[string]string        `json:"categoryI18n,omitempty"`
	IconURL          string                   `json:"iconUrl,omitempty"`
	InstallProtected bool                     `json:"installProtected"`
	CommentsEnabled  bool                     `json:"commentsEnabled"`
	OutdatedMarks    int                      `json:"outdatedMarks,omitempty"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots,omitempty"`
	LatestVersion    *VersionDTO              `json:"latestVersion,omitempty"`
	Versions         []VersionDTO             `json:"versions,omitempty"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

type SyncAllResult struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

type InstalledApplicationDTO struct {
	AppID             string `json:"appid"`
	Title             string `json:"title,omitempty"`
	Version           string `json:"version,omitempty"`
	Status            string `json:"status,omitempty"`
	InstanceStatus    string `json:"instanceStatus,omitempty"`
	Icon              string `json:"icon,omitempty"`
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
}

type ClientAppUpdatePolicyDTO struct {
	PackageID         string `json:"packageId"`
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
}

type ClientAppUpdatePolicyInput struct {
	AutoUpdateEnabled *bool `json:"autoUpdateEnabled"`
}

type InstallRequestDTO struct {
	AppID           int    `json:"appId"`
	Version         string `json:"version,omitempty"`
	InstallPassword string `json:"installPassword,omitempty"`
	MirrorID        string `json:"mirrorId,omitempty"`
	Name            string `json:"name,omitempty"`
	PackageID       string `json:"pkgId,omitempty"`
	DownloadURL     string `json:"downloadUrl,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
}

type InstallResultDTO struct {
	Mode   string `json:"mode"`
	TaskID string `json:"taskId,omitempty"`
	Status string `json:"status,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type InstallTaskDTO struct {
	TaskID         string  `json:"taskId"`
	Status         string  `json:"status"`
	DownloadedSize uint64  `json:"downloadedSize,omitempty"`
	TotalSize      *uint64 `json:"totalSize,omitempty"`
	Detail         string  `json:"detail,omitempty"`
}

type UpdateQueueItemDTO struct {
	AppID            int    `json:"appId"`
	SourceID         int    `json:"sourceId"`
	SourceName       string `json:"sourceName"`
	PackageID        string `json:"packageId"`
	AppName          string `json:"appName"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	Version          string `json:"version,omitempty"`
	Status           string `json:"status"`
	Detail           string `json:"detail,omitempty"`
}

type UpdateQueueMirrorOverrideDTO struct {
	SourceID         int    `json:"sourceId"`
	DownloadMirrorID string `json:"downloadMirrorId"`
	RawMirrorID      string `json:"rawMirrorId"`
}

type UpdateQueueCandidateDTO struct {
	AppID            int    `json:"appId"`
	SourceID         int    `json:"sourceId"`
	PackageID        string `json:"packageId"`
	InstalledVersion string `json:"installedVersion"`
	TargetVersion    string `json:"targetVersion"`
}

type UpdateQueueRequestDTO struct {
	MirrorOverrides         []UpdateQueueMirrorOverrideDTO `json:"mirrorOverrides,omitempty"`
	Candidates              []UpdateQueueCandidateDTO      `json:"candidates,omitempty"`
	RespectAutoUpdatePolicy bool                           `json:"-"`
}

type UpdateQueueResultDTO struct {
	Status           string               `json:"status"`
	Items            []UpdateQueueItemDTO `json:"items,omitempty"`
	Error            string               `json:"error,omitempty"`
	PasswordRequired int                  `json:"passwordRequired,omitempty"`
}

type PackageManager interface {
	QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error)
	InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error)
	GetInstallTask(ctx context.Context, userID, taskID string) (InstallTaskDTO, error)
	CancelInstall(ctx context.Context, userID, taskID string) error
}

type InstallHistoryDTO struct {
	ID          int       `json:"id"`
	SourceID    *int      `json:"sourceId,omitempty"`
	SourceAppID *int      `json:"sourceAppId,omitempty"`
	SourceName  string    `json:"sourceName,omitempty"`
	PackageID   string    `json:"packageId"`
	AppName     string    `json:"appName"`
	Version     string    `json:"version,omitempty"`
	Result      string    `json:"result"`
	DownloadURL string    `json:"downloadUrl,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CommentDTO struct {
	ID           int          `json:"id"`
	AppID        int          `json:"appId"`
	UserID       int          `json:"userId"`
	ParentID     *int         `json:"parentId,omitempty"`
	AuthorType   string       `json:"authorType"`
	ClientUserID string       `json:"clientUserId,omitempty"`
	Username     string       `json:"username"`
	Body         string       `json:"body"`
	CanDelete    bool         `json:"canDelete"`
	Replies      []CommentDTO `json:"replies,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type CommentInput struct {
	Body        string `json:"body"`
	ParentID    *int   `json:"parentId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type ClientSettingsDTO struct {
	ClientTitle                  string     `json:"clientTitle"`
	CommentDisplayName           string     `json:"commentDisplayName"`
	DefaultPageSize              int        `json:"defaultPageSize"`
	AutoSyncEnabled              bool       `json:"autoSyncEnabled"`
	AutoSyncIntervalMinutes      int        `json:"autoSyncIntervalMinutes"`
	SyncOnStartup                bool       `json:"syncOnStartup"`
	InstallSuccessDismissSeconds int        `json:"installSuccessDismissSeconds"`
	LastAutoSyncAt               *time.Time `json:"lastAutoSyncAt,omitempty"`
	LastAutoSyncStatus           string     `json:"lastAutoSyncStatus,omitempty"`
	LastAutoSyncError            string     `json:"lastAutoSyncError,omitempty"`
	AutoUpdateEnabled            bool       `json:"autoUpdateEnabled"`
	AutoUpdateIntervalMinutes    int        `json:"autoUpdateIntervalMinutes"`
	LastAutoUpdateAt             *time.Time `json:"lastAutoUpdateAt,omitempty"`
	LastAutoUpdateStatus         string     `json:"lastAutoUpdateStatus,omitempty"`
	LastAutoUpdateError          string     `json:"lastAutoUpdateError,omitempty"`
}

type ClientSettingsUpdateDTO struct {
	ClientTitle                  string `json:"clientTitle"`
	CommentDisplayName           string `json:"commentDisplayName"`
	DefaultPageSize              int    `json:"defaultPageSize"`
	AutoSyncEnabled              bool   `json:"autoSyncEnabled"`
	AutoSyncIntervalMinutes      int    `json:"autoSyncIntervalMinutes"`
	SyncOnStartup                bool   `json:"syncOnStartup"`
	InstallSuccessDismissSeconds *int   `json:"installSuccessDismissSeconds"`
	AutoUpdateEnabled            bool   `json:"autoUpdateEnabled"`
	AutoUpdateIntervalMinutes    int    `json:"autoUpdateIntervalMinutes"`
}
