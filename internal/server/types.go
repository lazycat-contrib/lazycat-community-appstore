package server

import (
	"time"

	"lazycat.community/appstore/ent"
)

type publicUser struct {
	ID            int       `json:"id"`
	Username      string    `json:"username"`
	Email         *string   `json:"email,omitempty"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"emailVerified"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toPublicUser(u *ent.User) publicUser {
	return publicUser{
		ID:            u.ID,
		Username:      u.Username,
		Email:         u.Email,
		Role:          string(u.Role),
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
}

type appSummary struct {
	ID                     int       `json:"id"`
	OwnerID                int       `json:"ownerId"`
	Owner                  string    `json:"owner"`
	CategoryID             *int      `json:"categoryId,omitempty"`
	Category               string    `json:"category,omitempty"`
	Name                   string    `json:"name"`
	Slug                   string    `json:"slug"`
	Summary                string    `json:"summary"`
	Description            string    `json:"description"`
	IconURL                *string   `json:"iconUrl,omitempty"`
	Status                 string    `json:"status"`
	AllowUnreviewedUpdates bool      `json:"allowUnreviewedUpdates"`
	CommentsEnabled        bool      `json:"commentsEnabled"`
	DownloadCount          int       `json:"downloadCount"`
	Tags                   []string  `json:"tags"`
	VisibleGroupIDs        []int     `json:"visibleGroupIds"`
	LatestVersion          *version  `json:"latestVersion,omitempty"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type appDetail struct {
	appSummary
	Versions         []version    `json:"versions"`
	Screenshots      []screenshot `json:"screenshots"`
	Comments         []comment    `json:"comments"`
	Favorites        int          `json:"favorites"`
	OutdatedMarks    int          `json:"outdatedMarks"`
	CanManageApp     bool         `json:"canManageApp"`
	CanUploadVersion bool         `json:"canUploadVersion"`
}

type screenshot struct {
	ID        int       `json:"id"`
	AppID     int       `json:"appId"`
	ImageURL  string    `json:"imageUrl"`
	Caption   string    `json:"caption"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
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
	StoragePath string     `json:"storagePath,omitempty"`
	FileSize    int64      `json:"fileSize"`
	SHA256      string     `json:"sha256"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type comment struct {
	ID        int       `json:"id"`
	AppID     int       `json:"appId"`
	UserID    int       `json:"userId"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
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
