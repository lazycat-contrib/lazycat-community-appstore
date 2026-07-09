package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLPKStoresFileAndComputesHash(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(root, "/files/")
	content := []byte("lpk content")

	obj, err := SaveLPK(context.Background(), backend, bytes.NewReader(content), "demo.lpk", 1024)
	if err != nil {
		t.Fatalf("SaveLPK returned error: %v", err)
	}

	wantHash := sha256.Sum256(content)
	if obj.SHA256 != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("SHA256 = %q, want %q", obj.SHA256, hex.EncodeToString(wantHash[:]))
	}
	if obj.Size != int64(len(content)) {
		t.Fatalf("Size = %d, want %d", obj.Size, len(content))
	}
	if !strings.HasSuffix(obj.Path, ".lpk") {
		t.Fatalf("Path = %q, want .lpk suffix", obj.Path)
	}
}

func TestSaveLPKRejectsWrongExtension(t *testing.T) {
	backend := NewLocalBackend(t.TempDir(), "/files/")

	_, err := SaveLPK(context.Background(), backend, strings.NewReader("data"), "demo.zip", 1024)
	if err == nil {
		t.Fatal("SaveLPK accepted a non-LPK file")
	}
}

func TestSaveLPKRejectsOversizedFile(t *testing.T) {
	backend := NewLocalBackend(t.TempDir(), "/files/")

	_, err := SaveLPK(context.Background(), backend, strings.NewReader("123456"), "demo.lpk", 3)
	if err == nil {
		t.Fatal("SaveLPK accepted an oversized file")
	}
}

func TestLocalDeleteRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	backend := NewLocalBackend(root, "/files/")

	err := backend.Delete(context.Background(), filepath.Join("..", filepath.Base(filepath.Dir(outside)), filepath.Base(outside)))
	if err == nil {
		t.Fatal("Delete accepted a traversal path")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("outside file was touched: %v", statErr)
	}
}

func TestLocalDeleteRemovesStoredPath(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(root, "/files/")
	obj, err := SaveLPK(context.Background(), backend, strings.NewReader("data"), "demo.lpk", 1024)
	if err != nil {
		t.Fatalf("SaveLPK returned error: %v", err)
	}

	if err := backend.Delete(context.Background(), obj.Path); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(obj.Path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stored file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestCleanObjectPathRejectsUnsafePaths(t *testing.T) {
	for _, input := range []string{"", "/backup.zip", "../backup.zip", "backups/../backup.zip", "backups//backup.zip", "backups/./backup.zip"} {
		t.Run(input, func(t *testing.T) {
			if _, err := CleanObjectPath(input); err == nil {
				t.Fatalf("CleanObjectPath(%q) accepted unsafe path", input)
			}
		})
	}
}

func TestLocalSaveObjectWritesRequestedRelativePath(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(root, "/files/")

	obj, err := backend.SaveObject(context.Background(), "backups/appstore/snapshot.zip", strings.NewReader("zip"))
	if err != nil {
		t.Fatalf("SaveObject returned error: %v", err)
	}
	if obj.Path != "backups/appstore/snapshot.zip" {
		t.Fatalf("Path = %q, want requested path", obj.Path)
	}
	data, err := os.ReadFile(filepath.Join(root, "backups", "appstore", "snapshot.zip"))
	if err != nil {
		t.Fatalf("read stored object: %v", err)
	}
	if string(data) != "zip" {
		t.Fatalf("stored content = %q, want zip", string(data))
	}
}

func TestLocalListObjectsReturnsRelativeObjects(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(root, "/files/")
	for _, objectPath := range []string{
		"backups/appstore/a.zip",
		"backups/appstore/nested/b.zip",
		"other/c.zip",
	} {
		if _, err := backend.SaveObject(context.Background(), objectPath, strings.NewReader("zip")); err != nil {
			t.Fatalf("SaveObject(%q) returned error: %v", objectPath, err)
		}
	}

	objects, err := backend.ListObjects(context.Background(), "backups/appstore")
	if err != nil {
		t.Fatalf("ListObjects returned error: %v", err)
	}
	paths := map[string]bool{}
	for _, object := range objects {
		paths[object.Path] = true
	}
	for _, want := range []string{"backups/appstore/a.zip", "backups/appstore/nested/b.zip"} {
		if !paths[want] {
			t.Fatalf("ListObjects missing %q: %+v", want, objects)
		}
	}
	if paths["other/c.zip"] {
		t.Fatalf("ListObjects included object outside prefix: %+v", objects)
	}

	objects, err = backend.ListObjects(context.Background(), "missing")
	if err != nil {
		t.Fatalf("ListObjects missing prefix returned error: %v", err)
	}
	if len(objects) != 0 {
		t.Fatalf("ListObjects missing prefix returned %d objects, want 0", len(objects))
	}
}
