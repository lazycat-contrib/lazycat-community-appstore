package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

const testMigrationFileLimit int64 = 1 << 20

func TestMigrationUploadProductionLimits(t *testing.T) {
	if maxMigrationUploadBytes != 512<<20 {
		t.Fatalf("migration file limit = %d, want 512 MiB", maxMigrationUploadBytes)
	}
	if maxMigrationTextBytes != 64<<10 {
		t.Fatalf("migration text limit = %d, want 64 KiB", maxMigrationTextBytes)
	}
}

func TestMigrationImportOptionsUseStreamedValues(t *testing.T) {
	options := migrationImportOptions(url.Values{
		"includeSite":    {"true"},
		"includePeople":  {"on"},
		"includeApps":    {"1"},
		"includeFiles":   {"yes"},
		"mode":           {"replace"},
		"confirmReplace": {"OVERWRITE"},
	}, 42)
	if !options.IncludeSite || !options.IncludePeople || !options.IncludeApps || !options.IncludeFiles {
		t.Fatalf("migration options = %+v", options)
	}
	if options.Mode != "replace" || options.ConfirmReplace != "OVERWRITE" || options.ActorUserID != 42 {
		t.Fatalf("migration import metadata = %+v", options)
	}
}

func TestReadMigrationUploadFileBoundariesAndCleanup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	for _, tt := range []struct {
		name       string
		fileBytes  int64
		wantOK     bool
		wantStatus int
	}{
		{name: "exact", fileBytes: testMigrationFileLimit, wantOK: true},
		{name: "overflow", fileBytes: testMigrationFileLimit + 1, wantStatus: http.StatusUnprocessableEntity},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := migrationMultipartBody(t, func(writer *multipart.Writer) {
				part, err := writer.CreateFormFile("file", "migration.zip")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.CopyN(part, zeroMigrationReader{}, tt.fileBytes); err != nil {
					t.Fatal(err)
				}
			})
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()
			upload, ok := readMigrationUploadWithLimits(rec, req, testMigrationFileLimit, maxMigrationTextBytes)
			if ok != tt.wantOK {
				t.Fatalf("readMigrationUploadWithLimits() ok = %v, status=%d body=%s", ok, rec.Code, rec.Body.String())
			}
			if !ok {
				if rec.Code != tt.wantStatus {
					t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
				}
				assertNoMigrationTempFiles(t, tmp)
				return
			}
			if upload.Size != tt.fileBytes {
				t.Fatalf("upload size = %d, want %d", upload.Size, tt.fileBytes)
			}
			if err := upload.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if err := upload.Cleanup(); err != nil {
				t.Fatalf("second Cleanup() error = %v", err)
			}
			assertNoMigrationTempFiles(t, tmp)
		})
	}
}

func TestReadMigrationUploadAcceptsMultipartOverheadWithinBudget(t *testing.T) {
	body, contentType := migrationMultipartBody(t, func(writer *multipart.Writer) {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="file"; filename="migration.zip"`)
		header.Set("Content-Type", "application/zip")
		header.Set("X-Padding", strings.Repeat("x", 60<<10))
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.CopyN(part, zeroMigrationReader{}, testMigrationFileLimit); err != nil {
			t.Fatal(err)
		}
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	upload, ok := readMigrationUploadWithLimits(httptest.NewRecorder(), req, testMigrationFileLimit, maxMigrationTextBytes)
	if !ok {
		t.Fatal("multipart overhead within the 64 KiB request allowance was rejected")
	}
	if err := upload.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestReadMigrationUploadRejectsInvalidPartShapes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		parts func(*multipart.Writer)
	}{
		{name: "missing file", parts: func(writer *multipart.Writer) {
			_ = writer.WriteField("mode", "merge")
		}},
		{name: "duplicate file", parts: func(writer *multipart.Writer) {
			writeMigrationTestFile(t, writer, "file", "one.zip", 1)
			writeMigrationTestFile(t, writer, "file", "two.zip", 1)
		}},
		{name: "second file field", parts: func(writer *multipart.Writer) {
			writeMigrationTestFile(t, writer, "file", "one.zip", 1)
			writeMigrationTestFile(t, writer, "attachment", "two.zip", 1)
		}},
		{name: "text overflow", parts: func(writer *multipart.Writer) {
			writeMigrationTestFile(t, writer, "file", "one.zip", 1)
			_ = writer.WriteField("one", strings.Repeat("a", 40<<10))
			_ = writer.WriteField("two", strings.Repeat("b", 25<<10))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			body, contentType := migrationMultipartBody(t, tt.parts)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()
			if upload, ok := readMigrationUploadWithLimits(rec, req, testMigrationFileLimit, maxMigrationTextBytes); ok || upload != nil {
				t.Fatal("invalid multipart upload was accepted")
			}
			assertNoMigrationTempFiles(t, tmp)
		})
	}
}

func TestMigrationExportFailureDoesNotCommitDownloadHeaders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/migration/export", strings.NewReader(`{"includeSite":true}`))
	rec := httptest.NewRecorder()
	(&Server{}).handleMigrationExport(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON error", got)
	}
	if rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("Content-Disposition committed on export failure: %q", rec.Header().Get("Content-Disposition"))
	}
	assertNoMigrationTempFiles(t, tmp)
}

type zeroMigrationReader struct{}

func (zeroMigrationReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func migrationMultipartBody(t *testing.T, write func(*multipart.Writer)) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	write(writer)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func writeMigrationTestFile(t *testing.T, writer *multipart.Writer, field, name string, size int64) {
	t.Helper()
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(part, zeroMigrationReader{}, size); err != nil {
		t.Fatal(err)
	}
}

func assertNoMigrationTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "appstore-migration-*.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("migration temp files remain: %v", matches)
	}
}
