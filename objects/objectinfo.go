package objects

import (
	"os"
	pathutil "path"
	"time"
)

// implements contract for os.FileInfo
type ObjectInfo struct {
	path        string
	directory   bool
	sizeInBytes int64
	modTime     time.Time
}

func NewObjectInfo(path string, directory bool, sizeInBytes int64, modTime time.Time) ObjectInfo {
	return ObjectInfo{
		path:        path,
		directory:   directory,
		sizeInBytes: sizeInBytes,
		modTime:     modTime,
	}
}

func (fi ObjectInfo) Path() string {
	return fi.path
}

func (fi ObjectInfo) Name() string {
	return pathutil.Base(fi.path)
}

func (fi ObjectInfo) IsDir() bool {
	return fi.directory
}

func (fi ObjectInfo) Size() int64 {
	return fi.sizeInBytes
}

//nolint:gomnd
func (fi ObjectInfo) Mode() os.FileMode {
	if fi.directory {
		return 0755
	}
	return 0664
}

func (fi ObjectInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi ObjectInfo) Sys() interface{} {
	return nil
}
