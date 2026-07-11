package lpkinspect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/lib-x/lzc-toolkit-go/archive"
	"github.com/lib-x/lzc-toolkit-go/lpk"

	"lazycat.community/appstore/internal/catalogmeta"
)

const maxLPKIconBytes = 2 << 20

var (
	ErrPackageNotFound = errors.New("package.yml not found")
	ErrInvalidPackage  = errors.New("invalid package.yml")
)

// Metadata is the application metadata extracted from an LPK. Archive and
// package.yml parsing is delegated to lzc-toolkit-go; this type keeps that
// third-party representation out of server handlers.
type Metadata struct {
	PackageID       string
	Version         string
	Name            string
	NameI18n        catalogmeta.LocalizedText
	Description     string
	DescriptionI18n catalogmeta.LocalizedText
	Author          string
	License         string
	Homepage        string
	MinOSVersion    string
	IconPath        string
	IconMediaType   string
	IconData        []byte
}

type packageFile struct {
	Package      string                   `yaml:"package"`
	Version      string                   `yaml:"version"`
	Name         string                   `yaml:"name"`
	Description  string                   `yaml:"description"`
	Icon         string                   `yaml:"icon"`
	Author       string                   `yaml:"author"`
	License      string                   `yaml:"license"`
	Homepage     string                   `yaml:"homepage"`
	MinOSVersion string                   `yaml:"min_os_version"`
	Locales      map[string]packageLocale `yaml:"locales"`
}

type packageLocale struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func parseLPKReaderAt(ctx context.Context, src io.ReaderAt, size, maxInputBytes int64) (Metadata, error) {
	reader, err := lpk.OpenReaderAt(ctx, src, size, lpk.WithLimits(lpkArchiveLimits(maxInputBytes)))
	if err != nil {
		return Metadata{}, err
	}
	defer func() { _ = reader.Close() }()
	return parseLPKReader(ctx, reader)
}

func parseLPKFile(ctx context.Context, filename string, maxInputBytes int64) (Metadata, error) {
	reader, err := lpk.OpenFile(ctx, filename, lpk.WithLimits(lpkArchiveLimits(maxInputBytes)))
	if err != nil {
		return Metadata{}, err
	}
	defer func() { _ = reader.Close() }()
	return parseLPKReader(ctx, reader)
}

func lpkArchiveLimits(maxInputBytes int64) archive.Limits {
	limits := archive.DefaultLimits()
	if maxInputBytes > 0 {
		limits.MaxInputBytes = maxInputBytes
	}
	return limits
}

func parseLPKReader(ctx context.Context, reader *lpk.Reader) (Metadata, error) {
	document, err := reader.PackageInfo(ctx)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Metadata{}, ErrPackageNotFound
		}
		return Metadata{}, err
	}
	var pkg packageFile
	if err := document.Decode(&pkg); err != nil {
		return Metadata{}, fmt.Errorf("%w: %v", ErrInvalidPackage, err)
	}
	meta := normalizeMetadata(pkg)
	if meta.PackageID == "" {
		return Metadata{}, fmt.Errorf("%w: package is required", ErrInvalidPackage)
	}
	if meta.Version == "" {
		return Metadata{}, fmt.Errorf("%w: version is required", ErrInvalidPackage)
	}
	return applyIcon(ctx, reader, pkg.Icon, meta)
}

func normalizeMetadata(pkg packageFile) Metadata {
	nameI18n := catalogmeta.LocalizedText{}
	descriptionI18n := catalogmeta.LocalizedText{}
	for key, locale := range pkg.Locales {
		if name := strings.TrimSpace(locale.Name); name != "" {
			nameI18n[key] = name
		}
		if description := strings.TrimSpace(locale.Description); description != "" {
			descriptionI18n[key] = description
		}
	}
	meta := Metadata{
		PackageID:       strings.TrimSpace(pkg.Package),
		Version:         strings.TrimSpace(pkg.Version),
		Name:            strings.TrimSpace(pkg.Name),
		NameI18n:        catalogmeta.CleanLocalizedText(nameI18n),
		Description:     strings.TrimSpace(pkg.Description),
		DescriptionI18n: catalogmeta.CleanLocalizedText(descriptionI18n),
		Author:          strings.TrimSpace(pkg.Author),
		License:         strings.TrimSpace(pkg.License),
		Homepage:        strings.TrimSpace(pkg.Homepage),
		MinOSVersion:    strings.TrimSpace(pkg.MinOSVersion),
		IconPath:        cleanArchiveName(pkg.Icon),
	}
	for _, locale := range []string{"zh", "zh-CN", "zh_Hans", "en"} {
		entry, ok := pkg.Locales[locale]
		if !ok {
			continue
		}
		if meta.Name == "" {
			meta.Name = strings.TrimSpace(entry.Name)
		}
		if meta.Description == "" {
			meta.Description = strings.TrimSpace(entry.Description)
		}
		if meta.Name != "" && meta.Description != "" {
			break
		}
	}
	return meta
}

func applyIcon(ctx context.Context, reader *lpk.Reader, configured string, meta Metadata) (Metadata, error) {
	entries, err := reader.Entries(ctx)
	if err != nil {
		return Metadata{}, err
	}
	regular := make(map[string]struct{}, len(entries))
	potential := make([]string, 0)
	for _, entry := range entries {
		if entry.Type != archive.EntryRegular {
			continue
		}
		regular[entry.Name] = struct{}{}
		if isPotentialIconFile(entry.Name) {
			potential = append(potential, entry.Name)
		}
	}
	candidates := []string{meta.IconPath, "icon.png", "icon.jpg", "icon.jpeg", "icon.webp"}
	if strings.TrimSpace(configured) == "" {
		candidates = append(candidates, potential...)
	}
	for _, candidate := range candidates {
		candidate = cleanArchiveName(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := regular[candidate]; !ok {
			continue
		}
		data, mediaType, err := readIconEntry(ctx, reader, candidate)
		if err != nil {
			return Metadata{}, err
		}
		meta.IconPath = candidate
		meta.IconData = data
		meta.IconMediaType = mediaType
		return meta, nil
	}
	return meta, nil
}

func readIconEntry(ctx context.Context, reader *lpk.Reader, name string) ([]byte, string, error) {
	contents, err := reader.OpenEntry(ctx, name)
	if err != nil {
		return nil, "", err
	}
	data, readErr := io.ReadAll(io.LimitReader(contents, maxLPKIconBytes+1))
	closeErr := contents.Close()
	if readErr != nil {
		return nil, "", readErr
	}
	if closeErr != nil {
		return nil, "", closeErr
	}
	if len(data) > maxLPKIconBytes {
		return nil, "", fmt.Errorf("%w: icon exceeds %d bytes", ErrInvalidPackage, maxLPKIconBytes)
	}
	mediaType := http.DetectContentType(data)
	if mediaType == "application/octet-stream" {
		if byExtension := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); byExtension != "" {
			mediaType = strings.Split(byExtension, ";")[0]
		}
	}
	return data, mediaType, nil
}

func cleanArchiveName(name string) string {
	clean := path.Clean(strings.TrimSpace(strings.TrimPrefix(name, "./")))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return ""
	}
	return clean
}

func isPotentialIconFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".webp" {
		return false
	}
	base := strings.ToLower(path.Base(name))
	return base == "icon"+extension || strings.Contains(base, "icon")
}
