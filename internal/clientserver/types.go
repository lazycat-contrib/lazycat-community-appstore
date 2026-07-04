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
