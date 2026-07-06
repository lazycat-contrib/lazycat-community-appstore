package lpkmeta

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

const packageYAML = "package.yml"
const maxPackageYAMLBytes = 1 << 20

var (
	ErrPackageNotFound = errors.New("package.yml not found")
	ErrInvalidPackage  = errors.New("invalid package.yml")
)

type Metadata struct {
	PackageID    string
	Version      string
	Name         string
	Description  string
	Author       string
	License      string
	Homepage     string
	MinOSVersion string
}

type packageFile struct {
	Package      string                   `yaml:"package"`
	Version      string                   `yaml:"version"`
	Name         string                   `yaml:"name"`
	Description  string                   `yaml:"description"`
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
	for _, file := range zr.File {
		if cleanArchiveName(file.Name) != packageYAML {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return Metadata{}, err
		}
		defer rc.Close()
		return parsePackageYAML(io.LimitReader(rc, maxPackageYAMLBytes+1))
	}
	return Metadata{}, ErrPackageNotFound
}

func parseTar(r io.Reader) (Metadata, error) {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return Metadata{}, ErrPackageNotFound
		}
		if err != nil {
			return Metadata{}, err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if cleanArchiveName(header.Name) != packageYAML {
			continue
		}
		return parsePackageYAML(io.LimitReader(tr, maxPackageYAMLBytes+1))
	}
}

func parsePackageYAML(r io.Reader) (Metadata, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Metadata{}, err
	}
	if len(raw) > maxPackageYAMLBytes {
		return Metadata{}, fmt.Errorf("%w: package.yml exceeds %d bytes", ErrInvalidPackage, maxPackageYAMLBytes)
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
	return meta, nil
}

func normalize(pkg packageFile) Metadata {
	meta := Metadata{
		PackageID:    strings.TrimSpace(pkg.Package),
		Version:      strings.TrimSpace(pkg.Version),
		Name:         strings.TrimSpace(pkg.Name),
		Description:  strings.TrimSpace(pkg.Description),
		Author:       strings.TrimSpace(pkg.Author),
		License:      strings.TrimSpace(pkg.License),
		Homepage:     strings.TrimSpace(pkg.Homepage),
		MinOSVersion: strings.TrimSpace(pkg.MinOSVersion),
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

func cleanArchiveName(name string) string {
	clean := path.Clean(strings.TrimSpace(strings.TrimPrefix(name, "./")))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return ""
	}
	return clean
}
