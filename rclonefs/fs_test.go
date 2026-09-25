package rclonefs_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/rclonefs"
)

var remoteID atomic.Uint64

func TestAferoAdapterBrowseReadAndWrite(t *testing.T) {
	base := newMemoryFS(t)
	userFS := afero.NewBasePathFs(base, "/team/alice")

	if err := userFS.MkdirAll("/reports/2026", 0755); err != nil {
		t.Fatal(err)
	}
	file, err := userFS.OpenFile("/reports/2026/data.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "object data"); err != nil {
		t.Fatal(err)
	}
	if info, err := file.Stat(); err != nil || info.Size() != int64(len("object data")) {
		t.Fatalf("unexpected open-file stat: info=%v err=%v", info, err)
	}
	if got := file.Name(); got != "/reports/2026/data.txt" {
		t.Fatalf("unexpected scoped file name %q", got)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := afero.ReadFile(userFS, "/reports/2026/data.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "object data" {
		t.Fatalf("unexpected data %q", data)
	}

	opened, err := userFS.Open("/reports/2026/data.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Seek(7, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	tail, err := io.ReadAll(opened)
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if string(tail) != "data" {
		t.Fatalf("unexpected seek result %q", tail)
	}

	entries, err := afero.ReadDir(userFS, "/reports/2026")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "data.txt" || entries[0].IsDir() {
		t.Fatalf("unexpected directory entries %#v", entries)
	}

	if err := userFS.Chmod("/reports/2026/data.txt", 0600); err != nil {
		t.Fatalf("chmod compatibility no-op failed: %v", err)
	}
}

func TestAferoAdapterMutationsAndErrors(t *testing.T) {
	base := newMemoryFS(t)

	if err := afero.WriteFile(base, "/source.txt", []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := base.Rename("/source.txt", "/renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Stat("/source.txt"); !errors.Is(err, fs.ErrNotExist) || !os.IsNotExist(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
	if err := base.MkdirAll("/tree/nested", 0755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(base, "/tree/nested/file.txt", []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := base.RemoveAll("/tree"); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Stat("/tree"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected removed tree to be absent, got %v", err)
	}
	if err := base.RemoveAll("/missing"); err != nil {
		t.Fatalf("RemoveAll must ignore a missing path: %v", err)
	}
	if err := base.RemoveAll("/"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected root removal to be denied, got %v", err)
	}
}

func newMemoryFS(t *testing.T) *rclonefs.FS {
	t.Helper()
	name := "my-" + strconv.FormatUint(remoteID.Add(1), 10)
	remote, err := memory.NewFs(context.Background(), name, "", configmap.Simple{})
	if err != nil {
		t.Fatal(err)
	}
	fileSystem, err := rclonefs.New(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fileSystem.Close(); err != nil {
			t.Error(err)
		}
	})
	return fileSystem
}
