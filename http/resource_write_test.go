package http

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
)

func TestWriteFileReturnsCloseError(t *testing.T) {
	closeErr := errors.New("close upload")
	fileSystem := &closeErrorFS{
		Fs:  afero.NewMemMapFs(),
		err: closeErr,
	}

	_, err := writeFile(fileSystem, "/upload.txt", io.NopCloser(&zeroReader{}))
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected close error, got %v", err)
	}
}

type closeErrorFS struct {
	afero.Fs
	err error
}

func (f *closeErrorFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	file, err := f.Fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &closeErrorFile{File: file, err: f.err}, nil
}

type closeErrorFile struct {
	afero.File
	err error
}

func (f *closeErrorFile) Close() error {
	return errors.Join(f.File.Close(), f.err)
}

type zeroReader struct{}

func (*zeroReader) Read([]byte) (int, error) {
	return 0, io.EOF
}
