package server

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/migration"
)

type failingMigrationResponseWriter struct {
	header http.Header
}

func (w *failingMigrationResponseWriter) Header() http.Header {
	return w.header
}

func (*failingMigrationResponseWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*failingMigrationResponseWriter) WriteHeader(int) {}

func TestReplaceMigrationRestartsOnlyAfterSuccessfulWriteAndFlush(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		req, admin := replaceMigrationRequest(t, app.server)
		rec := httptest.NewRecorder()
		app.server.handleMigrationImport(rec, req, admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if !rec.Flushed {
			t.Fatal("replace migration response was not flushed")
		}
		select {
		case <-app.server.RestartRequested():
		default:
			t.Fatal("restart was not requested after a successful response flush")
		}
	})

	t.Run("write failure", func(t *testing.T) {
		app := newTestApp(t)
		req, admin := replaceMigrationRequest(t, app.server)
		writer := &failingMigrationResponseWriter{header: make(http.Header)}
		app.server.handleMigrationImport(writer, req, admin)
		select {
		case <-app.server.RestartRequested():
			t.Fatal("restart was requested after response write failure")
		default:
		}
	})

	t.Run("flush unsupported", func(t *testing.T) {
		app := newTestApp(t)
		req, admin := replaceMigrationRequest(t, app.server)
		writer := &nonFlushingMigrationResponseWriter{header: make(http.Header)}
		app.server.handleMigrationImport(writer, req, admin)
		if writer.body.Len() == 0 {
			t.Fatal("migration result was not written before flush check")
		}
		select {
		case <-app.server.RestartRequested():
			t.Fatal("restart was requested when response flushing is unsupported")
		default:
		}
	})
}

func TestWriteMigrationResultPropagatesFlushFailure(t *testing.T) {
	writer := &flushFailingResponseWriter{header: make(http.Header)}
	err := writeMigrationResult(writer, http.StatusOK, map[string]bool{"ok": true})
	if !errors.Is(err, errForcedFlush) {
		t.Fatalf("writeMigrationResult() error = %v, want flush failure", err)
	}
}

func TestWriteMigrationResultRejectsShortWrite(t *testing.T) {
	writer := &shortMigrationResponseWriter{header: make(http.Header)}
	err := writeMigrationResult(writer, http.StatusOK, map[string]bool{"ok": true})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("writeMigrationResult() error = %v, want short write", err)
	}
}

func TestWriteMigrationResultRequiresFlushSupport(t *testing.T) {
	writer := &nonFlushingMigrationResponseWriter{header: make(http.Header)}
	err := writeMigrationResult(writer, http.StatusOK, map[string]bool{"ok": true})
	if !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("writeMigrationResult() error = %v, want ErrNotSupported", err)
	}
}

var errForcedFlush = errors.New("forced flush failure")

type flushFailingResponseWriter struct {
	header http.Header
}

func (w *flushFailingResponseWriter) Header() http.Header { return w.header }
func (*flushFailingResponseWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
func (*flushFailingResponseWriter) WriteHeader(int)   {}
func (*flushFailingResponseWriter) FlushError() error { return errForcedFlush }

type shortMigrationResponseWriter struct {
	header http.Header
}

func (w *shortMigrationResponseWriter) Header() http.Header { return w.header }
func (*shortMigrationResponseWriter) Write(p []byte) (int, error) {
	return len(p) / 2, nil
}
func (*shortMigrationResponseWriter) WriteHeader(int) {}

type nonFlushingMigrationResponseWriter struct {
	header http.Header
	body   bytes.Buffer
}

func (w *nonFlushingMigrationResponseWriter) Header() http.Header { return w.header }
func (w *nonFlushingMigrationResponseWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}
func (*nonFlushingMigrationResponseWriter) WriteHeader(int) {}

func replaceMigrationRequest(t *testing.T, srv *Server) (*http.Request, *ent.User) {
	t.Helper()
	path, _, err := srv.exportMigrationFile(t.Context(), migration.Options{IncludeSite: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(path) }()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body, contentType := migrationMultipartBody(t, func(writer *multipart.Writer) {
		_ = writer.WriteField("includeSite", "true")
		_ = writer.WriteField("mode", string(migration.ImportModeReplace))
		_ = writer.WriteField("confirmReplace", "OVERWRITE")
		part, createErr := writer.CreateFormFile("file", "migration.zip")
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := part.Write(raw); writeErr != nil {
			t.Fatal(writeErr)
		}
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/migration/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	admin := srv.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(t.Context())
	return req, admin
}
