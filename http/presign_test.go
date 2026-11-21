package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"golang.org/x/crypto/bcrypt"

	appmetrics "github.com/versioneer-tech/package-r/metrics"
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
	defer func() {
		if closeErr := result.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()

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
		url:   "https://objects.example.invalid/data.txt?signature=test",
	}
	store.Users = linker

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{
		Root: root,
	})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/my-share/data.txt?presign=true&follow=true", http.NoBody)
	recorder := httptest.NewRecorder()
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected temporary redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != linker.url {
		t.Fatalf("expected object-storage URL, got %q", location)
	}
	if strings.Contains(output.String(), "[DOWNLOAD]") {
		t.Fatalf("presigned redirect produced a proxy download log: %s", output.String())
	}
}

func TestLegacyPublicDownloadUsesPresignedURL(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	name := "S2B_T33UXP_20260218T100524_L2A/overview.tif"
	sharePath := "/my-bucket/vienna-s2l2a-26"
	writePresignTestFile(t, root, filepath.Join(sharePath, name))
	if err := store.Share.Save(&share.Link{Hash: "vienna-s2l2a-26", Path: sharePath, UserID: user.ID}); err != nil {
		t.Fatal(err)
	}
	linker := &recordingPublicLinkStore{
		Store: store.Users,
		url:   "https://objects.example.invalid/data.tif?signature=test",
	}
	store.Users = linker

	handler := handle(newLegacyPublicDownloadHandler(&settings.Server{
		Root:                     root,
		PublicPresignConcurrency: 1,
	}), "/api/public/dl/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/dl/vienna-s2l2a-26/"+name, http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected temporary redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != linker.url {
		t.Fatalf("expected object-storage URL, got %q", location)
	}
	if linker.name != filepath.ToSlash(filepath.Join(sharePath, name)) {
		t.Fatalf("expected shared object path, got %q", linker.name)
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
		url:   "https://objects.example.invalid/data.txt?signature=test",
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

func TestPublicSharePresignConcurrencyLimit(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if saveErr := store.Share.Save(&share.Link{
		Hash: "test-share", Path: "/files", UserID: user.ID,
		PasswordHash: string(passwordHash), Token: "test-token",
	}); saveErr != nil {
		t.Fatal(saveErr)
	}
	linker := &blockingPublicLinkStore{
		Store:   store.Users,
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	store.Users = linker
	server := &settings.Server{Root: root, PublicPresignConcurrency: 2}
	handler := newPublicShareHandler(server)
	set, err := store.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	telemetry := appmetrics.New(context.Background(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := func() (int, error) {
		req := httptest.NewRequest(http.MethodGet, "/test-share/data.txt?presign=true&follow=true", http.NoBody).WithContext(ctx)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/")
		req.Header.Set("X-SHARE-PASSWORD", "test-password")
		return handler(httptest.NewRecorder(), req, &data{store: store, server: server, settings: set, metrics: telemetry})
	}
	var requests sync.WaitGroup
	for range server.PublicPresignConcurrency {
		requests.Go(func() {
			status, requestErr := request()
			if requestErr != nil || status != http.StatusTemporaryRedirect {
				t.Errorf("accepted request: status=%d err=%v", status, requestErr)
			}
		})
	}
	defer requests.Wait()
	release := sync.OnceFunc(func() { close(linker.release) })
	defer release()
	for range server.PublicPresignConcurrency {
		select {
		case <-linker.entered:
		case <-ctx.Done():
			t.Fatal("accepted requests did not reach object storage")
		}
	}
	if status, requestErr := request(); requestErr != nil || status != http.StatusTooManyRequests {
		t.Fatalf("overflow request: status=%d err=%v", status, requestErr)
	}
	release()
	requests.Wait()
	if status, requestErr := request(); requestErr != nil || status != http.StatusTemporaryRedirect {
		t.Fatalf("request after completion: status=%d err=%v", status, requestErr)
	}
}

type blockingPublicLinkStore struct {
	users.Store
	entered chan struct{}
	release chan struct{}
}

func (s *blockingPublicLinkStore) PublicLink(ctx context.Context, _ *users.User, _ string, _ time.Duration) (string, error) {
	s.entered <- struct{}{}
	select {
	case <-s.release:
		return "https://objects.example.invalid/data.txt", nil
	case <-ctx.Done():
		return "", ctx.Err()
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
		url:   "https://objects.example.invalid/bucket/prefix/team/alice/files/data.txt?signature=test",
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
	defer func() {
		if closeErr := result.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
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
	linker := &recordingPublicLinkStore{
		Store: store.Users,
		url:   "https://objects.example.invalid/data.txt?signature=test",
	}
	store.Users = linker

	token := newTestAuthToken(t, store, user)
	handler := handle(resourceGetHandler, "/api/resources", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/resources/files/data.txt?presign=true", http.NoBody)
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	result := recorder.Result()
	defer func() {
		if closeErr := result.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	var file struct {
		PresignedURL string `json:"presignedURL"`
	}
	if err := json.NewDecoder(result.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.PresignedURL != linker.url {
		t.Fatalf("expected object-storage URL, got %q", file.PresignedURL)
	}
	if strings.Contains(output.String(), "[DOWNLOAD]") {
		t.Fatalf("presigned URL creation produced a proxy download log: %s", output.String())
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
		if closeErr := db.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})

	store := bolt.NewStorage(db)
	set := &settings.Settings{Key: []byte("test-key")}
	if saveErr2 := store.Settings.Save(set); saveErr2 != nil {
		t.Fatal(saveErr2)
	}

	user := &users.User{
		Username: "admin",
		Password: "my-password",
		Scope:    "/",
		Perm:     users.Permissions{Download: true},
	}
	if saveErr3 := store.Users.Save(user); saveErr3 != nil {
		t.Fatal(saveErr3)
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
	writePresignTestContent(t, root, relativePath, "data")
}

func writePresignTestContent(t *testing.T, root, relativePath, content string) {
	t.Helper()

	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
