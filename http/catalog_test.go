package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/asdine/storm/v3"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage/bolt"
	"github.com/versioneer-tech/package-r/users"
)

const openAerialMapID = "67793f0b9478720001790586"

func TestPublicCatalogEndpointReturnsSTACFromFixtureParquet(t *testing.T) {
	repoRoot := testRepoRoot(t)
	root := t.TempDir()
	publicDir := filepath.Join(root, "public")
	if err := os.CopyFS(publicDir, os.DirFS(filepath.Join(repoRoot, "tests", "data"))); err != nil {
		t.Fatal(err)
	}

	db, err := storm.Open(filepath.Join(t.TempDir(), "filebrowser.db"))
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
	if err := store.Users.Save(&users.User{Username: "admin", Password: "password", Scope: "/"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Share.Save(&share.Link{
		Hash:       "public-share",
		Path:       "/public",
		UserID:     1,
		CatalogURL: filepath.Join(publicDir, "catalog.parquet"),
	}); err != nil {
		t.Fatal(err)
	}

	handler := handle(catalogHandler, "/api/public/catalog/", store, &settings.Server{Root: root})

	collection := callCatalog[struct {
		Type     string                   `json:"type"`
		Features []map[string]interface{} `json:"features"`
	}](t, handler, "/api/public/catalog/public-share")
	if collection.Type != "FeatureCollection" || len(collection.Features) != 3 {
		t.Fatalf("expected STAC FeatureCollection with 3 features, got %#v", collection)
	}

	feature := findFeature(t, collection.Features, openAerialMapID)
	if feature["type"] != "Feature" {
		t.Fatalf("expected STAC Feature, got %#v", feature)
	}

	expectedThumbnail := "http://localhost:8080/api/public/share/public-share/openaerialmap-assets/" +
		openAerialMapID + "/thumbnail.png?presign&followRedirect"
	if href := stacAssetHref(t, feature, "thumbnail"); href != expectedThumbnail {
		t.Fatalf("unexpected rewritten asset href: %q", href)
	}

	item := callCatalog[map[string]interface{}](t, handler, "/api/public/catalog/public-share/openaerialmap-assets/"+
		openAerialMapID+"/thumbnail.png")
	if item["id"] != openAerialMapID {
		t.Fatalf("expected STAC item %q, got %#v", openAerialMapID, item)
	}
}

func callCatalog[T any](t *testing.T, handler http.Handler, path string) T {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080"+path, http.NoBody)
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
