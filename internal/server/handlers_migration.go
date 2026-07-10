package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	filePath, _, err := s.exportMigrationFile(r.Context(), options)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MIGRATION_EXPORT_FAILED", "Could not export migration package", nil)
		return
	}
	defer func() { _ = os.Remove(filePath) }()
	file, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MIGRATION_EXPORT_FAILED", "Could not open migration package", nil)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("close migration export: %v", err)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MIGRATION_EXPORT_FAILED", "Could not inspect migration package", nil)
		return
	}
	filename := fmt.Sprintf("lazycat-appstore-migration-%s.zip", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func (s *Server) handleMigrationImportPreview(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	upload, ok := readMigrationUpload(w, r)
	if !ok {
		return
	}
	defer func() { _ = upload.Cleanup() }()
	importer := migration.NewImporter(s.db, s.migrationStorageResolver())
	preview, err := importer.Preview(r.Context(), upload.File, upload.Size)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "MIGRATION_PREVIEW_FAILED", "Could not preview migration package", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preview": preview})
}

func (s *Server) handleMigrationImport(w http.ResponseWriter, r *http.Request, u *ent.User) {
	upload, ok := readMigrationUpload(w, r)
	if !ok {
		return
	}
	defer func() { _ = upload.Cleanup() }()
	options := migrationImportOptions(upload.Values, u.ID)
	importer := migration.NewImporter(s.db, s.migrationStorageResolver())
	result, err := importer.Import(r.Context(), upload.File, upload.Size, options)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "MIGRATION_IMPORT_FAILED", "Could not import migration package", nil)
		return
	}
	payload := map[string]any{"result": result}
	if options.Mode != migration.ImportModeReplace {
		writeJSON(w, http.StatusOK, payload)
		return
	}
	if err := writeMigrationResult(w, http.StatusOK, payload); err != nil {
		log.Printf("write migration response before restart: %v", err)
		return
	}
	s.requestRestart()
}

func writeMigrationResult(w http.ResponseWriter, status int, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	n, err := w.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	if err := http.NewResponseController(w).Flush(); err != nil {
		return err
	}
	return nil
}
