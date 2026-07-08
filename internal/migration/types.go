package migration

import (
	"context"
	"io"
	"time"

	"lazycat.community/appstore/internal/storage"
)

const FormatVersion = 1

type Module string

const (
	ModuleSite   Module = "site"
	ModulePeople Module = "people"
	ModuleApps   Module = "apps"
	ModuleFiles  Module = "files"
)

type Options struct {
	IncludeSite   bool `json:"includeSite"`
	IncludePeople bool `json:"includePeople"`
	IncludeApps   bool `json:"includeApps"`
	IncludeFiles  bool `json:"includeFiles"`
}

type ImportMode string

const (
	ImportModeMerge   ImportMode = "merge"
	ImportModeReplace ImportMode = "replace"
)

type ImportOptions struct {
	Options
	Mode           ImportMode `json:"mode"`
	ConfirmReplace string     `json:"confirmReplace,omitempty"`
	ActorUserID    int        `json:"-"`
}

type Manifest struct {
	FormatVersion  int            `json:"formatVersion"`
	ServerVersion  string         `json:"serverVersion"`
	CreatedAt      time.Time      `json:"createdAt"`
	Modules        []Module       `json:"modules"`
	Counts         map[string]int `json:"counts"`
	Files          []FileManifest `json:"files,omitempty"`
	TotalFileBytes int64          `json:"totalFileBytes"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type FileManifest struct {
	Path        string `json:"path"`
	StorageKey  string `json:"storageKey"`
	StoragePath string `json:"storagePath"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
}

type Preview struct {
	FormatVersion  int            `json:"formatVersion"`
	ServerVersion  string         `json:"serverVersion"`
	CreatedAt      time.Time      `json:"createdAt"`
	Modules        []Module       `json:"modules"`
	Counts         map[string]int `json:"counts"`
	TotalFileBytes int64          `json:"totalFileBytes"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type ImportResult struct {
	Mode     ImportMode `json:"mode"`
	Created  int        `json:"created"`
	Updated  int        `json:"updated"`
	Skipped  int        `json:"skipped"`
	Warnings []string   `json:"warnings,omitempty"`
}

type StorageResolver interface {
	BackendForKey(ctx context.Context, key string) (storage.Backend, error)
}

type StorageResolverFunc func(ctx context.Context, key string) (storage.Backend, error)

func (fn StorageResolverFunc) BackendForKey(ctx context.Context, key string) (storage.Backend, error) {
	return fn(ctx, key)
}

type StoredFile struct {
	Manifest FileManifest
	Reader   io.ReadCloser
}

func DefaultOptions() Options {
	return Options{IncludeSite: true, IncludePeople: true, IncludeApps: true, IncludeFiles: true}
}

func NormalizeOptions(options Options) Options {
	if !options.IncludeSite && !options.IncludePeople && !options.IncludeApps && !options.IncludeFiles {
		return options
	}
	if options.IncludeFiles {
		options.IncludeApps = true
	}
	return options
}

func (o Options) Modules() []Module {
	modules := make([]Module, 0, 4)
	if o.IncludeSite {
		modules = append(modules, ModuleSite)
	}
	if o.IncludePeople {
		modules = append(modules, ModulePeople)
	}
	if o.IncludeApps {
		modules = append(modules, ModuleApps)
	}
	if o.IncludeFiles {
		modules = append(modules, ModuleFiles)
	}
	return modules
}

func OptionsFromModules(modules []Module) Options {
	var options Options
	for _, module := range modules {
		switch module {
		case ModuleSite:
			options.IncludeSite = true
		case ModulePeople:
			options.IncludePeople = true
		case ModuleApps:
			options.IncludeApps = true
		case ModuleFiles:
			options.IncludeFiles = true
		}
	}
	return NormalizeOptions(options)
}
