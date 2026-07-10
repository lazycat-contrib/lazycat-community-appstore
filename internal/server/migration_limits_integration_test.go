package server

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMigrationPreviewHandlerEnforcesActual512MiBBoundary(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	t.Run("exact file reaches preview", func(t *testing.T) {
		rec, writerErr := streamMigrationPreviewRequest(t, maxMigrationUploadBytes)
		if writerErr != nil {
			t.Fatalf("multipart writer error = %v", writerErr)
		}
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "MIGRATION_PREVIEW_FAILED") {
			t.Fatalf("exact-limit response code=%d body=%s", rec.Code, rec.Body.String())
		}
		assertNoMigrationTempFiles(t, tmp)
	})

	t.Run("one byte overflow is rejected before preview", func(t *testing.T) {
		rec, writerErr := streamMigrationPreviewRequest(t, maxMigrationUploadBytes+1)
		if writerErr != nil && !errors.Is(writerErr, io.ErrClosedPipe) {
			t.Fatalf("multipart writer error = %v", writerErr)
		}
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") || !strings.Contains(rec.Body.String(), "too large") {
			t.Fatalf("overflow response code=%d body=%s", rec.Code, rec.Body.String())
		}
		assertNoMigrationTempFiles(t, tmp)
	})
}

func streamMigrationPreviewRequest(t *testing.T, fileBytes int64) (*httptest.ResponseRecorder, error) {
	t.Helper()
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()
	writerDone := make(chan error, 1)
	go func() {
		part, err := multipartWriter.CreateFormFile("file", "migration.zip")
		if err == nil {
			_, err = io.CopyN(part, zeroMigrationReader{}, fileBytes)
		}
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		writerDone <- err
	}()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/migration/import/preview", reader)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	(&Server{}).handleMigrationImportPreview(rec, req, nil)
	_ = req.Body.Close()
	return rec, <-writerDone
}
