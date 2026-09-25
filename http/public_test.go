package http

import (
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
)

func TestPublicShareBypassesGeneratedUserDirBaseRules(t *testing.T) {
	root := t.TempDir()
	writePresignTestFile(t, root, "home/bob/data.txt")

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

	set := &settings.Settings{
		Key: []byte("test-key"),
	}
	if err := store.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	user := &users.User{Username: "alice", Password: "password", Scope: "/"}
	set.ApplyUserDefaults(user)
	if err := store.Users.Save(user); err != nil {
		t.Fatal(err)
	}

	if err := store.Share.Save(&share.Link{
		Hash:   "public-share",
		Path:   "/home/bob",
		UserID: user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	handler := handle(publicShareHandler, "/api/public/share/", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/public/share/public-share/data.txt", http.NoBody)
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

func TestAuthenticateShareRequest(t *testing.T) {
	t.Parallel()

	const passwordBcrypt = "$2y$10$TFAmdCbyd/mEZDe5fUeZJu.MaJQXRTwdqb/IQV.eTn6dWrF58gCSe" //nolint:gosec
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
			req:            newHTTPRequest(t, func(r *http.Request) { r.URL.RawQuery = "token=1234" }),
			expectedStatus: http.StatusUnauthorized,
		},
		"Private share, authentication via password": {
			share: &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:   newHTTPRequest(t, func(r *http.Request) { r.Header.Set("X-SHARE-PASSWORD", "password") }),
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
