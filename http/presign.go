package http

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	appErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/users"
)

const presignLifetime = 7 * 24 * time.Hour

func presignOrLocalURL(
	r *http.Request,
	store users.Store,
	user *users.User,
	filePath string,
	localURL string,
	lifetime time.Duration,
) (string, error) {
	if r.Method != http.MethodGet {
		return "", appErrors.ErrInvalidOption
	}
	linker, ok := store.(users.PublicLinker)
	if !ok {
		return localURL, nil
	}
	return linker.PublicLink(r.Context(), user, filePath, lifetime)
}

func localRawURL(r *http.Request, filePath string) string {
	return localRequestURL(r, "/api/raw"+slashClean(filePath))
}

func localPublicDownloadURL(r *http.Request) string {
	return localRequestURL(r, "/api/public/dl/"+strings.TrimPrefix(r.URL.Path, "/"))
}

func localRequestURL(r *http.Request, urlPath string) string {
	return (&url.URL{
		Scheme: requestScheme(r),
		Host:   r.Host,
		Path:   urlPath,
	}).String()
}

func requestScheme(r *http.Request) string {
	switch proto := r.Header.Get("X-Forwarded-Proto"); proto {
	case "http", "https":
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
