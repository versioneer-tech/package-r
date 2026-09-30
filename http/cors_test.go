package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddlewareIsDisabledWithoutOrigins(t *testing.T) {
	handler := corsMiddleware("", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "http://package-r.example/api/public/share/demo", http.NoBody)
	req.Header.Set("Origin", "https://viewer.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected CORS to be disabled, got origin %q", got)
	}
}

func TestCORSMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	handler := corsMiddleware(
		"https://portal.example, https://viewer.example",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "http://package-r.example/api/resources/", http.NoBody)
	req.Header.Set("Origin", "https://viewer.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected downstream status 401, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://viewer.example" {
		t.Fatalf("expected configured origin, got %q", got)
	}
	if !strings.Contains(recorder.Header().Get("Access-Control-Expose-Headers"), "X-Renew-Token") {
		t.Fatal("expected authentication response header to be exposed")
	}
	if !strings.Contains(strings.Join(recorder.Header().Values("Vary"), ","), "Origin") {
		t.Fatal("expected response to vary by origin")
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected credentialed CORS to remain disabled, got %q", got)
	}
}

func TestCORSMiddlewareHandlesPreflight(t *testing.T) {
	downstreamCalled := false
	handler := corsMiddleware(
		"https://viewer.example",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			downstreamCalled = true
		}),
	)
	req := httptest.NewRequest(http.MethodOptions, "http://package-r.example/api/public/share/demo", http.NoBody)
	req.Header.Set("Origin", "https://viewer.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "X-SHARE-PASSWORD, Range")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if downstreamCalled {
		t.Fatal("expected middleware to handle preflight")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodGet) {
		t.Fatal("expected GET to be allowed")
	}
	for _, header := range []string{"X-SHARE-PASSWORD", "Range", "Upload-Offset"} {
		if !strings.Contains(recorder.Header().Get("Access-Control-Allow-Headers"), header) {
			t.Fatalf("expected %s to be allowed", header)
		}
	}
}

func TestCORSMiddlewareDoesNotAllowUnconfiguredOrigin(t *testing.T) {
	handler := corsMiddleware(
		"https://viewer.example",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "http://package-r.example/api/public/share/demo", http.NoBody)
	req.Header.Set("Origin", "https://other.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected origin to be rejected, got %q", got)
	}
}

func TestCORSMiddlewareSupportsWildcardOrigin(t *testing.T) {
	handler := corsMiddleware("*", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "http://package-r.example/api/public/share/demo", http.NoBody)
	req.Header.Set("Origin", "https://viewer.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin, got %q", got)
	}
}
