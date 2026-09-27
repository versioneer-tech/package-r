package rclonefs

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/spf13/afero"
)

func TestBucketCatalogRoutesObjectsAndProtectsRoots(t *testing.T) {
	data := afero.NewMemMapFs()
	archive := afero.NewMemMapFs()
	if err := afero.WriteFile(data, "/report.txt", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(archive, "/old.txt", []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := newBucketCatalogFS(map[string]afero.Fs{
		"xyz-data":    data,
		"xyz-archive": archive,
	})
	if err != nil {
		t.Fatal(err)
	}

	contents, err := afero.ReadFile(catalog, "/xyz-data/report.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "data" {
		t.Fatalf("unexpected object contents %q", contents)
	}
	if err := afero.WriteFile(catalog, "/xyz-archive/new.txt", []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if exists, err := afero.Exists(archive, "/new.txt"); err != nil || !exists {
		t.Fatalf("write did not reach selected bucket: exists=%t err=%v", exists, err)
	}
	if _, err := catalog.Open("/xyz-missing/report.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected missing bucket error, got %v", err)
	}
	if _, err := catalog.Open("/xyz-data/report\x00.txt"); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("expected invalid path error, got %v", err)
	}
	if err := catalog.Remove("/xyz-data"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected bucket root removal to be denied, got %v", err)
	}
	if err := catalog.Rename("/xyz-data/report.txt", "/xyz-archive/report.txt"); !errors.Is(err, ErrCrossBucketRename) {
		t.Fatalf("expected cross-bucket rename error, got %v", err)
	}
}
