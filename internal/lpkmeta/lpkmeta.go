package lpkmeta

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"lazycat.community/appstore/internal/catalogmeta"
)

const packageYAML = "package.yml"
const maxPackageYAMLBytes = 1 << 20
const maxIconBytes = 2 << 20

var (
	ErrPackageNotFound = errors.New("package.yml not found")
	ErrInvalidPackage  = errors.New("invalid package.yml")
)

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

func ParseFile(filename string) (Metadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Metadata{}, err
	}
	return ParseReaderAt(file, info.Size())
}

func ParseReaderAt(r io.ReaderAt, size int64) (Metadata, error) {
	if size <= 0 {
		return Metadata{}, ErrPackageNotFound
	}
	if meta, err := parseZip(r, size); err == nil {
		return meta, nil
	} else if !errors.Is(err, ErrPackageNotFound) {
		// If the data is not a zip archive, fall through to tar. Other zip
		// errors may still be caused by reading tar bytes through zip.NewReader.
	}
	return parseTar(io.NewSectionReader(r, 0, size))
}

func parseZip(r io.ReaderAt, size int64) (Metadata, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return Metadata{}, err
	}
	var packageRaw []byte
	icons := map[string]iconFile{}
	for _, file := range zr.File {
		name := cleanArchiveName(file.Name)
		if name != packageYAML && !isPotentialIconFile(name) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return Metadata{}, err
		}
		if name == packageYAML {
			packageRaw, err = readLimited(rc, maxPackageYAMLBytes)
		} else if file.FileInfo().Mode().IsRegular() {
			var data []byte
			data, err = readLimited(rc, maxIconBytes)
			if err == nil {
				icons[name] = iconFile{path: name, data: data}
			}
		}
		closeErr := rc.Close()
		if err != nil {
			return Metadata{}, err
		}
		if closeErr != nil {
			return Metadata{}, closeErr
		}
	}
	if len(packageRaw) == 0 {
		return Metadata{}, ErrPackageNotFound
	}
	return parsePackageYAML(bytes.NewReader(packageRaw), icons)
}

func parseTar(r io.Reader) (Metadata, error) {
	tr := tar.NewReader(r)
	var packageRaw []byte
	icons := map[string]iconFile{}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Metadata{}, err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		name := cleanArchiveName(header.Name)
		if name != packageYAML && !isPotentialIconFile(name) {
			continue
		}
		if name == packageYAML {
			packageRaw, err = readLimited(tr, maxPackageYAMLBytes)
		} else {
			var data []byte
			data, err = readLimited(tr, maxIconBytes)
			if err == nil {
				icons[name] = iconFile{path: name, data: data}
			}
		}
		if err != nil {
			return Metadata{}, err
		}
	}
	if len(packageRaw) == 0 {
		return Metadata{}, ErrPackageNotFound
	}
	return parsePackageYAML(bytes.NewReader(packageRaw), icons)
}

func parsePackageYAML(r io.Reader, icons map[string]iconFile) (Metadata, error) {
	raw, err := readLimited(r, maxPackageYAMLBytes)
	if err != nil {
		return Metadata{}, err
	}
	var pkg packageFile
	if err := yaml.Unmarshal(raw, &pkg); err != nil {
		return Metadata{}, fmt.Errorf("%w: %v", ErrInvalidPackage, err)
	}
	meta := normalize(pkg)
	if meta.PackageID == "" {
		return Metadata{}, fmt.Errorf("%w: package is required", ErrInvalidPackage)
	}
	if meta.Version == "" {
		return Metadata{}, fmt.Errorf("%w: version is required", ErrInvalidPackage)
	}
	applyIcon(&meta, pkg, icons)
	return meta, nil
}

func normalize(pkg packageFile) Metadata {
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
	if meta.Name == "" {
		for _, key := range []string{"zh", "zh-CN", "zh_Hans", "en"} {
			if locale, ok := pkg.Locales[key]; ok {
				meta.Name = strings.TrimSpace(locale.Name)
				if meta.Description == "" {
					meta.Description = strings.TrimSpace(locale.Description)
				}
				if meta.Name != "" {
					break
				}
			}
		}
	}
	if meta.Description == "" {
		for _, key := range []string{"zh", "zh-CN", "zh_Hans", "en"} {
			if locale, ok := pkg.Locales[key]; ok {
				meta.Description = strings.TrimSpace(locale.Description)
				if meta.Description != "" {
					break
				}
			}
		}
	}
	return meta
}

func (m Metadata) IconDataURL() string {
	if len(m.IconData) == 0 || m.IconMediaType == "" {
		return ""
	}
	return "data:" + m.IconMediaType + ";base64," + base64.StdEncoding.EncodeToString(m.IconData)
}

func cleanArchiveName(name string) string {
	clean := path.Clean(strings.TrimSpace(strings.TrimPrefix(name, "./")))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return ""
	}
	return clean
}

type iconFile struct {
	path string
	data []byte
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%w: archive entry exceeds %d bytes", ErrInvalidPackage, limit)
	}
	return raw, nil
}

func isPotentialIconFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return false
	}
	base := strings.ToLower(path.Base(name))
	return base == "icon"+ext || strings.Contains(base, "icon")
}

func applyIcon(meta *Metadata, pkg packageFile, icons map[string]iconFile) {
	if len(icons) == 0 {
		return
	}
	candidates := []string{}
	if meta.IconPath != "" {
		candidates = append(candidates, meta.IconPath)
	}
	candidates = append(candidates, "icon.png", "icon.jpg", "icon.jpeg", "icon.webp")
	for _, candidate := range candidates {
		if icon, ok := icons[cleanArchiveName(candidate)]; ok {
			meta.IconPath = icon.path
			meta.IconData = append([]byte(nil), icon.data...)
			meta.IconMediaType = iconMediaType(icon.path, icon.data)
			return
		}
	}
	if strings.TrimSpace(pkg.Icon) == "" {
		for _, icon := range icons {
			meta.IconPath = icon.path
			meta.IconData = append([]byte(nil), icon.data...)
			meta.IconMediaType = iconMediaType(icon.path, icon.data)
			return
		}
	}
}

func iconMediaType(name string, data []byte) string {
	if detected := httpDetectContentType(data); detected != "application/octet-stream" {
		return detected
	}
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); byExt != "" {
		return strings.Split(byExt, ";")[0]
	}
	return "application/octet-stream"
}

func httpDetectContentType(data []byte) string {
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	switch {
	case len(sample) >= 8 && bytes.Equal(sample[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png"
	case len(sample) >= 3 && bytes.Equal(sample[:3], []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg"
	case len(sample) >= 12 && string(sample[:4]) == "RIFF" && string(sample[8:12]) == "WEBP":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
