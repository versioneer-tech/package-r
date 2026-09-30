package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage/bolt"
	"github.com/versioneer-tech/package-r/users"
)

const sentinelItemID = "S2B_T33UXP_20260218T100524_L2A"

//nolint:gocyclo
func TestPublicCatalogEndpointReturnsSTACFromFixtureParquet(t *testing.T) {
	repoRoot := testRepoRoot(t)
	root := t.TempDir()
	fixturePath := filepath.Join(repoRoot, "tests", "data", "vienna-s2l2a-26")
	for _, required := range []string{
		"vienna-s2l2a-26.parquet",
		"vienna-boundary.geojson",
		filepath.Join(sentinelItemID, "rgb.tif"),
	} {
		if _, err := os.Stat(filepath.Join(fixturePath, required)); err != nil {
			t.Fatal("Sentinel-2 test data is not materialized. Run scripts/download_sentinel2.py; it downloads about 200 MB of Vienna data from February, May, and August 2026 from the Earth Search Sentinel-2 Collection 1 Level-2A catalog: https://earth-search.aws.element84.com/v1")
		}
	}
	catalogData, err := os.ReadFile(filepath.Join(fixturePath, "vienna-s2l2a-26.parquet"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatal("Sentinel-2 test data is not materialized. Run scripts/download_sentinel2.py; it downloads about 200 MB of Vienna data from February, May, and August 2026 from the Earth Search Sentinel-2 Collection 1 Level-2A catalog: https://earth-search.aws.element84.com/v1")
		}
		t.Fatal(err)
	}
	memoryFS := afero.NewMemMapFs()
	if err := memoryFS.MkdirAll("/public", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(memoryFS, "/public/catalog.parquet", catalogData, 0o600); err != nil {
		t.Fatal(err)
	}
	var sawRangeRequest atomic.Bool
	catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/catalog.parquet" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Range") != "" {
			sawRangeRequest.Store(true)
		}
		http.ServeContent(w, r, "catalog.parquet", time.Time{}, bytes.NewReader(catalogData))
	}))
	t.Cleanup(catalogServer.Close)
	overviewPath := filepath.Join(sentinelItemID, "overview.tif")
	overviewData, err := os.ReadFile(filepath.Join(fixturePath, overviewPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := memoryFS.MkdirAll(filepath.Join("/public", filepath.Dir(overviewPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(memoryFS, filepath.Join("/public", overviewPath), overviewData, 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := storm.Open(filepath.Join(t.TempDir(), "package-r.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})

	store := bolt.NewStorage(db)
	if err := store.Settings.Save(&settings.Settings{Key: []byte("test-key")}); err != nil {
		t.Fatal(err)
	}
	if err := store.Users.Save(&users.User{Username: "admin", Password: "my-password", Scope: "/"}); err != nil {
		t.Fatal(err)
	}
	catalogUsers := &catalogTestUserStore{
		Store:     store.Users,
		fs:        memoryFS,
		publicURL: catalogServer.URL + "/catalog.parquet",
	}
	store.Users = catalogUsers
	link := &share.Link{
		Hash:       "vienna-s2l2a-26",
		Path:       "/public",
		UserID:     1,
		CatalogURL: "/public/catalog.parquet",
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	handler := handle(catalogHandler, "/api/public/catalog/", store, &settings.Server{
		Root:    root,
		BaseURL: "/package-r",
	})

	collection := callCatalog[struct {
		Type        string                   `json:"type"`
		ID          string                   `json:"id"`
		STACVersion string                   `json:"stac_version"`
		Extent      map[string]interface{}   `json:"extent"`
		Links       []map[string]interface{} `json:"links"`
	}](t, handler, "/api/public/catalog/vienna-s2l2a-26")
	if collection.Type != "Collection" || collection.ID != "vienna-s2l2a-26" {
		t.Fatalf("expected STAC Collection root, got %#v", collection)
	}
	if collection.STACVersion != "1.1.0" {
		t.Fatalf("expected STAC version 1.1.0, got %q", collection.STACVersion)
	}
	if collection.Extent == nil || len(collection.Links) != 4 {
		t.Fatalf("expected Collection extent and three Item links, got %#v", collection)
	}

	feature := callCatalog[map[string]interface{}](t, handler,
		"/api/public/catalog/vienna-s2l2a-26/"+sentinelItemID+"/")
	if feature["type"] != "Feature" {
		t.Fatalf("expected STAC Feature, got %#v", feature)
	}
	properties, ok := feature["properties"].(map[string]interface{})
	if !ok || properties["datetime"] == nil {
		t.Fatalf("expected STAC datetime in item properties, got %#v", feature["properties"])
	}
	if _, exists := feature["collection"]; exists {
		t.Fatalf("expected collection without a link to move into properties, got %#v", feature)
	}
	if properties["collection"] != "sentinel-2-c1-l2a" {
		t.Fatalf("expected source collection metadata in properties, got %#v", properties["collection"])
	}
	if links, ok := feature["links"].([]interface{}); !ok || len(links) != 1 {
		t.Fatalf("expected STAC item links, got %#v", feature["links"])
	}

	expectedOverview := "http://localhost:8888/package-r/api/public/share/vienna-s2l2a-26/" +
		sentinelItemID + "/overview.tif?presign&followRedirect"
	if href := stacAssetHref(t, feature, "overview"); href != expectedOverview {
		t.Fatalf("unexpected rewritten asset href: %q", href)
	}
	expectedRGB := "http://localhost:8888/package-r/api/public/share/vienna-s2l2a-26/" +
		sentinelItemID + "/rgb.tif?presign&followRedirect"
	if href := stacAssetHref(t, feature, "rgb"); href != expectedRGB {
		t.Fatalf("unexpected rewritten RGB asset href: %q", href)
	}
	expectedBoundary := "http://localhost:8888/package-r/api/public/share/vienna-s2l2a-26/" +
		"vienna-boundary.geojson?presign&followRedirect"
	if href := stacAssetHref(t, feature, "vienna_boundary"); href != expectedBoundary {
		t.Fatalf("unexpected rewritten boundary asset href: %q", href)
	}

	item := callCatalog[map[string]interface{}](t, handler, "/api/public/catalog/vienna-s2l2a-26/"+
		sentinelItemID+"/overview.tif")
	if item["id"] != sentinelItemID {
		t.Fatalf("expected STAC item %q, got %#v", sentinelItemID, item)
	}
	links := feature["links"].([]interface{})
	selfLink := links[0].(map[string]interface{})
	expectedSelfHref := "http://localhost:8888/package-r/api/public/catalog/vienna-s2l2a-26/" +
		sentinelItemID + "/"
	if selfLink["href"] != expectedSelfHref {
		t.Fatalf("expected resolvable STAC self link %q, got %#v", expectedSelfHref, selfLink)
	}

	if collection.Links[0]["href"] != "http://localhost:8888/package-r/api/public/catalog/vienna-s2l2a-26" {
		t.Fatalf("unexpected Collection self link: %#v", collection.Links[0])
	}

	if catalogUsers.publicLinkName != "/public/catalog.parquet" {
		t.Fatalf("unexpected signed catalog path %q", catalogUsers.publicLinkName)
	}
	if catalogUsers.publicLinkExpire != catalogLinkLifetime {
		t.Fatalf("unexpected signed catalog lifetime %v", catalogUsers.publicLinkExpire)
	}
	if !sawRangeRequest.Load() {
		t.Fatal("expected DuckDB to read the catalog with an HTTP range request")
	}

	link.AssetMappings = []share.CatalogAssetMapping{{
		From: sentinelItemID + "/",
		To:   "mapped-assets/" + sentinelItemID,
	}}
	if err := store.Share.Update(link); err != nil {
		t.Fatal(err)
	}
	mappedOverviewPath := filepath.Join("/public/mapped-assets", sentinelItemID, "overview.tif")
	if err := memoryFS.MkdirAll(filepath.Dir(mappedOverviewPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(memoryFS, mappedOverviewPath, overviewData, 0o600); err != nil {
		t.Fatal(err)
	}
	mappedFeature := callCatalog[map[string]interface{}](t, handler,
		"/api/public/catalog/vienna-s2l2a-26/mapped-assets/"+sentinelItemID)
	expectedMappedOverview := "http://localhost:8888/package-r/api/public/share/vienna-s2l2a-26/mapped-assets/" +
		sentinelItemID + "/overview.tif?presign&followRedirect"
	if href := stacAssetHref(t, mappedFeature, "overview"); href != expectedMappedOverview {
		t.Fatalf("share asset mapping was not applied: %q", href)
	}
}

type catalogTestUserStore struct {
	users.Store
	fs               afero.Fs
	publicURL        string
	publicLinkName   string
	publicLinkExpire time.Duration
}

func (s *catalogTestUserStore) Get(baseScope string, id interface{}) (*users.User, error) {
	user, err := s.Store.Get(baseScope, id)
	if err != nil {
		return nil, err
	}
	user.Fs = s.fs
	return user, nil
}

func (s *catalogTestUserStore) PublicLink(_ context.Context, _ *users.User, name string, expire time.Duration) (string, error) {
	s.publicLinkName = name
	s.publicLinkExpire = expire
	return s.publicURL, nil
}

func TestRemoteCatalogURLDoesNotOutliveShare(t *testing.T) {
	memoryFS := afero.NewMemMapFs()
	if err := afero.WriteFile(memoryFS, "catalog.parquet", []byte("catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	linker := &recordingPublicLinkStore{url: "https://objects.example.invalid/catalog.parquet?signature=my-secret"}
	expire := time.Now().Add(time.Minute).Unix()

	got, release, err := remoteCatalogURL(
		context.Background(),
		linker,
		&users.User{Scope: "/team/alice"},
		memoryFS,
		"/public",
		"catalog.parquet",
		expire,
	)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()

	if got != linker.url {
		t.Fatalf("expected signed URL %q, got %q", linker.url, got)
	}
	if linker.name != "/public/catalog.parquet" {
		t.Fatalf("unexpected signed catalog path %q", linker.name)
	}
	if linker.expire <= 50*time.Second || linker.expire > time.Minute {
		t.Fatalf("expected signed URL to expire with the share, got %v", linker.expire)
	}
}

func TestRemoteCatalogURLRejectsOversizeCatalog(t *testing.T) {
	memoryFS := afero.NewMemMapFs()
	if err := afero.WriteFile(memoryFS, "catalog.parquet", []byte("catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	fsys := catalogSizeFS{Fs: memoryFS, size: maxRemoteCatalogSize + 1}
	linker := &recordingPublicLinkStore{url: "https://objects.example.invalid/catalog.parquet"}
	slotsBefore := len(catalogQuerySlots)

	_, release, err := remoteCatalogURL(
		context.Background(),
		linker,
		&users.User{},
		fsys,
		"/public",
		"catalog.parquet",
		0,
	)
	if err == nil {
		t.Fatal("expected an oversize catalog error")
	}
	if release != nil {
		t.Fatal("expected no release callback for a rejected catalog")
	}
	if slotsAfter := len(catalogQuerySlots); slotsAfter != slotsBefore {
		t.Fatalf("expected the query slot to be released, before=%d after=%d", slotsBefore, slotsAfter)
	}
	if linker.name != "" {
		t.Fatalf("expected no signed URL request, got path %q", linker.name)
	}
}

type catalogSizeFS struct {
	afero.Fs
	size int64
}

func (fsys catalogSizeFS) Stat(name string) (os.FileInfo, error) {
	info, err := fsys.Fs.Stat(name)
	if err != nil {
		return nil, err
	}
	return catalogSizeInfo{FileInfo: info, size: fsys.size}, nil
}

type catalogSizeInfo struct {
	os.FileInfo
	size int64
}

func (info catalogSizeInfo) Size() int64 {
	return info.size
}

func TestCatalogPathInShare(t *testing.T) {
	tests := []struct {
		name       string
		catalogURL string
		sharePath  string
		want       string
		wantError  bool
	}{
		{
			name:       "logical path",
			catalogURL: "/public/catalog.parquet",
			sharePath:  "/public",
			want:       "catalog.parquet",
		},
		{
			name:       "nested logical path",
			catalogURL: "/public/meta/catalog.parquet",
			sharePath:  "/public",
			want:       "meta/catalog.parquet",
		},
		{
			name:       "outside logical path",
			catalogURL: "/private/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
		{
			name:       "sibling prefix",
			catalogURL: "/publicity/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
		{
			name:       "traversal",
			catalogURL: "/public/../private/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
		{
			name:       "catalog equals share",
			catalogURL: "/public",
			sharePath:  "/public",
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := catalogPathInShare(test.catalogURL, test.sharePath)
			if test.wantError {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func callCatalog[T any](t *testing.T, handler http.Handler, path string) T {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888"+path, http.NoBody)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	result := recorder.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 for %s, got %d", path, result.StatusCode)
	}

	var response T
	if err := json.NewDecoder(result.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	return response
}

func stacAssetHref(t *testing.T, feature map[string]interface{}, assetKey string) string {
	t.Helper()

	assets, ok := feature["assets"].(map[string]interface{})
	if !ok {
		t.Fatalf("feature assets missing or invalid: %#v", feature["assets"])
	}
	asset, ok := assets[assetKey].(map[string]interface{})
	if !ok {
		t.Fatalf("asset %q missing or invalid: %#v", assetKey, assets[assetKey])
	}
	href, ok := asset["href"].(string)
	if !ok {
		t.Fatalf("asset href missing or invalid: %#v", asset["href"])
	}
	return href
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file path")
	}
	return filepath.Dir(filepath.Dir(file))
}
