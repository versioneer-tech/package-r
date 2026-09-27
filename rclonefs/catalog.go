package rclonefs

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// ErrCrossBucketRename reports a rename between two configured buckets.
var ErrCrossBucketRename = errors.New("cross-bucket rename is not supported")

// bucketCatalogFS exposes configured bucket filesystems below a synthetic root.
type bucketCatalogFS struct {
	root    afero.Fs
	buckets map[string]afero.Fs
}

var _ afero.Fs = (*bucketCatalogFS)(nil)

func newBucketCatalogFS(buckets map[string]afero.Fs) (*bucketCatalogFS, error) {
	root := afero.NewMemMapFs()
	for bucket := range buckets {
		if err := root.Mkdir("/"+bucket, 0o555); err != nil {
			return nil, err
		}
	}
	return &bucketCatalogFS{root: root, buckets: buckets}, nil
}

func (f *bucketCatalogFS) Name() string { return "configured S3 buckets" }

func (f *bucketCatalogFS) Create(name string) (afero.File, error) {
	backend, child, exact, err := f.resolve("create", name)
	if err != nil {
		return nil, err
	}
	if exact {
		return nil, pathError("create", name, fs.ErrPermission)
	}
	return backend.Create(child)
}

func (f *bucketCatalogFS) Mkdir(name string, perm os.FileMode) error {
	backend, child, exact, err := f.resolve("mkdir", name)
	if err != nil {
		return err
	}
	if exact {
		return pathError("mkdir", name, fs.ErrPermission)
	}
	return backend.Mkdir(child, perm)
}

func (f *bucketCatalogFS) MkdirAll(name string, perm os.FileMode) error {
	backend, child, exact, err := f.resolve("mkdir", name)
	if err != nil {
		return err
	}
	if exact {
		return nil
	}
	return backend.MkdirAll(child, perm)
}

func (f *bucketCatalogFS) Open(name string) (afero.File, error) {
	cleaned, err := cleanCatalogPath(name)
	if err != nil {
		return nil, err
	}
	if cleaned == "/" {
		return f.root.Open("/")
	}
	backend, child, _, err := f.resolve("open", name)
	if err != nil {
		return nil, err
	}
	return backend.Open(child)
}

func (f *bucketCatalogFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	cleaned, err := cleanCatalogPath(name)
	if err != nil {
		return nil, err
	}
	if cleaned == "/" {
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
			return nil, pathError("open", name, fs.ErrPermission)
		}
		return f.root.OpenFile("/", flag, perm)
	}
	backend, child, exact, err := f.resolve("open", name)
	if err != nil {
		return nil, err
	}
	if exact && flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		return nil, pathError("open", name, fs.ErrPermission)
	}
	return backend.OpenFile(child, flag, perm)
}

func (f *bucketCatalogFS) Remove(name string) error {
	backend, child, exact, err := f.resolve("remove", name)
	if err != nil {
		return err
	}
	if exact {
		return pathError("remove", name, fs.ErrPermission)
	}
	return backend.Remove(child)
}

func (f *bucketCatalogFS) RemoveAll(name string) error {
	backend, child, exact, err := f.resolve("remove", name)
	if err != nil {
		return err
	}
	if exact {
		return pathError("remove", name, fs.ErrPermission)
	}
	return backend.RemoveAll(child)
}

func (f *bucketCatalogFS) Rename(oldName, newName string) error {
	oldBackend, oldChild, oldExact, err := f.resolve("rename", oldName)
	if err != nil {
		return err
	}
	newBackend, newChild, newExact, err := f.resolve("rename", newName)
	if err != nil {
		return err
	}
	if oldExact || newExact {
		return &os.LinkError{Op: "rename", Old: oldName, New: newName, Err: fs.ErrPermission}
	}
	if oldBackend != newBackend {
		return &os.LinkError{Op: "rename", Old: oldName, New: newName, Err: ErrCrossBucketRename}
	}
	return oldBackend.Rename(oldChild, newChild)
}

func (f *bucketCatalogFS) Stat(name string) (os.FileInfo, error) {
	cleaned, err := cleanCatalogPath(name)
	if err != nil {
		return nil, err
	}
	if cleaned == "/" {
		return f.root.Stat("/")
	}
	backend, child, exact, err := f.resolve("stat", name)
	if err != nil {
		return nil, err
	}
	if exact {
		return f.root.Stat(cleaned)
	}
	return backend.Stat(child)
}

func (f *bucketCatalogFS) Chmod(name string, mode os.FileMode) error {
	backend, child, exact, err := f.resolve("chmod", name)
	if err != nil {
		return err
	}
	if exact {
		return pathError("chmod", name, fs.ErrPermission)
	}
	return backend.Chmod(child, mode)
}

func (f *bucketCatalogFS) Chown(name string, uid, gid int) error {
	backend, child, exact, err := f.resolve("chown", name)
	if err != nil {
		return err
	}
	if exact {
		return pathError("chown", name, fs.ErrPermission)
	}
	return backend.Chown(child, uid, gid)
}

func (f *bucketCatalogFS) Chtimes(name string, atime, mtime time.Time) error {
	backend, child, exact, err := f.resolve("chtimes", name)
	if err != nil {
		return err
	}
	if exact {
		return pathError("chtimes", name, fs.ErrPermission)
	}
	return backend.Chtimes(child, atime, mtime)
}

func (f *bucketCatalogFS) resolve(op, name string) (afero.Fs, string, bool, error) {
	cleaned, err := cleanCatalogPath(name)
	if err != nil {
		return nil, "", false, err
	}
	if cleaned == "/" {
		return nil, "", false, pathError(op, name, fs.ErrPermission)
	}
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	backend, ok := f.buckets[parts[0]]
	if !ok {
		return nil, "", false, pathError(op, name, fs.ErrNotExist)
	}
	if len(parts) == 1 {
		return backend, "/", true, nil
	}
	return backend, "/" + strings.Join(parts[1:], "/"), false, nil
}

func cleanCatalogPath(name string) (string, error) {
	cleaned, err := cleanName(name)
	if err != nil {
		return "", err
	}
	if cleaned == "" {
		return "/", nil
	}
	return "/" + cleaned, nil
}

func pathError(op, name string, err error) error {
	return &os.PathError{Op: op, Path: name, Err: err}
}
