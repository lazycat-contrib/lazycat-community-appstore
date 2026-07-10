package migration

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"lazycat.community/appstore/internal/storage"
)

func writeFileEntries(ctx context.Context, zw *zip.Writer, resolver StorageResolver, refs []fileRef) ([]FileManifest, []string, error) {
	if resolver == nil {
		return nil, []string{"attachment files were skipped because storage is not configured"}, nil
	}
	files := make([]FileManifest, 0, len(refs))
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
		if reader.Size > maxJSONEntryBytes {
			if err := reader.Body.Close(); err != nil {
				return nil, nil, fmt.Errorf("close %s attachment: %w", ref.Kind, err)
			}
			warnings = append(warnings, fmt.Sprintf("skipped %s attachment: file is too large", ref.Kind))
			continue
		}
		zipPath := "files/" + path.Clean(ref.StorageKey+"/"+ref.StoragePath)
		if !isSafeZipPath(zipPath) {
			if err := reader.Body.Close(); err != nil {
				return nil, nil, fmt.Errorf("close %s attachment: %w", ref.Kind, err)
			}
			warnings = append(warnings, fmt.Sprintf("skipped unsafe attachment path for %s", ref.Kind))
			continue
		}
		entry, err := zw.CreateHeader(&zip.FileHeader{Name: zipPath, Method: zip.Deflate})
		if err != nil {
			_ = reader.Body.Close()
			return nil, nil, err
		}
		hasher := sha256.New()
		limited := &io.LimitedReader{R: reader.Body, N: maxJSONEntryBytes + 1}
		n, copyErr := io.CopyBuffer(io.MultiWriter(entry, hasher), &contextReader{ctx: ctx, reader: limited}, make([]byte, 64<<10))
		closeErr := reader.Body.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return nil, nil, fmt.Errorf("stream %s attachment: %w", ref.Kind, err)
		}
		if n > maxJSONEntryBytes {
			return nil, nil, fmt.Errorf("stream %s attachment: file is too large", ref.Kind)
		}
		files = append(files, FileManifest{
			Path:        zipPath,
			StorageKey:  ref.StorageKey,
			StoragePath: ref.StoragePath,
			Size:        n,
			SHA256:      hex.EncodeToString(hasher.Sum(nil)),
		})
	}
	return files, warnings, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
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
		var entry *zip.File
		for _, candidate := range zr.File {
			if candidate.Name != file.Path {
				continue
			}
			if entry != nil || candidate.FileInfo().IsDir() {
				return nil, nil, fmt.Errorf("read attachment: invalid attachment entry")
			}
			entry = candidate
		}
		if entry == nil {
			return nil, nil, fmt.Errorf("read attachment: zip entry not found")
		}
		backend, err := resolver.BackendForKey(ctx, file.StorageKey)
		if err != nil {
			return nil, nil, fmt.Errorf("load attachment storage: %w", err)
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, nil, fmt.Errorf("read attachment: %w", err)
		}
		obj, saveErr := storage.SaveFile(ctx, backend, rc, path.Base(file.StoragePath), maxJSONEntryBytes)
		closeErr := rc.Close()
		if err := errors.Join(saveErr, closeErr); err != nil {
			var deleteErr error
			if obj.Path != "" {
				deleteErr = deleteAttachment(ctx, backend, obj.Path)
			}
			return nil, nil, errors.Join(fmt.Errorf("save attachment: %w", err), deleteErr)
		}
		if obj.Size != file.Size || !strings.EqualFold(obj.SHA256, file.SHA256) {
			deleteErr := deleteAttachment(ctx, backend, obj.Path)
			if obj.Size != file.Size {
				return nil, nil, errors.Join(errors.New("attachment size mismatch"), deleteErr)
			}
			return nil, nil, errors.Join(errors.New("attachment hash mismatch"), deleteErr)
		}
		pathMap[file.StorageKey+"\x00"+file.StoragePath] = obj.Path
	}
	return pathMap, warnings, nil
}

func deleteAttachment(ctx context.Context, backend storage.Backend, objectPath string) error {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cleanupCancel()
	err := backend.Delete(cleanupCtx, objectPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
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
