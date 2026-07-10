package migration

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

const (
	manifestName         = "manifest.json"
	maxZipEntries        = 4096
	maxCompressedBytes   = 512 << 20
	maxDecompressedBytes = 2 << 30
	maxJSONEntryBytes    = 64 << 20
)

func PreviewPackage(_ context.Context, r io.ReaderAt, size int64) (*Preview, error) {
	manifest, err := readManifestFromReaderAt(r, size)
	if err != nil {
		return nil, err
	}
	return previewFromManifest(manifest), nil
}

func previewFromManifest(manifest Manifest) *Preview {
	return &Preview{
		FormatVersion:  manifest.FormatVersion,
		ServerVersion:  manifest.ServerVersion,
		CreatedAt:      manifest.CreatedAt,
		Modules:        manifest.Modules,
		Counts:         manifest.Counts,
		TotalFileBytes: manifest.TotalFileBytes,
		Warnings:       manifest.Warnings,
	}
}

func readManifestFromReaderAt(r io.ReaderAt, size int64) (Manifest, error) {
	zr, err := zipReaderFromReaderAt(r, size)
	if err != nil {
		return Manifest{}, err
	}
	return readManifest(zr)
}

func zipReaderFromReaderAt(r io.ReaderAt, size int64) (*zip.Reader, error) {
	if r == nil {
		return nil, fmt.Errorf("migration package reader is required")
	}
	if size < 0 || size > maxCompressedBytes {
		return nil, fmt.Errorf("migration package is too large")
	}
	return zip.NewReader(r, size)
}

func readManifest(zr *zip.Reader) (Manifest, error) {
	if len(zr.File) > maxZipEntries {
		return Manifest{}, fmt.Errorf("migration package has too many files")
	}
	var total uint64
	for _, file := range zr.File {
		if err := validateZipEntry(file); err != nil {
			return Manifest{}, err
		}
		total += file.UncompressedSize64
		if total > maxDecompressedBytes {
			return Manifest{}, fmt.Errorf("migration package expands too large")
		}
	}
	data, err := readZipEntry(zr, manifestName, maxJSONEntryBytes)
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest is required")
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest is invalid")
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported migration format version")
	}
	seen := map[Module]bool{}
	for _, module := range manifest.Modules {
		switch module {
		case ModuleSite, ModulePeople, ModuleApps, ModuleFiles:
			seen[module] = true
		default:
			return fmt.Errorf("unknown migration module %q", module)
		}
	}
	if seen[ModuleFiles] && !seen[ModuleApps] {
		return fmt.Errorf("files module requires apps module")
	}
	for _, file := range manifest.Files {
		if !isSafeZipPath(file.Path) || !strings.HasPrefix(file.Path, "files/") {
			return fmt.Errorf("manifest contains unsafe file path")
		}
		if strings.TrimSpace(file.StorageKey) == "" || !isSafeStoragePath(file.StoragePath) {
			return fmt.Errorf("manifest contains unsafe storage path")
		}
		if file.Size < 0 {
			return fmt.Errorf("manifest contains invalid file size")
		}
	}
	return nil
}

func validateZipEntry(file *zip.File) error {
	if !isSafeZipPath(file.Name) {
		return fmt.Errorf("migration package contains unsafe path")
	}
	mode := file.FileInfo().Mode()
	if mode&^0o777 != 0 {
		return fmt.Errorf("migration package contains unsupported file type")
	}
	return nil
}

func readZipEntry(zr *zip.Reader, name string, maxBytes int64) ([]byte, error) {
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		if file.UncompressedSize64 > uint64(maxBytes) {
			return nil, fmt.Errorf("zip entry is too large")
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("zip entry is too large")
		}
		return data, nil
	}
	return nil, fmt.Errorf("zip entry not found")
}

func writeJSONEntry(zw *zip.Writer, name string, value any) error {
	if !isSafeZipPath(name) {
		return fmt.Errorf("unsafe zip path")
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func readJSONEntry(zr *zip.Reader, name string, target any) error {
	data, err := readZipEntry(zr, name, maxJSONEntryBytes)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

func isSafeZipPath(name string) bool {
	if strings.TrimSpace(name) == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	clean := path.Clean(name)
	if clean == "." || clean != name {
		return false
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func isSafeStoragePath(name string) bool {
	if strings.TrimSpace(name) == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	clean := path.Clean(name)
	if clean == "." {
		return false
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
