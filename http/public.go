package http

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
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

		d.user, err = d.store.Users.Get(link.UserID)
		if err != nil {
			return errToStatus(err), err
		}

		d.user.Fs = d.InitFs(link.Path)

		fileInfo, err := files.NewFileInfo(&files.FileOptions{
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

		if fileInfo.IsDir {
			basePath = filepath.Dir(basePath)
			filePath = ifPath
		}

		// set fs root to the shared file/folder
		d.user.Fs = afero.NewBasePathFs(d.user.Fs, basePath)

		token := link.Token

		fileInfo, err = files.NewFileInfo(&files.FileOptions{
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

		if !fileInfo.IsDir {
			var keys []string
			file, err := fileInfo.Fs.Open(fileInfo.Path)
			if err == nil {
				keys = append(keys, file.Name())
				presignedURLs, _, err := presign(keys)
				if err == nil {
					fileInfo.Content = presignedURLs[0]
				}
			}
		}

		d.raw = fileInfo
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

//nolint:gocritic
func presign(keys []string) (presignedUrls []string, status int, err error) {
	presignedURLs := []string{}

	session, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
		Endpoint:         aws.String(os.Getenv("AWS_ENDPOINT_URL")),
		Region:           aws.String(os.Getenv("AWS_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		log.Print("Could not create session:", err)
		return presignedURLs, 0, nil
	}

	s3Client := s3.New(session)

	for _, key := range keys {
		getObjectInput := s3.GetObjectInput{
			Bucket: aws.String(os.Getenv("BUCKET_DEFAULT")),
			Key:    aws.String(key),
		}

		req, _ := s3Client.GetObjectRequest(&getObjectInput)

		presignedURL, err := req.Presign(7 * 24 * time.Hour) // 7d
		if err != nil {
			log.Printf("Could not presign %v: %v", getObjectInput, err)
			return presignedURLs, http.StatusInternalServerError, err
		}

		presignedURLs = append(presignedURLs, presignedURL)
	}

	return presignedURLs, 0, nil
}

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	fileInfo := d.raw.(*files.FileInfo)
	file, err := fileInfo.Fs.Open(fileInfo.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	maxObs, err := strconv.Atoi(os.Getenv("MAX_OBJECTS"))
	if err != nil {
		maxObs = 5000
	}

	var keys []string
	if fileInfo.IsDir {
		for {
			obs, err2 := file.Readdir(-1000)
			if err2 != nil {
				if errors.Is(err2, io.EOF) {
					break
				}
				return http.StatusInternalServerError, err
			}
			if len(obs) == 0 {
				break
			}
			for _, obj := range obs {
				keys = append(keys, obj.Name())
			}
			log.Printf("prepare presign (current %v)", len(keys))
			if len(keys) >= maxObs {
				break
			}
		}

	} else {
		keys = append(keys, file.Name())
	}

	log.Printf("start presign (total %v)", len(keys))
	presignedURLs, status, err := presign(keys)

	if err != nil {
		return status, err
	}

	//nolint:goconst
	if r.URL.Query().Get("file") == "true" {
		reader := strings.NewReader(strings.Join(presignedURLs, "\n"))
		filename := url.PathEscape(strings.ReplaceAll(os.Getenv("BRANDING_NAME")+"/"+file.Name()+".txt", "/", "__"))
		log.Printf("return presign file '%s'", filename)
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+filename)
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
