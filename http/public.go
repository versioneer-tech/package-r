package http

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"golang.org/x/crypto/bcrypt"

	appErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
)

type catalogedFile struct {
	File          *files.FileInfo
	ShareHash     string
	SharePath     string
	CatalogURL    string
	AssetMappings []share.CatalogAssetMapping
	ShareExpire   int64
}

var withHashFile = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, ifPath := splitSharePath(r)
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
		d.skipUserDirBaseRules = true

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
			filePath = ifPath
		}

		// set fs root to the shared file/folder
		d.user.Fs = afero.NewBasePathFs(d.user.Fs, basePath)

		file, err = files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       filePath,
			Modify:     false,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
			Token:      link.Token,
			Content:    true,
		})
		if err != nil {
			return errToStatus(err), err
		}

		d.raw = &catalogedFile{
			File:          file,
			ShareHash:     link.Hash,
			SharePath:     link.Path,
			CatalogURL:    link.CatalogURL,
			AssetMappings: link.AssetMappings,
			ShareExpire:   link.Expire,
		}

		return fn(w, r, d)
	}
}

func newPublicShareHandler(server *settings.Server) handleFunc {
	limit := make(chan struct{}, server.PublicPresignConcurrency)
	passwords := newSharePasswordCache()
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.Method != http.MethodGet || !requestQueryEnabled(r, "presign") ||
			(!requestQueryEnabled(r, "follow") && !requestQueryEnabled(r, "followRedirect")) {
			return publicShareHandler(w, r, d)
		}
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		default:
			w.Header().Set("Retry-After", "1")
			return http.StatusTooManyRequests, nil
		}

		startedTotal := time.Now()
		defer func() { d.metrics.ObservePresignDuration("total", time.Since(startedTotal)) }()

		id, relativePath := splitSharePath(r)
		link, err := d.store.Share.GetByHash(id)
		if err != nil {
			return errToStatus(err), err
		}
		started := time.Now()
		status, err := passwords.authenticate(r, link)
		d.metrics.ObservePresignDuration("authentication", time.Since(started))
		if status != 0 || err != nil {
			return status, err
		}
		d.user, err = d.store.Users.Get(d.server.Root, link.UserID)
		if err != nil {
			return errToStatus(err), err
		}
		d.skipUserDirBaseRules = true

		if !d.Check(link.Path) || !d.Check(relativePath) {
			return http.StatusForbidden, nil
		}
		target := slashClean(path.Join(link.Path, relativePath))
		started = time.Now()
		file, err := d.user.Fs.Stat(target)
		d.metrics.ObservePresignDuration("resolution", time.Since(started))
		if err != nil {
			return errToStatus(err), err
		}
		if file.IsDir() {
			return publicShareHandler(w, r, d)
		}

		started = time.Now()
		redirectURL, err := presignOrLocalURL(r, d.store.Users, d.user, target, "", publicSharePresignLifetime(link.Expire))
		d.metrics.ObservePresignDuration("public_link", time.Since(started))
		d.metrics.ObservePresign(err == nil)
		if errors.Is(err, appErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return http.StatusTemporaryRedirect, nil
	}
}

type sharePasswordCache struct {
	mu       sync.RWMutex
	verified map[[sha256.Size]byte]struct{}
}

func newSharePasswordCache() *sharePasswordCache {
	return &sharePasswordCache{verified: make(map[[sha256.Size]byte]struct{})}
}

func (c *sharePasswordCache) authenticate(r *http.Request, link *share.Link) (int, error) {
	password := r.Header.Get("X-SHARE-PASSWORD")
	if link.PasswordHash == "" || password == "" ||
		(link.Token != "" && r.URL.Query().Get("token") == link.Token) {
		return authenticateShareRequest(r, link)
	}

	key := sha256.Sum256([]byte(link.PasswordHash + "\x00" + password))
	c.mu.RLock()
	_, ok := c.verified[key]
	c.mu.RUnlock()
	if ok {
		return 0, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.verified[key]; ok {
		return 0, nil
	}
	status, err := authenticateShareRequest(r, link)
	if status == 0 && err == nil {
		c.verified[key] = struct{}{}
	}
	return status, err
}

func splitSharePath(r *http.Request) (id, filePath string) {
	pathElements := strings.Split(r.URL.Path, "/")

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

	if cf.CatalogURL != "" && d.settings.STACBrowserURL != "" && requestQueryEnabled(r, "preview") {
		previewPath := catalogPreviewTarget(r.URL.Path, file.IsDir)
		catalogURL := localRequestURL(
			r,
			d.server.BaseURL,
			"/api/public/catalog/"+previewPath,
		)
		file.STACBrowserURL = d.settings.STACBrowserURL + catalogURL
	}

	if file.IsDir {
		file.Sorting = files.Sorting{By: "name", Asc: false}
		file.ApplySort()
		return renderJSON(w, r, file)
	}

	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if errors.Is(err, appErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}

		// do not waste bandwidth
		file.Content = ""
	}

	presign, ok := r.URL.Query()["presign"]
	if ok && !strings.EqualFold(presign[0], "false") {
		url, err := presignOrLocalURL(
			r,
			d.store.Users,
			d.user,
			publicSharePresignPath(cf),
			"",
			publicSharePresignLifetime(cf.ShareExpire),
		)
		d.metrics.ObservePresign(err == nil)
		if errors.Is(err, appErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
		file.PresignedURL = url
	}

	follow := requestQueryEnabled(r, "follow") || requestQueryEnabled(r, "followRedirect")
	if follow && file.PresignedURL != "" {
		status := http.StatusTemporaryRedirect // 307 to preserve method
		http.Redirect(w, r, file.PresignedURL, status)
		return status, nil
	}

	return renderJSON(w, r, file)
})

func catalogPreviewTarget(requestPath string, isDir bool) string {
	clean := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if isDir {
		if !strings.Contains(clean, "/") {
			return clean
		}
		return strings.TrimRight(clean, "/") + "/"
	}
	return strings.TrimRight(path.Dir(clean), "/") + "/"
}

func publicSharePresignPath(cf *catalogedFile) string {
	return slashClean(path.Join(cf.SharePath, cf.File.Path))
}

func publicSharePresignLifetime(expireUnix int64) time.Duration {
	if expireUnix == 0 {
		return presignLifetime
	}
	remaining := time.Until(time.Unix(expireUnix, 0))
	if remaining < presignLifetime {
		return remaining
	}
	return presignLifetime
}

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 0, nil
	}

	if l.Token != "" && r.URL.Query().Get("token") == l.Token {
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
