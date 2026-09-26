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

func TestConfiguredShareGetsReturnsSafeLinksForUserScope(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	user.Scope = "/team/xyz"
	if err := store.Users.Update(user, "Scope"); err != nil {
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
	if err := store.Share.Save(&share.Link{
		Hash:   "other-share",
		Path:   "/team/other",
		UserID: user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	token := newTestAuthToken(t, store, user)
	handler := handle(configuredShareGetsHandler, "/api/shares", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8888/api/shares", http.NoBody)
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
	if links[0].Hash != "xyz-share" || links[0].URL != "/share/xyz-share/" {
		t.Fatalf("unexpected configured share: %#v", links[0])
	}
}

func TestConfiguredSharesForUserAppliesRules(t *testing.T) {
	links := []*share.Link{
		{Hash: "allowed", Path: "/team/xyz/public"},
		{Hash: "denied", Path: "/team/xyz/private"},
	}

	configured := configuredSharesForUser(links, "/team/xyz", func(requestPath string) bool {
		return requestPath != "/private"
	})
	if len(configured) != 1 || configured[0].Hash != "allowed" {
		t.Fatalf("unexpected configured shares: %#v", configured)
	}
}
