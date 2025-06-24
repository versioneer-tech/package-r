package http

import (
	"net/http"
	"strings"

	"github.com/versioneer-tech/package-r/catalog"
)

var catalogHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	cf := d.raw.(*catalogedFile)

	if cf.CatalogURL == "" {
		return http.StatusNotFound, nil
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) < 1 {
		return http.StatusBadRequest, nil
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	assetsURL := scheme + "://" + r.Host + "/api/public/share/" + parts[0] // TBD consider configurable base path

	result, err := catalog.QueryCatalogParquet(r.Context(), cf.CatalogURL, cf.FilterField, cf.AssetsBaseURL, cf.File.Path, assetsURL)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, result)
})
