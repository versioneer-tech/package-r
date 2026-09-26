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

const openAerialMapID = "67793f0b9478720001790586"

//nolint:gocyclo
func TestPublicCatalogEndpointReturnsSTACFromFixtureParquet(t *testing.T) {
	repoRoot := testRepoRoot(t)
	root := t.TempDir()
	catalogData, err := os.ReadFile(filepath.Join(repoRoot, "tests", "data", "catalog-sample", "catalog.parquet"))
	if err != nil {
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
	thumbnailPath := filepath.Join("openaerialmap-assets", openAerialMapID, "thumbnail.png")
	thumbnailData, err := os.ReadFile(filepath.Join(repoRoot, "tests", "data", "catalog-sample", thumbnailPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := memoryFS.MkdirAll(filepath.Join("/public", filepath.Dir(thumbnailPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(memoryFS, filepath.Join("/public", thumbnailPath), thumbnailData, 0o600); err != nil {
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

	store, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatal(err)
	}
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
		Hash:       "my-share",
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
		STACVersion string                   `json:"stac_version"`
		Links       []map[string]interface{} `json:"links"`
		Features    []map[string]interface{} `json:"features"`
	}](t, handler, "/api/public/catalog/my-share")
	if collection.Type != "FeatureCollection" || len(collection.Features) != 3 {
		t.Fatalf("expected STAC FeatureCollection with 3 features, got %#v", collection)
	}
	if collection.STACVersion != "1.1.0" {
		t.Fatalf("expected STAC version 1.1.0, got %q", collection.STACVersion)
	}
	if collection.Links == nil {
		t.Fatal("expected STAC FeatureCollection links")
	}

	feature := findFeature(t, collection.Features, openAerialMapID)
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
	if properties["collection"] != "openaerialmap" {
		t.Fatalf("expected source collection metadata in properties, got %#v", properties["collection"])
	}
	if links, ok := feature["links"].([]interface{}); !ok || len(links) != 1 {
		t.Fatalf("expected STAC item links, got %#v", feature["links"])
	}

	expectedThumbnail := "http://localhost:8888/package-r/api/public/share/my-share/openaerialmap-assets/" +
		openAerialMapID + "/thumbnail.png?presign&followRedirect"
	if href := stacAssetHref(t, feature, "thumbnail"); href != expectedThumbnail {
		t.Fatalf("unexpected rewritten asset href: %q", href)
	}

	item := callCatalog[map[string]interface{}](t, handler, "/api/public/catalog/my-share/openaerialmap-assets/"+
		openAerialMapID+"/thumbnail.png")
	if item["id"] != openAerialMapID {
		t.Fatalf("expected STAC item %q, got %#v", openAerialMapID, item)
	}
	links := feature["links"].([]interface{})
	selfLink := links[0].(map[string]interface{})
	expectedSelfHref := "http://localhost:8888/package-r/api/public/catalog/my-share/openaerialmap-assets/" +
		openAerialMapID + "/metadata.json"
	if selfLink["href"] != expectedSelfHref {
		t.Fatalf("expected resolvable STAC self link %q, got %#v", expectedSelfHref, selfLink)
	}

	link.CatalogURL = "/workspace/public/catalog.parquet"
	if err := store.Share.Update(link); err != nil {
		t.Fatal(err)
	}
	legacyCollection := callCatalog[struct {
		Features []map[string]interface{} `json:"features"`
	}](t, handler, "/api/public/catalog/my-share")
	if len(legacyCollection.Features) != 3 {
		t.Fatalf("expected legacy catalog path to use the VFS, got %#v", legacyCollection)
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
		From: "openaerialmap-assets/",
		To:   "mapped-assets",
	}}
	if err := store.Share.Update(link); err != nil {
		t.Fatal(err)
	}
	mappedCollection := callCatalog[struct {
		Features []map[string]interface{} `json:"features"`
	}](t, handler, "/api/public/catalog/my-share")
	mappedFeature := findFeature(t, mappedCollection.Features, openAerialMapID)
	expectedMappedThumbnail := "http://localhost:8888/package-r/api/public/share/my-share/mapped-assets/" +
		openAerialMapID + "/thumbnail.png?presign&followRedirect"
	if href := stacAssetHref(t, mappedFeature, "thumbnail"); href != expectedMappedThumbnail {
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
	root := t.TempDir()
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
			name:       "legacy physical path",
			catalogURL: "/workspace/public/catalog.parquet",
			sharePath:  "/public",
			want:       "catalog.parquet",
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
		{
			name:       "legacy root sibling",
			catalogURL: root + "-other/public/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := catalogPathInShare(test.catalogURL, root, test.sharePath)
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

func findFeature(t *testing.T, features []map[string]interface{}, id string) map[string]interface{} {
	t.Helper()

	for _, feature := range features {
		if feature["id"] == id {
			return feature
		}
	}
	t.Fatalf("feature %q not found in %#v", id, features)
	return nil
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
