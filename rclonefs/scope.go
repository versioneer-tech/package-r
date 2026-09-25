package rclonefs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// scopeFS closes a path-boundary gap in BasePathFs by checking the resolved
// path again before it reaches the shared object filesystem.
type scopeFS struct {
	base afero.Fs
	root string
}

var _ afero.Fs = (*scopeFS)(nil)

func newScopeFS(base afero.Fs, root string) afero.Fs {
	return &scopeFS{base: base, root: filepath.Clean(filepath.Join("/", root))}
}

func (s *scopeFS) Name() string {
	return s.base.Name()
}

func (s *scopeFS) Create(name string) (afero.File, error) {
	if err := s.check("create", name); err != nil {
		return nil, err
	}
	return s.base.Create(name)
}

func (s *scopeFS) Mkdir(name string, perm os.FileMode) error {
	if err := s.check("mkdir", name); err != nil {
		return err
	}
	return s.base.Mkdir(name, perm)
}

func (s *scopeFS) MkdirAll(name string, perm os.FileMode) error {
	if err := s.check("mkdir", name); err != nil {
		return err
	}
	return s.base.MkdirAll(name, perm)
}

func (s *scopeFS) Open(name string) (afero.File, error) {
	if err := s.check("open", name); err != nil {
		return nil, err
	}
	return s.base.Open(name)
}

func (s *scopeFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if err := s.check("open", name); err != nil {
		return nil, err
	}
	return s.base.OpenFile(name, flag, perm)
}

func (s *scopeFS) Remove(name string) error {
	if err := s.check("remove", name); err != nil {
		return err
	}
	return s.base.Remove(name)
}

func (s *scopeFS) RemoveAll(name string) error {
	if err := s.check("remove", name); err != nil {
		return err
	}
	return s.base.RemoveAll(name)
}

func (s *scopeFS) Rename(oldName, newName string) error {
	if err := s.check("rename", oldName); err != nil {
		return err
	}
	if err := s.check("rename", newName); err != nil {
		return err
	}
	return s.base.Rename(oldName, newName)
}

func (s *scopeFS) Stat(name string) (os.FileInfo, error) {
	if err := s.check("stat", name); err != nil {
		return nil, err
	}
	return s.base.Stat(name)
}

func (s *scopeFS) Chmod(name string, mode os.FileMode) error {
	if err := s.check("chmod", name); err != nil {
		return err
	}
	return s.base.Chmod(name, mode)
}

func (s *scopeFS) Chown(name string, uid, gid int) error {
	if err := s.check("chown", name); err != nil {
		return err
	}
	return s.base.Chown(name, uid, gid)
}

func (s *scopeFS) Chtimes(name string, atime, mtime time.Time) error {
	if err := s.check("chtimes", name); err != nil {
		return err
	}
	return s.base.Chtimes(name, atime, mtime)
}

func (s *scopeFS) check(op, name string) error {
	absolute := filepath.Clean(name)
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join("/", absolute)
	}
	relative, err := filepath.Rel(s.root, absolute)
	if err != nil {
		return &os.PathError{Op: op, Path: name, Err: err}
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return &os.PathError{Op: op, Path: name, Err: fs.ErrPermission}
	}
	return nil
}
