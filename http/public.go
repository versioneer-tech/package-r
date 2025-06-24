package http

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/crypto/bcrypt"

	fbErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/share"
)

type catalogedFile struct {
	File          *files.FileInfo
	CatalogURL    string
	FilterField   string
	AssetsBaseURL string
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

		user, err := d.store.Users.Get(d.server.Root, link.UserID)
		if err != nil {
			return errToStatus(err), err
		}

		d.user = user

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       link.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
			Token:      link.Token,
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

		file, err = files.NewFileInfo(&files.FileOptions{
			Fs:      d.user.Fs,
			Path:    filePath,
			Modify:  d.user.Perm.Modify,
			Expand:  true,
			Checker: d,
			Token:   link.Token,
		})
		if err != nil {
			return errToStatus(err), err
		}

		d.raw = &catalogedFile{
			File:          file,
			CatalogURL:    link.CatalogURL,
			FilterField:   link.FiltersField,
			AssetsBaseURL: link.AssetsBaseURL,
		}

		return fn(w, r, d)
	}
}

// ref to https://github.com/filebrowser/filebrowser/pull/727
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
	cf := d.raw.(*catalogedFile)
	file := cf.File

	if file.IsDir {
		file.Sorting = files.Sorting{By: "name", Asc: false}
		file.ApplySort()
		return renderJSON(w, r, file)
	}

	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if errors.Is(err, fbErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}

		// do not waste bandwidth
		file.Content = ""
	}

	presign, ok := r.URL.Query()["presign"]
	if ok && !strings.EqualFold(presign[0], "false") {
		url, err := files.Presign(file.RealPath(), r.Method, *d.user.Envs)
		if errors.Is(err, fbErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
		file.PresignedURL = url

		// do not waste bandwidth
		file.Content = ""
	}

	follow, ok := r.URL.Query()["followRedirect"]
	if ok && !strings.EqualFold(follow[0], "false") && file.PresignedURL != "" {
		status := http.StatusTemporaryRedirect // 307 to preserve method
		http.Redirect(w, r, file.PresignedURL, status)
		return status, nil
	}

	if d.settings.Catalog.PreviewURL != "" {
		preview, ok := r.URL.Query()["preview"]
		if ok && !strings.EqualFold(preview[0], "false") {
			err := file.Preview()
			if errors.Is(err, fbErrors.ErrInvalidOption) {
				return http.StatusBadRequest, nil
			} else if err != nil {
				return http.StatusInternalServerError, err
			}

			scheme := "https"
			if strings.HasPrefix(r.Host, "localhost") {
				scheme = "http"
			}
			file.PreviewURL = d.settings.Catalog.PreviewURL + scheme + "://" + r.Host + "/api/public/catalog/" + r.URL.Path // TBD consider configurable base path

			// do not waste bandwidth
			file.Content = ""
		}
	}

	return renderJSON(w, r, file)
})

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusForbidden, nil
	}

	cf := d.raw.(*catalogedFile)
	file := cf.File

	if !file.IsDir {
		return rawFileHandler(w, r, file)
	}

	return rawDirHandler(w, r, d, file)
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
