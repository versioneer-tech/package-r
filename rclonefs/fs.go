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
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/spf13/afero"

	appLogging "github.com/versioneer-tech/package-r/logging"
)

// FS is an afero filesystem backed by an rclone VFS.
type FS struct {
	vfs       *vfs.VFS
	closeOnce sync.Once
}

// Stats contains safe, aggregate VFS write-cache values.
type Stats struct {
	CacheBytes        int64
	ErroredFiles      int64
	UploadsInProgress int64
	UploadsQueued     int64
	OutOfSpace        bool
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

func (f *FS) stats() Stats {
	values := f.vfs.Stats()
	diskCache, ok := values["diskCache"].(rc.Params)
	if !ok {
		return Stats{}
	}

	return Stats{
		CacheBytes:        metricInt64(diskCache["bytesUsed"]),
		ErroredFiles:      metricInt64(diskCache["erroredFiles"]),
		UploadsInProgress: metricInt64(diskCache["uploadsInProgress"]),
		UploadsQueued:     metricInt64(diskCache["uploadsQueued"]),
		OutOfSpace:        metricBool(diskCache["outOfSpace"]),
	}
}

func metricInt64(value interface{}) int64 {
	switch value := value.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case uint64:
		return int64(value)
	default:
		return 0
	}
}

func metricBool(value interface{}) bool {
	result, _ := value.(bool)
	return result
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
	started := time.Now()
	link, err := publicLink(ctx, name, rclone.Duration(expire), false)
	appLogging.Infof(
		"rclone presign path=%q lifetime=%s duration=%s error=%v",
		name,
		expire,
		time.Since(started),
		err,
	)
	return link, err
}

func (f *FS) Create(name string) (afero.File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	handle, err := f.vfs.Create(name)
	appLogging.Infof("rclone create path=%q error=%v", name, err)
	return wrapHandle(name, handle, err, true)
}

func (f *FS) Mkdir(name string, perm os.FileMode) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	started := time.Now()
	err = f.vfs.Mkdir(name, perm)
	logChange("mkdir", name, time.Since(started), err)
	return err
}

func (f *FS) MkdirAll(name string, perm os.FileMode) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	started := time.Now()
	err = f.vfs.MkdirAll(name, perm)
	logChange("mkdir-all", name, time.Since(started), err)
	return err
}

func (f *FS) Open(name string) (afero.File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	handle, err := f.vfs.Open(name)
	logGet("open", name, time.Since(started), err)
	return wrapHandle(name, handle, err, false)
}

func (f *FS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	handle, err := f.vfs.OpenFile(name, flag, perm)
	elapsed := time.Since(started)
	change := opensForChange(flag)
	if change {
		appLogging.Infof(
			"rclone open-write path=%q flags=%d duration=%s error=%v",
			name,
			flag,
			elapsed,
			err,
		)
	} else {
		logGet("open-file", name, elapsed, err)
	}
	return wrapHandle(name, handle, err, change)
}

func (f *FS) Remove(name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	started := time.Now()
	err = f.vfs.Remove(name)
	logChange("remove", name, time.Since(started), err)
	return err
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
		logChange("remove-all", name, 0, nil)
		return nil
	}
	if err != nil {
		return err
	}
	started := time.Now()
	err = node.RemoveAll()
	logChange("remove-all", name, time.Since(started), err)
	return err
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
	started := time.Now()
	err = f.vfs.Rename(oldName, newName)
	appLogging.Infof(
		"rclone rename source=%q destination=%q duration=%s error=%v",
		oldName,
		newName,
		time.Since(started),
		err,
	)
	return err
}

func (f *FS) Stat(name string) (os.FileInfo, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	info, err := f.vfs.Stat(name)
	logGet("stat", name, time.Since(started), err)
	return info, err
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
	started := time.Now()
	err = f.vfs.Chtimes(name, atime, mtime)
	logChange("change-times", name, time.Since(started), err)
	return err
}

func opensForChange(flag int) bool {
	return flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0
}

func logGet(operation, name string, elapsed time.Duration, err error) {
	appLogging.Debugf(
		"rclone get operation=%s path=%q duration=%s error=%v",
		operation,
		name,
		elapsed,
		err,
	)
}

func logChange(operation, name string, elapsed time.Duration, err error) {
	appLogging.Infof(
		"rclone change operation=%s path=%q duration=%s error=%v",
		operation,
		name,
		elapsed,
		err,
	)
}

func logList(name string, entries int, elapsed time.Duration, err error) {
	format := "rclone list path=%q entries=%d duration=%s error=%v"
	appLogging.Timedf(elapsed, format, name, entries, elapsed, err)
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
	name   string
	change bool
}

var _ afero.File = (*file)(nil)

func (f *file) Name() string {
	return "/" + f.name
}

func (f *file) Readdir(count int) ([]os.FileInfo, error) {
	started := time.Now()
	entries, err := f.Handle.Readdir(count)
	logList(f.name, len(entries), time.Since(started), err)
	return entries, err
}

func (f *file) Close() error {
	started := time.Now()
	err := f.Handle.Close()
	if f.change {
		logChange("close-write", f.name, time.Since(started), err)
	}
	return err
}

func wrapHandle(name string, handle vfs.Handle, err error, change bool) (afero.File, error) {
	if err != nil {
		return nil, err
	}
	return &file{Handle: handle, name: name, change: change}, nil
}
