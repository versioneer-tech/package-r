package objects

import (
	"errors"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// implements contract for afero.File
type Object struct {
	fs                *ObjectFs
	name              string
	continuationToken *string
	hasMore           bool
}

func NewObject(fs *ObjectFs, name string) *Object {
	return &Object{
		fs:   fs,
		name: name,
	}
}

func (f *Object) Name() string {
	return f.name
}

// Readdir reads directory entries and returns a slice up to n slices in directory order.
// If n >= 1, it returns at most n, an empty slice indicates an error (io.EOF at the end)
// If n == 0 or -1, it returns all entries the directory in one slice
// If an error occurs before reading the entire directory, it returns the read FileInfos and a non-nil error.
// If n <= 2, it returns up to n entries, including those from subdirectories (with io.EOF at the end)
func (f *Object) Readdir(n int) ([]os.FileInfo, error) {
	if f.hasMore {
		return nil, io.EOF
	}

	if n == 0 || n == -1 {
		return f.ReaddirAll()
	}

	maxKeys := int64(n)
	delimiter := "/"
	if n <= -2 {
		maxKeys = int64(-n)
		delimiter = ""
	}

	// remove / at start, add / at the end but not at root
	name := strings.TrimPrefix(f.Name(), "/")
	if name != "" && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	listObjectsV2Input := s3.ListObjectsV2Input{
		ContinuationToken: f.continuationToken,
		Bucket:            aws.String(f.fs.bucket),
		Prefix:            aws.String(name),
		Delimiter:         aws.String(delimiter),
		MaxKeys:           aws.Int64(maxKeys),
	}
	response, err := f.fs.s3API.ListObjectsV2(&listObjectsV2Input)
	if err != nil {
		return nil, err
	}
	log.Printf("ListObjectsV2 <- %v keys (hasMore=%v) with prefix %s in bucket %s",
		*response.KeyCount, *response.IsTruncated, *response.Prefix, *response.Name)
	f.continuationToken = response.NextContinuationToken
	if !(*response.IsTruncated) {
		f.hasMore = true
	}
	var fileInfos = make([]os.FileInfo, 0, len(response.CommonPrefixes)+len(response.Contents))
	for _, subfolder := range response.CommonPrefixes {
		fileInfos = append(fileInfos, NewObjectInfo(*subfolder.Prefix, true, 0, time.Unix(0, 0)))
	}
	for _, fileObject := range response.Contents {
		if strings.HasSuffix(*fileObject.Key, "/") {
			// s3 includes <key>/ in response for <key>
			continue
		}
		fileInfos = append(fileInfos, NewObjectInfo(*fileObject.Key, false, *fileObject.Size, *fileObject.LastModified))
	}

	return fileInfos, nil
}

// Readdirnames reads all directory entries in batches
func (f *Object) ReaddirAll() ([]os.FileInfo, error) {
	batchsize := 1000
	var fileInfos []os.FileInfo
	for {
		infos, err := f.Readdir(batchsize)
		fileInfos = append(fileInfos, infos...)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}
	return fileInfos, nil
}

// Readdirnames reads directory entries and returns a slice of names.
// If n > 0, it returns at most n names; an empty slice indicates an error.
// At the end of the directory, it returns io.EOF.
// If n <= 0, it returns all names in the directory or any error encountered.
func (f *Object) Readdirnames(n int) ([]string, error) {
	fileinfos, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(fileinfos))
	for i, fileinfo := range fileinfos {
		_, names[i] = path.Split(fileinfo.Name())
	}
	return names, nil
}

func (f *Object) Stat() (os.FileInfo, error) {
	return f.fs.Stat(f.Name())
}

func (f *Object) Sync() error {
	// noop
	return nil
}

func (f *Object) Truncate(int64) error {
	return ErrNotImplemented
}

func (f *Object) WriteString(_ string) (int, error) {
	return 0, ErrNotImplemented
}

func (f *Object) Close() error {
	// noop
	return nil
}

func (f *Object) Read(_ []byte) (int, error) {
	return 0, ErrNotImplemented
}

func (f *Object) ReadAt(_ []byte, _ int64) (n int, err error) {
	return 0, ErrNotImplemented
}

func (f *Object) Seek(_ int64, _ int) (int64, error) {
	return 0, ErrNotImplemented
}

func (f *Object) Write(_ []byte) (int, error) {
	return 0, ErrNotImplemented
}

func (f *Object) WriteAt(_ []byte, _ int64) (n int, err error) {
	return 0, ErrNotImplemented
}
