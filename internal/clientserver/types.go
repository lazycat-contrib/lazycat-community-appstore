package clientserver

import (
	"context"
	"time"

	"lazycat.community/appstore/internal/mirror"
)

type SourceDTO struct {
	ID                      int            `json:"id"`
	Name                    string         `json:"name"`
	URL                     string         `json:"url"`
	Password                string         `json:"password"`
	DefaultDownloadMirrorID string         `json:"defaultDownloadMirrorId"`
	DefaultRawMirrorID      string         `json:"defaultRawMirrorId"`
	GitHubMirrors           []mirror.Entry `json:"githubMirrors"`
	LastSync                *time.Time     `json:"lastSync,omitempty"`
	LastError               string         `json:"lastError,omitempty"`
	LastErrorCode           string         `json:"lastErrorCode,omitempty"`
	LastAppCount            int            `json:"lastAppCount"`
	LastInstallableCount    int            `json:"lastInstallableCount"`
}

type SourceInput struct {
	Name                    string `json:"name"`
	URL                     string `json:"url"`
	Password                string `json:"password"`
	DefaultDownloadMirrorID string `json:"defaultDownloadMirrorId"`
	DefaultRawMirrorID      string `json:"defaultRawMirrorId"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type VersionDTO struct {
	Version             string `json:"version"`
	DownloadURL         string `json:"downloadUrl"`
	UpstreamDownloadURL string `json:"upstreamDownloadUrl,omitempty"`
	SourceType          string `json:"sourceType,omitempty"`
	SHA256              string `json:"sha256"`
	Size                int64  `json:"size"`
}

type SourceAppDTO struct {
	ID               int          `json:"id"`
	SourceID         int          `json:"sourceId"`
	SourceName       string       `json:"sourceName"`
	ExternalID       string       `json:"externalId"`
	PackageID        string       `json:"packageId"`
	Name             string       `json:"name"`
	Slug             string       `json:"slug"`
	Summary          string       `json:"summary"`
	Category         string       `json:"category,omitempty"`
	IconURL          string       `json:"iconUrl,omitempty"`
	InstallProtected bool         `json:"installProtected"`
	LatestVersion    *VersionDTO  `json:"latestVersion,omitempty"`
	Versions         []VersionDTO `json:"versions,omitempty"`
}

type SyncAllResult struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

type InstalledApplicationDTO struct {
	AppID          string `json:"appid"`
	Title          string `json:"title,omitempty"`
	Version        string `json:"version,omitempty"`
	Status         string `json:"status,omitempty"`
	InstanceStatus string `json:"instanceStatus,omitempty"`
	Icon           string `json:"icon,omitempty"`
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

type PackageManager interface {
	QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error)
	InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error)
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
	CommentDisplayName string `json:"commentDisplayName"`
}
