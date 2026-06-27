package http

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/versioneer-tech/package-r/files"
)

func presignOrLocalURL(filePath, method string, envs *map[string]string, localURL string) (string, error) {
	userEnvs := map[string]string(nil)
	if envs != nil {
		userEnvs = *envs
	}

	presignedURL, err := files.Presign(filePath, method, userEnvs)
	if errors.Is(err, files.ErrMissingS3Credentials) {
		return localURL, nil
	}
	return presignedURL, err
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
