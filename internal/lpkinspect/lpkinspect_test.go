package lpkinspect

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"lazycat.community/appstore/internal/mirror"
)

func TestFetchURLsUsesMirrorsBeforeOriginalURL(t *testing.T) {
	mirrors := []mirror.Entry{
		{Kind: mirror.KindRaw, URL: "https://raw-mirror.test/https://raw.githubusercontent.com"},
		{Kind: mirror.KindDownload, URL: "https://release-mirror.test/https://github.com"},
	}

	rawCandidates, err := FetchURLs("https://github.com/acme/demo/raw/main/app.lpk", true, mirrors)
	if err != nil {
		t.Fatalf("FetchURLs raw: %v", err)
	}
	if got, want := rawCandidates[0].String(), "https://raw-mirror.test/https://raw.githubusercontent.com/acme/demo/main/app.lpk"; got != want {
		t.Fatalf("raw mirror candidate = %q, want %q", got, want)
	}
	if got, want := rawCandidates[1].String(), "https://raw.githubusercontent.com/acme/demo/main/app.lpk"; got != want {
		t.Fatalf("raw original candidate = %q, want %q", got, want)
	}

	releaseCandidates, err := FetchURLs("https://github.com/acme/demo/releases/download/v1/app.lpk", true, mirrors)
	if err != nil {
		t.Fatalf("FetchURLs release: %v", err)
	}
	if got, want := releaseCandidates[0].String(), "https://release-mirror.test/https://github.com/acme/demo/releases/download/v1/app.lpk"; got != want {
		t.Fatalf("release mirror candidate = %q, want %q", got, want)
	}
}

func TestInspectURLRetriesMirrorCandidates(t *testing.T) {
	lpk := testLPKArchive(t, `package: cloud.lazycat.test.inspect
version: 1.0.0
name: Inspect
description: Inspect package
`)
	sum := sha256.Sum256(lpk)
	firstHits := 0
	secondHits := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits++
		http.Error(w, "bad mirror", http.StatusBadGateway)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits++
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(lpk)
	}))
	defer second.Close()

	inspected, err := InspectURL(t.Context(), "https://github.com/acme/demo/releases/download/v1/app.lpk", URLOptions{
		MaxBytes:          int64(len(lpk) + 1024),
		UseMirrorDownload: true,
		Mirrors: []mirror.Entry{
			{Kind: mirror.KindDownload, URL: first.URL},
			{Kind: mirror.KindDownload, URL: second.URL},
		},
		AllowPrivateHosts: true,
		TotalTimeout:      time.Second,
		CandidateTimeout:  200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("InspectURL: %v", err)
	}
	if firstHits != 1 || secondHits != 1 {
		t.Fatalf("mirror hits first=%d second=%d, want 1/1", firstHits, secondHits)
	}
	if inspected.Metadata.PackageID != "cloud.lazycat.test.inspect" || inspected.Metadata.Version != "1.0.0" {
		t.Fatalf("unexpected metadata: %+v", inspected.Metadata)
	}
	if inspected.SHA256 != hex.EncodeToString(sum[:]) || inspected.Size != int64(len(lpk)) {
		t.Fatalf("unexpected inspection hash/size: sha=%q size=%d", inspected.SHA256, inspected.Size)
	}
}

func TestParseUploadedSeeksBackToStart(t *testing.T) {
	lpk := testLPKArchive(t, `package: cloud.lazycat.test.upload
version: 1.0.0
name: Upload
description: Upload package
`)
	file, err := os.CreateTemp(t.TempDir(), "upload-*.lpk")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := file.Write(lpk); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	meta, err := ParseUploaded(file, &multipart.FileHeader{Filename: "demo.lpk", Size: int64(len(lpk))}, int64(len(lpk)+1024))
	if err != nil {
		t.Fatalf("ParseUploaded: %v", err)
	}
	if meta.PackageID != "cloud.lazycat.test.upload" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	readBack, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(readBack, lpk) {
		t.Fatalf("file was not rewound after parse")
	}
}

func TestParseUploadedReadsMetadataAndIconWithToolkit(t *testing.T) {
	icon := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	lpk := testLPKArchiveWithFiles(t, `package: cloud.lazycat.test.toolkit
version: 1.2.3
icon: assets/icon.png
locales:
  zh:
    name: 工具包应用
    description: 工具包描述
  en:
    name: Toolkit app
    description: Toolkit description
`, map[string][]byte{"assets/icon.png": icon})
	file, err := os.CreateTemp(t.TempDir(), "toolkit-*.lpk")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := file.Write(lpk); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	meta, err := ParseUploaded(file, &multipart.FileHeader{Filename: "toolkit.lpk", Size: int64(len(lpk))}, int64(len(lpk)+1024))
	if err != nil {
		t.Fatalf("ParseUploaded: %v", err)
	}
	if meta.PackageID != "cloud.lazycat.test.toolkit" || meta.Version != "1.2.3" {
		t.Fatalf("metadata = %+v", meta)
	}
	if meta.Name != "工具包应用" || meta.Description != "工具包描述" || meta.NameI18n["en"] != "Toolkit app" {
		t.Fatalf("localized metadata = %+v", meta)
	}
	if meta.IconPath != "assets/icon.png" || meta.IconMediaType != "image/png" || !bytes.Equal(meta.IconData, icon) {
		t.Fatalf("icon metadata = %+v", meta)
	}
}

func testLPKArchive(t *testing.T, packageYAML string) []byte {
	return testLPKArchiveWithFiles(t, packageYAML, nil)
}

func testLPKArchiveWithFiles(t *testing.T, packageYAML string, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "package.yml", Mode: 0o644, Size: int64(len(packageYAML))}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write([]byte(packageYAML)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatalf("WriteHeader %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("Write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close tar: %v", err)
	}
	return buf.Bytes()
}
