package migration

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
)

type filePayload struct {
	Manifest FileManifest
	Data     []byte
}

func collectFilePayloads(ctx context.Context, resolver StorageResolver, people PeopleData, apps AppsData, site SiteData) ([]filePayload, []string) {
	if resolver == nil {
		return nil, []string{"attachment files were skipped because storage is not configured"}
	}
	refs := collectFileRefs(people, apps, site)
	payloads := make([]filePayload, 0, len(refs))
	warnings := []string{}
	seen := map[string]bool{}
	for _, ref := range refs {
		key := ref.StorageKey + "\x00" + ref.StoragePath
		if seen[key] {
			continue
		}
		seen[key] = true
		if strings.TrimSpace(ref.StorageKey) == "" || !isSafeStoragePath(ref.StoragePath) {
			warnings = append(warnings, fmt.Sprintf("skipped unsafe attachment path for %s", ref.Kind))
			continue
		}
		backend, err := resolver.BackendForKey(ctx, ref.StorageKey)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped %s attachment: storage %s is unavailable", ref.Kind, ref.StorageKey))
			continue
		}
		reader, err := backend.Open(ctx, ref.StoragePath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped %s attachment: file is unavailable", ref.Kind))
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(reader.Body, maxJSONEntryBytes+1))
		_ = reader.Body.Close()
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("skipped %s attachment: file could not be read", ref.Kind))
			continue
		}
		if int64(len(data)) > maxJSONEntryBytes {
			warnings = append(warnings, fmt.Sprintf("skipped %s attachment: file is too large", ref.Kind))
			continue
		}
		sum := sha256.Sum256(data)
		zipPath := "files/" + path.Clean(ref.StorageKey+"/"+ref.StoragePath)
		payloads = append(payloads, filePayload{
			Manifest: FileManifest{
				Path:        zipPath,
				StorageKey:  ref.StorageKey,
				StoragePath: ref.StoragePath,
				Size:        int64(len(data)),
				SHA256:      hex.EncodeToString(sum[:]),
			},
			Data: data,
		})
	}
	return payloads, warnings
}

type fileRef struct {
	Kind        string
	StorageKey  string
	StoragePath string
}

func collectFileRefs(people PeopleData, apps AppsData, site SiteData) []fileRef {
	refs := []fileRef{}
	for _, record := range people.Users {
		if record.AvatarStoragePath != "" {
			refs = append(refs, fileRef{Kind: "avatar", StorageKey: record.AvatarStorageKey, StoragePath: record.AvatarStoragePath})
		}
	}
	for _, record := range apps.AppVersions {
		if record.StoragePath != "" && record.SourceType != "GITHUB" {
			refs = append(refs, fileRef{Kind: "version", StorageKey: record.StorageKey, StoragePath: record.StoragePath})
		}
	}
	for _, record := range apps.AppScreenshots {
		if record.StoragePath != "" {
			refs = append(refs, fileRef{Kind: "screenshot", StorageKey: record.StorageKey, StoragePath: record.StoragePath})
		}
	}
	for _, setting := range site.SiteSettings {
		if setting.Key == "site_icon_url" {
			if key, storagePath, ok := parseStorageURL(setting.Value); ok {
				refs = append(refs, fileRef{Kind: "site icon", StorageKey: key, StoragePath: storagePath})
			}
		}
	}
	return refs
}

func writeFilePayloads(zw *zip.Writer, payloads []filePayload) error {
	for _, payload := range payloads {
		header := &zip.FileHeader{Name: payload.Manifest.Path, Method: zip.Deflate}
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := w.Write(payload.Data); err != nil {
			return err
		}
	}
	return nil
}

func importFiles(ctx context.Context, zr *zip.Reader, resolver StorageResolver, manifest Manifest) (map[string]string, []string, error) {
	pathMap := map[string]string{}
	warnings := []string{}
	if len(manifest.Files) == 0 {
		return pathMap, warnings, nil
	}
	if resolver == nil {
		return nil, nil, fmt.Errorf("storage is not configured")
	}
	for _, file := range manifest.Files {
		data, err := readZipEntry(zr, file.Path, maxJSONEntryBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("read attachment: %w", err)
		}
		if int64(len(data)) != file.Size {
			return nil, nil, fmt.Errorf("attachment size mismatch")
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), file.SHA256) {
			return nil, nil, fmt.Errorf("attachment hash mismatch")
		}
		backend, err := resolver.BackendForKey(ctx, file.StorageKey)
		if err != nil {
			return nil, nil, fmt.Errorf("load attachment storage: %w", err)
		}
		obj, err := backend.Save(ctx, path.Base(file.StoragePath), bytes.NewReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("save attachment: %w", err)
		}
		pathMap[file.StorageKey+"\x00"+file.StoragePath] = obj.Path
	}
	return pathMap, warnings, nil
}

func remapStoragePath(pathMap map[string]string, storageKey, storagePath string) string {
	if storagePath == "" {
		return storagePath
	}
	if next, ok := pathMap[storageKey+"\x00"+storagePath]; ok {
		return next
	}
	return storagePath
}

func parseStorageURL(raw string) (string, string, bool) {
	const marker = "/api/v1/files/"
	idx := strings.Index(raw, marker)
	if idx < 0 {
		return "", "", false
	}
	rest := strings.TrimPrefix(raw[idx+len(marker):], "/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || !isSafeStoragePath(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}
