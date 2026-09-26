package http

import (
	"errors"
	"net/http"
	"path"
	"sort"
	"strings"

	appErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/share"
)

// configuredShare is the safe, read-only view of a bootstrap share.
// Password hashes and access tokens must not be returned by this endpoint.
type configuredShare struct {
	Hash        string `json:"hash"`
	Description string `json:"description,omitempty"`
	Expire      int64  `json:"expire"`
	URL         string `json:"url"`
}

var configuredShareGetsHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	requestPath := cleanAccessPath(r.URL.Path)
	if !d.Check(requestPath) {
		return http.StatusForbidden, nil
	}

	links, err := d.store.Share.All()
	if errors.Is(err, appErrors.ErrNotExist) {
		return renderJSON(w, r, []configuredShare{})
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	targetPath := scopedSharePath(d.user.Scope, requestPath)
	configured := configuredSharesForPath(links, targetPath, strings.HasSuffix(r.URL.Path, "/"))
	return renderJSON(w, r, configured)
})

func scopedSharePath(scope, requestPath string) string {
	cleanScope := cleanAccessPath(scope)
	if cleanScope == "/" {
		return cleanAccessPath(requestPath)
	}
	return cleanAccessPath(path.Join(cleanScope, strings.TrimPrefix(requestPath, "/")))
}

func configuredSharesForPath(links []*share.Link, targetPath string, trailingSlash bool) []configuredShare {
	targetPath = cleanAccessPath(targetPath)
	configured := make([]configuredShare, 0)

	for _, link := range links {
		sharePath := cleanAccessPath(link.Path)
		if !pathAtOrBelow(targetPath, sharePath) {
			continue
		}

		relativePath := strings.TrimPrefix(targetPath, sharePath)
		publicURL := path.Join("/share", link.Hash, strings.TrimPrefix(relativePath, "/"))
		if trailingSlash && !strings.HasSuffix(publicURL, "/") {
			publicURL += "/"
		}

		configured = append(configured, configuredShare{
			Hash:        link.Hash,
			Description: link.Description,
			Expire:      link.Expire,
			URL:         publicURL,
		})
	}

	sort.Slice(configured, func(i, j int) bool {
		return configured[i].Hash < configured[j].Hash
	})
	return configured
}
