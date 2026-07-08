package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/internal/migration"
	"lazycat.community/appstore/internal/storage"
)

const maxMigrationUploadBytes = 512 << 20

func (s *Server) migrationStorageResolver() migration.StorageResolver {
	return migration.StorageResolverFunc(func(ctx context.Context, key string) (storage.Backend, error) {
		return s.storageBackendForKey(ctx, key)
	})
}

func (s *Server) handleMigrationExport(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	var options migration.Options
	if err := decodeJSON(r, &options); err != nil {
		badRequest(w, err)
		return
	}
	exporter := migration.NewExporter(s.db, s.migrationStorageResolver(), appVersion())
	var buf bytes.Buffer
	if _, err := exporter.Export(r.Context(), &buf, options); err != nil {
		writeError(w, http.StatusInternalServerError, "MIGRATION_EXPORT_FAILED", "Could not export migration package", nil)
		return
	}
	filename := fmt.Sprintf("lazycat-appstore-migration-%s.zip", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleMigrationImportPreview(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	data, size, ok := readMigrationUpload(w, r)
	if !ok {
		return
	}
	importer := migration.NewImporter(s.db, s.migrationStorageResolver())
	preview, err := importer.Preview(r.Context(), bytes.NewReader(data), size)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "MIGRATION_PREVIEW_FAILED", "Could not preview migration package", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preview": preview})
}

func (s *Server) handleMigrationImport(w http.ResponseWriter, r *http.Request, u *ent.User) {
	data, size, ok := readMigrationUpload(w, r)
	if !ok {
		return
	}
	options := migration.ImportOptions{
		Options: migration.Options{
			IncludeSite:   formBool(r, "includeSite"),
			IncludePeople: formBool(r, "includePeople"),
			IncludeApps:   formBool(r, "includeApps"),
			IncludeFiles:  formBool(r, "includeFiles"),
		},
		Mode:           migration.ImportMode(r.FormValue("mode")),
		ConfirmReplace: r.FormValue("confirmReplace"),
		ActorUserID:    u.ID,
	}
	importer := migration.NewImporter(s.db, s.migrationStorageResolver())
	result, err := importer.Import(r.Context(), bytes.NewReader(data), size, options)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "MIGRATION_IMPORT_FAILED", "Could not import migration package", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
	if options.Mode == migration.ImportModeReplace {
		s.scheduleRestartAfterImport()
	}
}

func (s *Server) scheduleRestartAfterImport() {
	if s.restartAfterImport == nil {
		return
	}
	s.restartAfterImportOnce.Do(func() {
		go func() {
			time.Sleep(750 * time.Millisecond)
			log.Print("migration overwrite import completed; restarting server")
			s.restartAfterImport()
		}()
	})
}

func readMigrationUpload(w http.ResponseWriter, r *http.Request) ([]byte, int64, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMigrationUploadBytes)
	if err := r.ParseMultipartForm(maxMigrationUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Migration package upload is invalid", nil)
		return nil, 0, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Migration package file is required", nil)
		return nil, 0, false
	}
	defer file.Close()
	if header.Size > maxMigrationUploadBytes {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Migration package is too large", nil)
		return nil, 0, false
	}
	data, err := io.ReadAll(io.LimitReader(file, maxMigrationUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Could not read migration package", nil)
		return nil, 0, false
	}
	if len(data) > maxMigrationUploadBytes {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Migration package is too large", nil)
		return nil, 0, false
	}
	return data, int64(len(data)), true
}
