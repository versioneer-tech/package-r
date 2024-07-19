package http

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/afero"
	"golang.org/x/crypto/bcrypt"

	"github.com/versioneer-tech/package-r/v2/files"
	"github.com/versioneer-tech/package-r/v2/share"
)

var withHashFile = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, ifPath := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
		if err != nil {
			return errToStatus(err), err
		}

		status, err := authenticateShareRequest(r, link)
		if status != 0 || err != nil {
			return status, err
		}

		user, err := d.store.Users.Get(link.Path, link.UserID)
		if err != nil {
			return errToStatus(err), err
		}

		d.user = user

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:      d.user.Fs,
			Path:    link.Path,
			Modify:  d.user.Perm.Modify,
			Expand:  false,
			Checker: d,
			Token:   link.Token,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// share base path
		basePath := link.Path

		// file relative path
		filePath := ""

		if file.IsDir {
			basePath = filepath.Dir(basePath)
			filePath = ifPath
		}

		// set fs root to the shared file/folder
		d.user.Fs = afero.NewBasePathFs(d.user.Fs, basePath)

		token := link.Token

		file, err = files.NewFileInfo(&files.FileOptions{
			Fs:      d.user.Fs,
			Path:    filePath,
			Modify:  d.user.Perm.Modify,
			Expand:  true,
			Checker: d,
			Token:   token,
		})
		if err != nil {
			return errToStatus(err), err
		}

		d.raw = file
		return fn(w, r, d)
	}
}

// ref to https://github.com/versioneer-tech/package-r/pull/727
// `/api/public/dl/MEEuZK-v/file-name.txt` for old browsers to save file with correct name
func ifPathWithName(r *http.Request) (id, filePath string) {
	pathElements := strings.Split(r.URL.Path, "/")
	// prevent maliciously constructed parameters like `/api/public/dl/XZzCDnK2_not_exists_hash_name`
	// len(pathElements) will be 1, and golang will panic `runtime error: index out of range`

	switch len(pathElements) {
	case 1:
		return r.URL.Path, "/"
	default:
		return pathElements[0], path.Join("/", path.Join(pathElements[1:]...))
	}
}

var publicShareHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file := d.raw.(*files.FileInfo)

	if file.IsDir {
		file.Listing.Sorting = files.Sorting{By: "name", Asc: false}
		file.Listing.ApplySort()
		return renderJSON(w, r, file)
	}

	return renderJSON(w, r, file)
})

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	fileInfo := d.raw.(*files.FileInfo)
	file, err := fileInfo.Fs.Open(fileInfo.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	var keys []string
	if fileInfo.IsDir {
		obs, err := file.Readdir(-100)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		for _, obj := range obs {
			keys = append(keys, obj.Name())
		}
	} else {
		keys = append(keys, file.Name())
	}

	session, errSession := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
		Endpoint:         aws.String(os.Getenv("AWS_ENDPOINT_URL")),
		Region:           aws.String(os.Getenv("AWS_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	})

	if errSession != nil {
		log.Print("Could not create session:", errSession)
		return 0, nil
	}

	s3Client := s3.New(session)

	var presignedURLs []string

	for _, key := range keys {
		getObjectInput := s3.GetObjectInput{
			Bucket: aws.String(os.Getenv("BUCKET_DEFAULT")),
			Key:    aws.String(key),
		}

		req, _ := s3Client.GetObjectRequest(&getObjectInput)

		presignedURL, err := req.Presign(7 * 24 * time.Hour) // 7d
		if err != nil {
			log.Printf("Could not presign %v: %v", getObjectInput, err)
			return http.StatusInternalServerError, err
		}

		presignedURLs = append(presignedURLs, presignedURL)
	}

	//nolint:goconst
	if r.URL.Query().Get("file") == "true" {
		reader := strings.NewReader(strings.Join(presignedURLs, "\n"))
		filename := "PRESIGNED_FILE_LIST.txt"
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(filename))
		w.Header().Add("Content-Security-Policy", `script-src 'none';`)
		w.Header().Set("Cache-Control", "private")
		http.ServeContent(w, r, filename, time.Now(), reader)
		return 0, nil
	}

	return renderJSON(w, r, presignedURLs)
})

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 0, nil
	}

	if r.URL.Query().Get("token") == l.Token {
		return 0, nil
	}

	password := r.Header.Get("X-SHARE-PASSWORD")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return 0, err
	}
	if password == "" {
		return http.StatusUnauthorized, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return http.StatusUnauthorized, nil
		}
		return 0, err
	}

	return 0, nil
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}
