package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/versioneer-tech/package-r/settings"
)

func TestProfilePatchUpdatesOnlyTheAuthenticatedUserPreferences(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	token := newTestAuthToken(t, store, user)
	handler := handle(profilePatchHandler, "", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodPatch, "http://localhost:8888/api/profile", strings.NewReader(`{
		"locale":"de",
		"viewMode":"mosaic",
		"singleClick":true,
		"sorting":{"by":"modified","asc":true},
		"hideDotfiles":true,
		"dateFormat":true
	}`))
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", recorder.Code, recorder.Body.String())
	}

	updated, err := store.Users.Get(root, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Locale != "de" || updated.ViewMode != "mosaic" || !updated.SingleClick ||
		updated.Sorting.By != "modified" || !updated.Sorting.Asc ||
		!updated.HideDotfiles || !updated.DateFormat {
		t.Fatalf("preferences were not stored: %+v", updated)
	}
	if updated.Username != user.Username || updated.Scope != user.Scope || updated.Perm != user.Perm {
		t.Fatal("profile update changed protected user fields")
	}
}

func TestProfilePatchRejectsUserManagementFields(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	token := newTestAuthToken(t, store, user)
	handler := handle(profilePatchHandler, "", store, &settings.Server{Root: root})
	req := httptest.NewRequest(http.MethodPatch, "http://localhost:8888/api/profile", strings.NewReader(`{
		"perm":{"create":true}
	}`))
	req.Header.Set("X-Auth", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestProfilePatchRejectsInvalidPreferences(t *testing.T) {
	root, store, user := newPresignTestStorage(t)
	token := newTestAuthToken(t, store, user)
	handler := handle(profilePatchHandler, "", store, &settings.Server{Root: root})

	for name, body := range map[string]string{
		"empty":        `{}`,
		"view mode":    `{"viewMode":"tiles"}`,
		"sorting mode": `{"sorting":{"by":"random","asc":true}}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "http://localhost:8888/api/profile", strings.NewReader(body))
			req.Header.Set("X-Auth", token)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", recorder.Code)
			}
		})
	}
}
