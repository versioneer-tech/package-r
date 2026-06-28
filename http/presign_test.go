package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage"
	"github.com/versioneer-tech/package-r/storage/bolt"
	"github.com/versioneer-tech/package-r/users"
)

func TestResourcePresignFallsBackToLocalRawURLWithoutS3Credentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")

	token := newTestAuthToken(t, store, user)
	handler := handle(resourceGetHandler, "/api/resources", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/resources/files/data.txt?presign=true", http.NoBody)
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	result := recorder.Result()
	defer result.Body.Close()

	var file struct {
		PresignedURL string `json:"presignedURL"`
	}
	if err := json.NewDecoder(result.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.PresignedURL != "http://localhost:8888/api/raw/files/data.txt" {
		t.Fatalf("expected local raw fallback URL, got %q", file.PresignedURL)
	}
}

func TestPublicSharePresignRedirectsToLocalDownloadURLWithoutS3Credentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	root, store, _ := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	if err := store.Share.Save(&share.Link{Hash: "public-share", Path: "/files", UserID: 1}); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/public-share/data.txt?presign=true&followRedirect=true", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected temporary redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "http://localhost:8888/api/public/dl/public-share/data.txt" {
		t.Fatalf("expected local public download fallback URL, got %q", location)
	}
}

func newPresignTestStorage(t *testing.T) (string, *storage.Storage, *users.User) {
	t.Helper()

	root := t.TempDir()
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
	set := &settings.Settings{Key: []byte("test-key")}
	if err := store.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	envs := map[string]string{
		"AWS_ACCESS_KEY_ID":     "",
		"AWS_SECRET_ACCESS_KEY": "",
	}
	user := &users.User{
		Username: "admin",
		Password: "password",
		Scope:    "/",
		Perm:     users.Permissions{Download: true},
		Envs:     &envs,
	}
	if err := store.Users.Save(user); err != nil {
		t.Fatal(err)
	}
	user, err = store.Users.Get(root, "admin")
	if err != nil {
		t.Fatal(err)
	}

	return root, store, user
}

func newTestAuthToken(t *testing.T, store *storage.Storage, user *users.User) string {
	t.Helper()

	set, err := store.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	status, err := printToken(recorder, httptest.NewRequest(http.MethodGet, "http://localhost:8888", http.NoBody), &data{settings: set}, user, DefaultTokenExpirationTime)
	if status != 0 || err != nil {
		t.Fatalf("failed to create test auth token: status=%d err=%v", status, err)
	}
	return recorder.Body.String()
}

func writePresignTestFile(t *testing.T, root, relativePath string) {
	t.Helper()

	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
}
