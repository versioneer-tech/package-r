// Package rclonefs adapts rclone's in-process VFS to afero.Fs.
package rclonefs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	rclone "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/spf13/afero"
)

// FS is an afero filesystem backed by an rclone VFS.
type FS struct {
	vfs       *vfs.VFS
	closeOnce sync.Once
}

var _ afero.Fs = (*FS)(nil)

// New creates an adapter with the write cache needed for seek and chunked uploads.
func New(ctx context.Context, remote rclone.Fs) (*FS, error) {
	var options vfscommon.Options
	if err := configstruct.Set(optionDefaults{options: vfscommon.OptionsInfo}, &options); err != nil {
		return nil, err
	}
	options.CacheMode = vfscommon.CacheModeWrites
	options.WriteBack = 0
	options.PollInterval = 0
	fileSystem := NewWithOptions(ctx, remote, &options)
	if fileSystem.vfs.Opt.CacheMode != vfscommon.CacheModeWrites {
		_ = fileSystem.Close()
		return nil, errors.New("rclone VFS write cache is unavailable")
	}
	return fileSystem, nil
}

// NewWithOptions creates an adapter with explicit VFS options.
func NewWithOptions(ctx context.Context, remote rclone.Fs, options *vfscommon.Options) *FS {
	return &FS{vfs: vfs.New(ctx, remote, options)}
}

// Close stops the VFS. Open files must be closed before this call.
func (f *FS) Close() error {
	f.closeOnce.Do(f.vfs.Shutdown)
	return nil
}

func (f *FS) Name() string {
	return "rclone"
}

// PublicLink creates a time-limited read URL through the backend used by the VFS.
func (f *FS) PublicLink(ctx context.Context, name string, expire time.Duration) (string, error) {
	name, err := cleanName(name)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", fs.ErrInvalid
	}
	publicLink := f.vfs.Fs().Features().PublicLink
	if publicLink == nil {
		return "", fmt.Errorf("%s does not support public links", f.vfs.Fs())
	}
	return publicLink(ctx, name, rclone.Duration(expire), false)
}

func (f *FS) Create(name string) (afero.File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	handle, err := f.vfs.Create(name)
	return wrapHandle(name, handle, err)
}

func (f *FS) Mkdir(name string, perm os.FileMode) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	return f.vfs.Mkdir(name, perm)
}

func (f *FS) MkdirAll(name string, perm os.FileMode) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	return f.vfs.MkdirAll(name, perm)
}

func (f *FS) Open(name string) (afero.File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	handle, err := f.vfs.Open(name)
	return wrapHandle(name, handle, err)
}

func (f *FS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	handle, err := f.vfs.OpenFile(name, flag, perm)
	return wrapHandle(name, handle, err)
}

func (f *FS) Remove(name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	return f.vfs.Remove(name)
}

func (f *FS) RemoveAll(name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	if name == "" {
		return os.ErrPermission
	}
	node, err := f.vfs.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return node.RemoveAll()
}

func (f *FS) Rename(oldName, newName string) error {
	oldName, err := cleanName(oldName)
	if err != nil {
		return err
	}
	newName, err = cleanName(newName)
	if err != nil {
		return err
	}
	return f.vfs.Rename(oldName, newName)
}

func (f *FS) Stat(name string) (os.FileInfo, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	return f.vfs.Stat(name)
}

// Chmod is a no-op because object stores do not expose POSIX modes.
func (f *FS) Chmod(name string, _ os.FileMode) error {
	_, err := f.Stat(name)
	return err
}

// Chown is a no-op because object stores do not expose POSIX ownership.
func (f *FS) Chown(name string, _, _ int) error {
	_, err := f.Stat(name)
	return err
}

func (f *FS) Chtimes(name string, atime, mtime time.Time) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	return f.vfs.Chtimes(name, atime, mtime)
}

func cleanName(name string) (string, error) {
	if strings.ContainsRune(name, 0) {
		return "", &os.PathError{Op: "path", Path: name, Err: fs.ErrInvalid}
	}

	name = filepath.ToSlash(name)
	name = strings.TrimLeft(name, "/")
	name = path.Clean(name)
	if name == "." {
		return "", nil
	}
	if name == ".." || strings.HasPrefix(name, "../") {
		return "", &os.PathError{Op: "path", Path: name, Err: fs.ErrPermission}
	}
	return name, nil
}

type file struct {
	vfs.Handle
	name string
}

var _ afero.File = (*file)(nil)

func (f *file) Name() string {
	return "/" + f.name
}

func wrapHandle(name string, handle vfs.Handle, err error) (afero.File, error) {
	if err != nil {
		return nil, err
	}
	return &file{Handle: handle, name: name}, nil
}
