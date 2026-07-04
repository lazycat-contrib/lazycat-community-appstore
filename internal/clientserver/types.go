package clientserver

import "time"

type SourceDTO struct {
	ID                   int        `json:"id"`
	Name                 string     `json:"name"`
	URL                  string     `json:"url"`
	Password             string     `json:"password"`
	Mirror               string     `json:"mirror"`
	LastSync             *time.Time `json:"lastSync,omitempty"`
	LastError            string     `json:"lastError,omitempty"`
	LastErrorCode        string     `json:"lastErrorCode,omitempty"`
	LastAppCount         int        `json:"lastAppCount"`
	LastInstallableCount int        `json:"lastInstallableCount"`
}

type SourceInput struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Password string `json:"password"`
	Mirror   string `json:"mirror"`
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
	ID               int         `json:"id"`
	SourceID         int         `json:"sourceId"`
	SourceName       string      `json:"sourceName"`
	Name             string      `json:"name"`
	Slug             string      `json:"slug"`
	Summary          string      `json:"summary"`
	Category         string      `json:"category,omitempty"`
	InstallProtected bool        `json:"installProtected"`
	LatestVersion    *VersionDTO `json:"latestVersion,omitempty"`
}

type SyncAllResult struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
}
