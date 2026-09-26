package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asdine/storm/v3"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage"
	"github.com/versioneer-tech/package-r/storage/bolt"
	"github.com/versioneer-tech/package-r/users"
)

func TestResourcePresignFallsBackToLocalRawURLWithoutRcloneStorage(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")

	token := newTestAuthToken(t, store, user)
	handler := handle(resourceGetHandler, "/api/resources", store, &settings.Server{
		Root:    root,
		BaseURL: "/package-r",
	})
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
	if file.PresignedURL != "http://localhost:8888/package-r/api/raw/files/data.txt" {
		t.Fatalf("expected local raw fallback URL, got %q", file.PresignedURL)
	}
}

func TestPublicShareOpenUsesPresignedURL(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	user.Perm.Download = false
	if err := store.Users.Update(user, "Perm"); err != nil {
		t.Fatal(err)
	}
	if err := store.Share.Save(&share.Link{Hash: "my-share", Path: "/files", UserID: 1}); err != nil {
		t.Fatal(err)
	}
	linker := &recordingPublicLinkStore{
		Store: store.Users,
		url:   "https://objects.example.invalid/data.txt?signature=xyz",
	}
	store.Users = linker

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{
		Root: root,
	})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/my-share/data.txt?presign=true&follow=true", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected temporary redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != linker.url {
		t.Fatalf("expected object-storage URL, got %q", location)
	}
}

func TestPublicSharePresignHasNoLocalDownloadFallback(t *testing.T) {
	root, store, _ := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	if err := store.Share.Save(&share.Link{Hash: "my-share", Path: "/files", UserID: 1}); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/my-share/data.txt?presign=true", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestPublicSharePresignRejectsHead(t *testing.T) {
	root, store, _ := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	if err := store.Share.Save(&share.Link{Hash: "my-share", Path: "/files", UserID: 1}); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodHead, "http://localhost:8888/api/public/share/my-share/data.txt?presign=true", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestPublicSharePresignDoesNotOutliveShare(t *testing.T) {
	root, store, _ := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	linker := &recordingPublicLinkStore{
		Store: store.Users,
		url:   "https://objects.example.invalid/data.txt?signature=xyz",
	}
	store.Users = linker
	expire := time.Now().Add(5 * time.Minute).Unix()
	if err := store.Share.Save(&share.Link{
		Hash:   "my-share",
		Path:   "/files",
		UserID: 1,
		Expire: expire,
	}); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/my-share/data.txt?presign=true", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if linker.expire <= 4*time.Minute || linker.expire > 5*time.Minute {
		t.Fatalf("expected public link expiry to match the remaining share lifetime, got %v", linker.expire)
	}
}

func TestResourcePresignUsesRclonePublicLinker(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	user.Scope = "/team/alice"
	if err := store.Users.Update(user, "Scope"); err != nil {
		t.Fatal(err)
	}
	linker := &recordingPublicLinkStore{
		Store: store.Users,
		url:   "https://objects.example.invalid/bucket/prefix/team/alice/files/data.txt?signature=xyz",
	}
	store.Users = linker
	writePresignTestFile(t, root, "team/alice/files/data.txt")

	token := newTestAuthToken(t, store, user)
	handler := handle(resourceGetHandler, "/api/resources", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/resources/files/data.txt?presign=true", http.NoBody)
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	result := recorder.Result()
	defer result.Body.Close()
	var file struct {
		PresignedURL string `json:"presignedURL"`
	}
	if err := json.NewDecoder(result.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	presigned, err := url.Parse(file.PresignedURL)
	if err != nil {
		t.Fatal(err)
	}
	if presigned.Path != "/bucket/prefix/team/alice/files/data.txt" {
		t.Fatalf("unexpected presigned object path %q", presigned.Path)
	}
	if linker.userScope != "/team/alice" || linker.name != "/files/data.txt" {
		t.Fatalf("unexpected public link input: scope=%q name=%q", linker.userScope, linker.name)
	}
	if linker.expire != presignLifetime {
		t.Fatalf("unexpected public link expiry %v", linker.expire)
	}
}

func TestResourcePresignDoesNotRequireProxyDownloadPermission(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	user.Perm.Download = false
	if err := store.Users.Update(user, "Perm"); err != nil {
		t.Fatal(err)
	}

	token := newTestAuthToken(t, store, user)
	handler := handle(resourceGetHandler, "/api/resources", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/resources/files/data.txt?presign=true", http.NoBody)
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	result := recorder.Result()
	defer result.Body.Close()
	var file struct {
		PresignedURL string `json:"presignedURL"`
	}
	if err := json.NewDecoder(result.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.PresignedURL == "" {
		t.Fatal("expected presigned URL")
	}
}

func newPresignTestStorage(t *testing.T) (string, *storage.Storage, *users.User) {
	t.Helper()

	root := t.TempDir()
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
	set := &settings.Settings{Key: []byte("test-key")}
	if err := store.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	user := &users.User{
		Username: "admin",
		Password: "my-password",
		Scope:    "/",
		Perm:     users.Permissions{Download: true},
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

type recordingPublicLinkStore struct {
	users.Store
	url       string
	userScope string
	name      string
	expire    time.Duration
}

func (s *recordingPublicLinkStore) PublicLink(_ context.Context, user *users.User, name string, expire time.Duration) (string, error) {
	s.userScope = user.Scope
	s.name = name
	s.expire = expire
	return s.url, nil
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
