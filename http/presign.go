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
		if localURL == "" {
			return "", appErrors.ErrInvalidOption
		}
		return localURL, nil
	}
	return linker.PublicLink(r.Context(), user, filePath, lifetime)
}

func localRawURL(r *http.Request, baseURL, filePath string) string {
	return localRequestURL(r, baseURL, "/api/raw"+slashClean(filePath))
}

func localRequestURL(r *http.Request, baseURL, urlPath string) string {
	urlPath = "/" + strings.TrimPrefix(urlPath, "/")
	if baseURL = strings.Trim(baseURL, "/"); baseURL != "" {
		urlPath = "/" + baseURL + urlPath
	}

	return (&url.URL{
		Scheme: requestScheme(r),
		Host:   r.Host,
		Path:   urlPath,
	}).String()
}

func requestQueryEnabled(r *http.Request, name string) bool {
	values, ok := r.URL.Query()[name]
	return ok && len(values) > 0 && !strings.EqualFold(values[0], "false")
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
