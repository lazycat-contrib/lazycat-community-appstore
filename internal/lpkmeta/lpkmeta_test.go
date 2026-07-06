package lpkmeta

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

const validPackageYAML = `package: cloud.lazycat.app.notes
version: 1.2.3
name: Notes
description: Source synced notes
icon: icon.png
author: LazyCat
license: MIT
homepage: https://example.com/notes
min_os_version: 1.5.0
`

var tinyPNG = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d}

func TestParseReaderAtTarPackageYAML(t *testing.T) {
	raw := tarLPK(t, validPackageYAML)

	meta, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ParseReaderAt returned error: %v", err)
	}

	if meta.PackageID != "cloud.lazycat.app.notes" || meta.Version != "1.2.3" || meta.Name != "Notes" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if meta.Description != "Source synced notes" || meta.MinOSVersion != "1.5.0" {
		t.Fatalf("unexpected metadata details: %+v", meta)
	}
}

func TestParseReaderAtZipPackageYAML(t *testing.T) {
	raw := zipLPK(t, validPackageYAML)

	meta, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ParseReaderAt returned error: %v", err)
	}

	if meta.PackageID != "cloud.lazycat.app.notes" || meta.Version != "1.2.3" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
}

func TestParseReaderAtExtractsTarIcon(t *testing.T) {
	raw := tarArchive(t, map[string]string{
		packageYAMLName(): validPackageYAML,
		"icon.png":        string(tinyPNG),
	})

	meta, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ParseReaderAt returned error: %v", err)
	}

	if meta.IconPath != "icon.png" || meta.IconMediaType != "image/png" || !bytes.Equal(meta.IconData, tinyPNG) {
		t.Fatalf("unexpected icon metadata: path=%q media=%q data=%x", meta.IconPath, meta.IconMediaType, meta.IconData)
	}
	if got := meta.IconDataURL(); !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("IconDataURL = %q, want png data URL", got)
	}
}

func TestParseReaderAtExtractsZipIcon(t *testing.T) {
	packageYAML := strings.Replace(validPackageYAML, "icon: icon.png", "icon: assets/icon.png", 1)
	raw := zipArchive(t, map[string]string{
		packageYAMLName(): packageYAML,
		"assets/icon.png": string(tinyPNG),
	})

	meta, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ParseReaderAt returned error: %v", err)
	}

	if meta.IconPath != "assets/icon.png" || meta.IconMediaType != "image/png" || !bytes.Equal(meta.IconData, tinyPNG) {
		t.Fatalf("unexpected icon metadata: path=%q media=%q data=%x", meta.IconPath, meta.IconMediaType, meta.IconData)
	}
}

func TestParseReaderAtUsesLocaleNameWhenTopLevelNameIsEmpty(t *testing.T) {
	raw := tarLPK(t, `package: cloud.lazycat.app.reader
version: 0.9.0
locales:
  zh:
    name: 阅读器
    description: 读书工具
`)

	meta, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ParseReaderAt returned error: %v", err)
	}

	if meta.Name != "阅读器" || meta.Description != "读书工具" {
		t.Fatalf("unexpected localized metadata: %+v", meta)
	}
}

func TestParseReaderAtRejectsMissingPackageYAML(t *testing.T) {
	raw := tarArchive(t, map[string]string{"manifest.yml": "package: old\n"})

	_, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("error = %v, want ErrPackageNotFound", err)
	}
}

func TestParseReaderAtRejectsMalformedPackageYAML(t *testing.T) {
	raw := tarLPK(t, "package: [")

	_, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("error = %v, want ErrInvalidPackage", err)
	}
}

func TestParseReaderAtRejectsEmptyRequiredFields(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{name: "package", body: "version: 1.0.0\n", want: "package is required"},
		{name: "version", body: "package: cloud.lazycat.app.empty\n", want: "version is required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := tarLPK(t, tt.body)
			_, err := ParseReaderAt(bytes.NewReader(raw), int64(len(raw)))
			if !errors.Is(err, ErrInvalidPackage) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func tarLPK(t *testing.T, packageYAML string) []byte {
	t.Helper()
	return tarArchive(t, map[string]string{packageYAMLName(): packageYAML})
}

func tarArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close tar: %v", err)
	}
	return buf.Bytes()
}

func zipLPK(t *testing.T, packageYAML string) []byte {
	t.Helper()
	return zipArchive(t, map[string]string{packageYAMLName(): packageYAML})
}

func zipArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}
	return buf.Bytes()
}

func packageYAMLName() string {
	return "package.yml"
}
