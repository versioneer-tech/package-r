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

// configuredShare is the safe, read-only view of a configured public share.
// Password hashes and access tokens must not be returned by this endpoint.
type configuredShare struct {
	Hash        string `json:"hash"`
	Description string `json:"description,omitempty"`
	Expire      int64  `json:"expire"`
	URL         string `json:"url"`
}

var configuredShareGetsHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	links, err := d.store.Share.All()
	if errors.Is(err, appErrors.ErrNotExist) {
		return renderJSON(w, r, []configuredShare{})
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	configured := configuredSharesForUser(links, d.user.Scope, d.Check)
	return renderJSON(w, r, configured)
})

func configuredSharesForUser(links []*share.Link, scope string, allowed func(string) bool) []configuredShare {
	scope = cleanAccessPath(scope)
	configured := make([]configuredShare, 0)

	for _, link := range links {
		sharePath := cleanAccessPath(link.Path)
		if !pathAtOrBelow(sharePath, scope) {
			continue
		}

		requestPath := strings.TrimPrefix(sharePath, scope)
		requestPath = cleanAccessPath(requestPath)
		if !allowed(requestPath) {
			continue
		}

		configured = append(configured, configuredShare{
			Hash:        link.Hash,
			Description: link.Description,
			Expire:      link.Expire,
			URL:         path.Join("/share", link.Hash) + "/",
		})
	}

	sort.Slice(configured, func(i, j int) bool {
		return configured[i].Hash < configured[j].Hash
	})
	return configured
}
