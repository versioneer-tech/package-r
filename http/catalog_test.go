package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage"
	"github.com/versioneer-tech/package-r/storage/bolt"
	"github.com/versioneer-tech/package-r/users"
)

func monkey(fn handleFunc, prefix string, store *storage.Storage, server *settings.Server) http.Handler {
	return handle(fn, prefix, store, server)
}

func TestCatalogHandler_Parquet(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	if err := store.Users.Save(&users.User{Username: "testuser", Password: "testpass"}); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	parquetPath := filepath.Join(filepath.Dir(filename), "tests-examples", "naip-10.parquet")

	link := &share.Link{
		Hash:       "public-naip-10",
		UserID:     1,
		CatalogURL: parquetPath,
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatalf("failed to save share: %v", err)
	}

	store.Users = &customFSUser{
		Store: store.Users,
		fs:    afero.NewOsFs(),
	}

	server := &settings.Server{}

	router := mux.NewRouter()
	sub := router.PathPrefix("/api/catalog/{hash}").Subrouter()
	sub.Handle("/search", monkey(catalogHandler, "/api/catalog", store, server)).Methods("GET")

	q := url.Values{}
	q.Set("url", parquetPath)
	q.Set("type", "parquet")
	req := httptest.NewRequest(http.MethodGet, "/api/catalog/public-naip-10"+q.Encode(), nil)
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if body == "" {
		t.Fatal("response body is empty")
	}

	featureCount := strings.Count(body, `"id":`)
	if featureCount != 10 {
		t.Errorf("expected 10 features, got %d\nBody: %s", featureCount, body)
	}
}
