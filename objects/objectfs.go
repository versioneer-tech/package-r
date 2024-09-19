package objects

import (
	"errors"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/afero"
)

type ObjectFs struct {
	bucket  string
	session *awsSession.Session
	s3API   *s3.S3
}

func NewObjectFs(bucket string, session *awsSession.Session) *ObjectFs {
	s3Api := s3.New(session)
	return &ObjectFs{
		bucket:  bucket,
		session: session,
		s3API:   s3Api,
	}
}

var ErrNotImplemented = errors.New("not implemented")

func (fs ObjectFs) Create(_ string) (afero.File, error) {
	return nil, ErrNotImplemented
}

func (fs ObjectFs) Mkdir(_ string, _ os.FileMode) error {
	return ErrNotImplemented
}

func (fs ObjectFs) MkdirAll(_ string, _ os.FileMode) error {
	return ErrNotImplemented
}

//nolint:gomnd
func (fs *ObjectFs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0777)
}

func (fs *ObjectFs) OpenFile(name string, _ int, _ os.FileMode) (afero.File, error) {
	file := NewObject(fs, name)
	_, err := file.Stat()
	return file, err
}

func (fs ObjectFs) Remove(_ string) error {
	return ErrNotImplemented
}

func (fs *ObjectFs) RemoveAll(_ string) error {
	return ErrNotImplemented
}

func (fs ObjectFs) Rename(_, _ string) error {
	return ErrNotImplemented
}

//nolint:gomnd
func (fs ObjectFs) Stat(name string) (os.FileInfo, error) {
	key := aws.String(name)
	if name == "/" {
		key = aws.String("*")
	}

	headObjectInput := s3.HeadObjectInput{
		Bucket: aws.String(fs.bucket),
		Key:    key,
	}
	log.Printf("HeadObject -> %s", headObjectInput)
	response, err := fs.s3API.HeadObject(&headObjectInput)
	if err != nil {
		var errRequestFailure awserr.RequestFailure
		if errors.As(err, &errRequestFailure) {
			statuscode := errRequestFailure.StatusCode()
			if statuscode == 404 {
				statDir, errStat := fs.statDirectory(name)
				return statDir, errStat
			}
			if statuscode == 403 {
				return nil, os.ErrPermission
			}
		}
		return ObjectInfo{}, &os.PathError{
			Op:   "stat",
			Path: name,
			Err:  err,
		}
	} else if strings.HasSuffix(name, "/") {
		// user asked for a directory, but this is a file
		return ObjectInfo{path: name}, nil
	}
	return NewObjectInfo(name, false, *response.ContentLength, *response.LastModified), nil
}

func (fs ObjectFs) statDirectory(name string) (os.FileInfo, error) {
	nameClean := path.Clean(name)
	out, err := fs.s3API.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(fs.bucket),
		Prefix:  aws.String(strings.TrimPrefix(nameClean, "/")),
		MaxKeys: aws.Int64(1),
	})
	if err != nil {
		return ObjectInfo{}, &os.PathError{
			Op:   "stat",
			Path: name,
			Err:  err,
		}
	}
	if (out.KeyCount == nil || *out.KeyCount == 0) && name != "" {
		return nil, &os.PathError{
			Op:   "stat",
			Path: name,
			Err:  os.ErrNotExist,
		}
	}
	return NewObjectInfo(name, true, 0, time.Unix(0, 0)), nil
}

func (ObjectFs) Name() string { return "ObjectFs" }

func (fs ObjectFs) Chmod(_ string, _ os.FileMode) error {
	return ErrNotImplemented
}

func (ObjectFs) Chown(string, int, int) error {
	return ErrNotImplemented
}

func (ObjectFs) Chtimes(string, time.Time, time.Time) error {
	return ErrNotImplemented
}
