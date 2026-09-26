package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage/bolt"
	"github.com/versioneer-tech/package-r/users"
	"golang.org/x/crypto/bcrypt"
)

func TestPublicShareBypassesGeneratedUserDirBaseRules(t *testing.T) {
	root := t.TempDir()
	writePresignTestFile(t, root, "home/bob/data.txt")

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

	set := &settings.Settings{
		Key: []byte("test-key"),
	}
	if err := store.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	user := &users.User{Username: "alice", Password: "my-password", Scope: "/"}
	set.ApplyUserDefaults(user)
	if err := store.Users.Save(user); err != nil {
		t.Fatal(err)
	}

	if err := store.Share.Save(&share.Link{
		Hash:   "my-share",
		Path:   "/home/bob",
		UserID: user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/my-share/data.txt", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected public share to bypass generated user-dir rules, got status %d", recorder.Code)
	}
}

func TestPublicSharePresignPathUsesSharedFilesystemPath(t *testing.T) {
	cf := &catalogedFile{
		SharePath: "/bucket/public",
		File:      &files.FileInfo{Path: "/openaerialmap-assets/item/thumbnail.png"},
	}

	got := publicSharePresignPath(cf)
	want := "/bucket/public/openaerialmap-assets/item/thumbnail.png"
	if got != want {
		t.Fatalf("expected public share presign path %q, got %q", want, got)
	}
}

func TestPublicShareSTACBrowserURLUsesRequestSchemeAndBaseURL(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	if err := store.Share.Save(&share.Link{
		Hash:       "my-share",
		Path:       "/files",
		UserID:     user.ID,
		CatalogURL: "catalog.parquet",
	}); err != nil {
		t.Fatal(err)
	}
	set, err := store.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.STACBrowserURL = "https://browser.moregeo.it/external/"
	if err := store.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{
		Root:    root,
		BaseURL: "/package-r",
	})
	req := httptest.NewRequest(
		http.MethodGet,
		"http://127.0.0.1:8888/api/public/share/my-share/data.txt?preview=true",
		http.NoBody,
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	result := recorder.Result()
	defer result.Body.Close()
	var file struct {
		STACBrowserURL string `json:"stacBrowserURL"`
	}
	if err := json.NewDecoder(result.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	want := "https://browser.moregeo.it/external/http://127.0.0.1:8888/package-r/api/public/catalog/my-share/data.txt"
	if file.STACBrowserURL != want {
		t.Fatalf("expected STAC Browser URL %q, got %q", want, file.STACBrowserURL)
	}
}

func TestPublicShareDoesNotReturnSTACBrowserURLWithoutCatalog(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	writePresignTestFile(t, root, "files/data.txt")
	if err := store.Share.Save(&share.Link{Hash: "my-share", Path: "/files", UserID: user.ID}); err != nil {
		t.Fatal(err)
	}
	set, err := store.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.STACBrowserURL = "https://browser.moregeo.it/external/"
	if err := store.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(
		http.MethodGet,
		"http://127.0.0.1:8888/api/public/share/my-share/data.txt?preview=true",
		http.NoBody,
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	result := recorder.Result()
	defer result.Body.Close()
	var file struct {
		STACBrowserURL string `json:"stacBrowserURL"`
	}
	if err := json.NewDecoder(result.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.STACBrowserURL != "" {
		t.Fatalf("expected no STAC Browser URL, got %q", file.STACBrowserURL)
	}
}

func TestAuthenticateShareRequest(t *testing.T) {
	t.Parallel()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("my-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	passwordBcrypt := string(passwordHash)
	testCases := map[string]struct {
		share          *share.Link
		req            *http.Request
		expectedStatus int
	}{
		"Public share, no auth required": {
			share: &share.Link{Hash: "h", UserID: 1},
			req:   newHTTPRequest(t),
		},
		"Private share, no auth provided, 401": {
			share:          &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:            newHTTPRequest(t),
			expectedStatus: http.StatusUnauthorized,
		},
		"Private share, authentication via token": {
			share: &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:   newHTTPRequest(t, func(r *http.Request) { r.URL.RawQuery = "token=123" }),
		},
		"Private share, authentication via invalid token, 401": {
			share:          &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:            newHTTPRequest(t, func(r *http.Request) { r.URL.RawQuery = "token=wrong-token" }),
			expectedStatus: http.StatusUnauthorized,
		},
		"Private share, authentication via password": {
			share: &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:   newHTTPRequest(t, func(r *http.Request) { r.Header.Set("X-SHARE-PASSWORD", "my-password") }),
		},
		"Private share, authentication via invalid password, 401": {
			share:          &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:            newHTTPRequest(t, func(r *http.Request) { r.Header.Set("X-SHARE-PASSWORD", "wrong-password") }),
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			status, err := authenticateShareRequest(tc.req, tc.share)
			if err != nil {
				t.Fatal(err)
			}
			if status != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, status)
			}
		})
	}
}

func newHTTPRequest(t *testing.T, requestModifiers ...func(*http.Request)) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "h", http.NoBody)
	if err != nil {
		t.Fatalf("failed to construct request: %v", err)
	}
	for _, modify := range requestModifiers {
		modify(r)
	}
	return r
}
