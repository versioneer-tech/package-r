package fileutils

import (
	"errors"
	"os"
	"testing"

	"github.com/spf13/afero"
)

var errCloseDestination = errors.New("close destination")
var errCloseSource = errors.New("close source")

func TestCopyFileReportsSourceCloseError(t *testing.T) {
	for _, destination := range []string{"/test-destination.txt", "/test-blocked/test-destination.txt"} {
		t.Run(destination, func(t *testing.T) {
			base := afero.NewMemMapFs()
			if err := afero.WriteFile(base, "/test-source.txt", []byte("data"), 0o644); err != nil {
				t.Fatal(err)
			}
			fsys := &closeErrorFS{Fs: base, source: "/test-source.txt"}
			if destination == "/test-blocked/test-destination.txt" {
				fsys.mkdirErr = os.ErrPermission
			}
			err := CopyFile(fsys, "/test-source.txt", destination)
			if !errors.Is(err, errCloseSource) {
				t.Fatalf("expected source close error, got %v", err)
			}
			if fsys.mkdirErr != nil && !errors.Is(err, fsys.mkdirErr) {
				t.Fatalf("expected directory creation error to be preserved, got %v", err)
			}
		})
	}
}

func TestCopyFileReturnsDestinationCloseError(t *testing.T) {
	base := afero.NewMemMapFs()
	if err := afero.WriteFile(base, "/source.txt", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := &closeErrorFS{Fs: base, destination: "/destination.txt"}

	err := CopyFile(fsys, "/source.txt", "/destination.txt")
	if !errors.Is(err, errCloseDestination) {
		t.Fatalf("expected destination close error, got %v", err)
	}
}

func TestCommonPrefix(t *testing.T) {
	testCases := map[string]struct {
		paths []string
		want  string
	}{
		"same lvl": {
			paths: []string{
				"/home/user/file1",
				"/home/user/file2",
			},
			want: "/home/user",
		},
		"sub folder": {
			paths: []string{
				"/home/user/folder",
				"/home/user/folder/file",
			},
			want: "/home/user/folder",
		},
		"relative path": {
			paths: []string{
				"/home/user/folder",
				"/home/user/folder/../folder2",
			},
			want: "/home/user",
		},
		"no common path": {
			paths: []string{
				"/home/user/folder",
				"/etc/file",
			},
			want: "",
		},
	}
	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := CommonPrefix('/', tt.paths...); got != tt.want {
				t.Errorf("CommonPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

type closeErrorFS struct {
	afero.Fs
	destination string
	source      string
	mkdirErr    error
}

func (f *closeErrorFS) MkdirAll(name string, perm os.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	return f.Fs.MkdirAll(name, perm)
}

func (f *closeErrorFS) Open(name string) (afero.File, error) {
	file, err := f.Fs.Open(name)
	if err != nil || name != f.source {
		return file, err
	}
	return &closeErrorFile{File: file, err: errCloseSource}, nil
}

func (f *closeErrorFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	file, err := f.Fs.OpenFile(name, flag, perm)
	if err != nil || name != f.destination {
		return file, err
	}
	return &closeErrorFile{File: file, err: errCloseDestination}, nil
}

type closeErrorFile struct {
	afero.File
	err error
}

func (f *closeErrorFile) Close() error {
	return errors.Join(f.File.Close(), f.err)
}
