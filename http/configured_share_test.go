package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
)

func TestConfiguredShareGetsIsReadOnlyAndDoesNotRequireSharePermission(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	user.Scope = "/team/xyz"
	user.Perm.Share = false
	if err := store.Users.Update(user, "Scope", "Perm"); err != nil {
		t.Fatal(err)
	}
	if err := store.Share.Save(&share.Link{
		Hash:         "xyz-share",
		Path:         "/team/xyz/public",
		UserID:       user.ID,
		Description:  "configured share",
		PasswordHash: "must-not-be-returned",
		Token:        "must-not-be-returned",
	}); err != nil {
		t.Fatal(err)
	}

	token := newTestAuthToken(t, store, user)
	handler := handle(configuredShareGetsHandler, "/api/share", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/share/public/report.txt", http.NoBody)
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "must-not-be-returned") {
		t.Fatal("read-only share response exposed a secret field")
	}

	var links []configuredShare
	if err := json.NewDecoder(recorder.Body).Decode(&links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected one configured share, got %d", len(links))
	}
	if links[0].Hash != "xyz-share" || links[0].URL != "/share/xyz-share/report.txt" {
		t.Fatalf("unexpected configured share: %#v", links[0])
	}
}

func TestConfiguredSharesForPathIncludesEnclosingShares(t *testing.T) {
	links := []*share.Link{
		{Hash: "xyz-public", Path: "/public"},
		{Hash: "xyz-other", Path: "/other"},
	}

	configured := configuredSharesForPath(links, "/public/folder", true)
	if len(configured) != 1 {
		t.Fatalf("expected one configured share, got %d", len(configured))
	}
	if configured[0].URL != "/share/xyz-public/folder/" {
		t.Fatalf("unexpected public URL %q", configured[0].URL)
	}
}
