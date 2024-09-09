package http

import (
	"encoding/json"
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

	"golang.org/x/crypto/bcrypt"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/v2/files"
	"github.com/versioneer-tech/package-r/v2/s3fs"
	"github.com/versioneer-tech/package-r/v2/share"
)

type LinkData struct {
	FileInfo   files.FileInfo
	Source     share.Source
	SecretName string
}

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

		secretName := link.Source.Name + "---" + link.Hash

		bucket, prefix, session := link.Source.Connect(secretName)
		if session != nil {
			d.user.Fs = afero.NewBasePathFs(s3fs.NewFs(bucket, session), prefix+"/")
		}

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

		linkData := LinkData{
			FileInfo:   *fileInfo,
			Source:     link.Source,
			SecretName: secretName,
		}

		d.raw = linkData
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
	linkData := d.raw.(LinkData)
	fileInfo := linkData.FileInfo

	if fileInfo.IsDir {
		fileInfo.Listing.Sorting = files.Sorting{By: "name", Asc: false}
		fileInfo.Listing.ApplySort()
		return renderJSON(w, r, fileInfo)
	}

	var keys []string
	file, err := fileInfo.Fs.Open(fileInfo.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	keys = append(keys, file.Name())
	presignedURLs, _, err := linkData.Source.Presign(linkData.SecretName, keys)
	if err == nil {
		fileInfo.PresignedURL = presignedURLs[0]
	}
	return renderJSON(w, r, fileInfo)
})

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	linkData := d.raw.(LinkData)
	fileInfo := linkData.FileInfo

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
	presignedURLs, status, err := linkData.Source.Presign(linkData.SecretName, keys)

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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(presignedURLs)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
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
