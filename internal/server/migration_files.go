package server

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"lazycat.community/appstore/internal/migration"
)

const maxMigrationTextBytes int64 = 64 << 10

type migrationUpload struct {
	File        *os.File
	Size        int64
	Values      url.Values
	cleanupOnce sync.Once
	cleanupErr  error
}

func (u *migrationUpload) Cleanup() error {
	if u == nil || u.File == nil {
		return nil
	}
	u.cleanupOnce.Do(func() {
		name := u.File.Name()
		closeErr := u.File.Close()
		removeErr := os.Remove(name)
		if errors.Is(removeErr, fs.ErrNotExist) {
			removeErr = nil
		}
		u.cleanupErr = errors.Join(closeErr, removeErr)
	})
	return u.cleanupErr
}

func readMigrationUpload(w http.ResponseWriter, r *http.Request) (*migrationUpload, bool) {
	return readMigrationUploadWithLimits(w, r, maxMigrationUploadBytes, maxMigrationTextBytes)
}

func readMigrationUploadWithLimits(w http.ResponseWriter, r *http.Request, maxFileBytes, maxTextBytes int64) (*migrationUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes+maxTextBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Migration package upload is invalid", nil)
		return nil, false
	}
	upload := &migrationUpload{Values: make(url.Values)}
	fail := func(status int, message string, err error) (*migrationUpload, bool) {
		_ = upload.Cleanup()
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			status = http.StatusRequestEntityTooLarge
			message = "Migration package upload is too large"
		}
		writeError(w, status, "VALIDATION_ERROR", message, nil)
		return nil, false
	}
	var textBytes int64
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(http.StatusBadRequest, "Migration package upload is invalid", err)
		}
		formName := part.FormName()
		filename := part.FileName()
		if filename != "" {
			if formName != "file" || upload.File != nil {
				closeErr := part.Close()
				return fail(http.StatusBadRequest, "Migration package must contain exactly one file", closeErr)
			}
			file, err := os.CreateTemp("", "appstore-migration-upload-*.zip")
			if err != nil {
				_ = part.Close()
				return fail(http.StatusInternalServerError, "Could not store migration package", err)
			}
			upload.File = file
			limited := &io.LimitedReader{R: part, N: maxFileBytes + 1}
			n, copyErr := io.Copy(file, limited)
			closeErr := part.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return fail(http.StatusBadRequest, "Could not read migration package", err)
			}
			if n > maxFileBytes {
				return fail(http.StatusUnprocessableEntity, "Migration package is too large", nil)
			}
			upload.Size = n
			continue
		}
		if formName == "file" {
			closeErr := part.Close()
			return fail(http.StatusBadRequest, "Migration package file is required", closeErr)
		}
		remaining := maxTextBytes - textBytes
		raw, readErr := io.ReadAll(&io.LimitedReader{R: part, N: remaining + 1})
		closeErr := part.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return fail(http.StatusBadRequest, "Migration package upload is invalid", err)
		}
		textBytes += int64(len(raw))
		if textBytes > maxTextBytes {
			return fail(http.StatusUnprocessableEntity, "Migration import options are too large", nil)
		}
		upload.Values.Add(formName, string(raw))
	}
	if upload.File == nil {
		return fail(http.StatusBadRequest, "Migration package file is required", nil)
	}
	if _, err := upload.File.Seek(0, io.SeekStart); err != nil {
		return fail(http.StatusInternalServerError, "Could not prepare migration package", err)
	}
	return upload, true
}

func migrationImportOptions(values url.Values, actorUserID int) migration.ImportOptions {
	return migration.ImportOptions{
		Options: migration.Options{
			IncludeSite:   valueBool(values.Get("includeSite")),
			IncludePeople: valueBool(values.Get("includePeople")),
			IncludeApps:   valueBool(values.Get("includeApps")),
			IncludeFiles:  valueBool(values.Get("includeFiles")),
		},
		Mode:           migration.ImportMode(values.Get("mode")),
		ConfirmReplace: values.Get("confirmReplace"),
		ActorUserID:    actorUserID,
	}
}

func valueBool(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func (s *Server) exportMigrationFile(ctx context.Context, options migration.Options) (string, *migration.Manifest, error) {
	file, err := os.CreateTemp("", "appstore-migration-export-*.zip")
	if err != nil {
		return "", nil, err
	}
	filePath := file.Name()
	exporter := migration.NewExporter(s.db, s.migrationStorageResolver(), appVersion())
	manifest, exportErr := exporter.Export(ctx, file, options)
	closeErr := file.Close()
	if err := errors.Join(exportErr, closeErr); err != nil {
		removeErr := os.Remove(filePath)
		if errors.Is(removeErr, fs.ErrNotExist) {
			removeErr = nil
		}
		return "", nil, errors.Join(err, removeErr)
	}
	return filePath, manifest, nil
}
