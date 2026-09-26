package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/catalog"
	"github.com/versioneer-tech/package-r/users"
)

const (
	maxRemoteCatalogSize int64 = 256 << 20
	catalogLinkLifetime        = 15 * time.Minute
)

var catalogQuerySlots = make(chan struct{}, 4)

var catalogHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	cf := d.raw.(*catalogedFile)

	if cf.CatalogURL == "" {
		return http.StatusNotFound, nil
	}
	catalogPath, err := catalogPathInShare(cf.CatalogURL, d.server.Root, cf.SharePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	catalogURL, release, err := remoteCatalogURL(
		r.Context(),
		d.store.Users,
		d.user,
		d.user.Fs,
		cf.SharePath,
		catalogPath,
		cf.ShareExpire,
	)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer release()

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) < 1 {
		return http.StatusBadRequest, nil
	}

	assetsURL := localRequestURL(r, d.server.BaseURL, "/api/public/share/"+parts[0])
	catalogEndpoint := localRequestURL(r, d.server.BaseURL, "/api/public/catalog/"+parts[0])
	assetMappings := make([]catalog.AssetMapping, 0, len(d.settings.Catalog.AssetMappings))
	for _, mapping := range d.settings.Catalog.AssetMappings {
		assetMappings = append(assetMappings, catalog.AssetMapping{From: mapping.From, To: mapping.To})
	}

	result, err := catalog.QueryCatalogParquet(r.Context(), catalog.QueryOptions{
		CatalogURL:      catalogURL,
		RequestPath:     cf.File.Path,
		AssetsURL:       assetsURL,
		CatalogEndpoint: catalogEndpoint,
		SharePath:       cf.SharePath,
		AssetMappings:   assetMappings,
	})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSONWithContentType(w, result, "application/geo+json; charset=utf-8")
})

func catalogPathInShare(catalogURL, root, sharePath string) (string, error) {
	cleanCatalog := filepath.Clean(catalogURL)
	cleanRoot := filepath.Clean(root)
	if filepath.IsAbs(cleanCatalog) && cleanRoot != "." {
		relativeToRoot, err := filepath.Rel(cleanRoot, cleanCatalog)
		if err == nil && pathStaysWithinBase(relativeToRoot) {
			logicalPath := "/" + filepath.ToSlash(relativeToRoot)
			if relative, ok := pathRelativeToShare(logicalPath, sharePath); ok {
				return relative, nil
			}
		}
	}
	const legacyMountRoot = "/workspace"
	legacyLogicalPath := strings.TrimPrefix(filepath.ToSlash(cleanCatalog), legacyMountRoot)
	if legacyLogicalPath != filepath.ToSlash(cleanCatalog) {
		if relative, ok := pathRelativeToShare(legacyLogicalPath, sharePath); ok {
			return relative, nil
		}
	}

	if relative, ok := pathRelativeToShare(filepath.ToSlash(catalogURL), sharePath); ok {
		return relative, nil
	}
	return "", errors.New("catalog path is outside the shared tree")
}

func pathStaysWithinBase(relativePath string) bool {
	return relativePath != ".." &&
		!strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) &&
		!filepath.IsAbs(relativePath)
}

func pathRelativeToShare(catalogPath, sharePath string) (string, bool) {
	cleanCatalog := path.Clean("/" + strings.TrimPrefix(catalogPath, "/"))
	cleanShare := path.Clean("/" + strings.TrimPrefix(filepath.ToSlash(sharePath), "/"))
	prefix := strings.TrimSuffix(cleanShare, "/") + "/"
	if !strings.HasPrefix(cleanCatalog, prefix) {
		return "", false
	}

	relative := strings.TrimPrefix(cleanCatalog, prefix)
	return relative, relative != "" && relative != "."
}

func remoteCatalogURL(
	ctx context.Context,
	store users.Store,
	user *users.User,
	fsys afero.Fs,
	sharePath string,
	catalogPath string,
	shareExpire int64,
) (string, func(), error) {
	select {
	case catalogQuerySlots <- struct{}{}:
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { <-catalogQuerySlots })
	}

	info, err := fsys.Stat(catalogPath)
	if err != nil {
		release()
		return "", nil, fmt.Errorf("stat catalog through storage: %w", err)
	}
	if info.Size() < 0 || info.Size() > maxRemoteCatalogSize {
		release()
		return "", nil, fmt.Errorf("catalog exceeds the %d-byte remote query limit", maxRemoteCatalogSize)
	}

	linker, ok := store.(users.PublicLinker)
	if !ok {
		release()
		return "", nil, errors.New("catalog storage does not support signed read URLs")
	}
	lifetime := catalogLinkLifetime
	if shareExpire != 0 {
		remaining := time.Until(time.Unix(shareExpire, 0))
		if remaining <= 0 {
			release()
			return "", nil, errors.New("catalog share has expired")
		}
		if remaining < lifetime {
			lifetime = remaining
		}
	}
	objectPath := slashClean(path.Join(sharePath, catalogPath))
	catalogURL, err := linker.PublicLink(ctx, user, objectPath, lifetime)
	if err != nil {
		release()
		return "", nil, fmt.Errorf("create signed catalog URL: %w", err)
	}
	return catalogURL, release, nil
}
