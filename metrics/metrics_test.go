package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/versioneer-tech/package-r/rclonefs"
)

type staticVFSStats struct {
	stats rclonefs.Stats
}

func (s staticVFSStats) Stats() rclonefs.Stats {
	return s.stats
}

func TestMetricsExposeAggregateValuesWithoutRequestPaths(t *testing.T) {
	telemetry := New(context.Background(), staticVFSStats{stats: rclonefs.Stats{
		CacheBytes:        1024,
		ErroredFiles:      2,
		UploadsInProgress: 3,
		UploadsQueued:     4,
		OutOfSpace:        true,
	}})
	telemetry.ObserveLogin("proxy", true)
	telemetry.ObserveLogin("proxy", false)
	telemetry.ObserveTokenRenewal()
	telemetry.ObserveTUS("started")
	telemetry.ObserveTUS("completed")
	telemetry.ObserveTUS("aborted")
	telemetry.ObserveTUS("failed")
	telemetry.ObservePresign(true)
	telemetry.ObservePresign(false)

	router := mux.NewRouter()
	router.Use(telemetry.HTTPMiddleware)
	router.HandleFunc("/objects/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}).Methods(http.MethodPost)
	router.Handle("/metrics", telemetry.Handler()).Methods(http.MethodGet)

	request := httptest.NewRequest(http.MethodPost, "/objects/private-object-name", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected response status %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	output := string(body)

	expected := []string{
		`package_r_http_requests_total{method="POST",route="/objects/{id}",status="201"} 1`,
		`package_r_auth_logins_total{method="proxy",result="success"} 1`,
		`package_r_auth_logins_total{method="proxy",result="failure"} 1`,
		`package_r_auth_token_renewals_total 1`,
		`package_r_tus_uploads_total{result="started"} 1`,
		`package_r_tus_uploads_total{result="completed"} 1`,
		`package_r_tus_uploads_total{result="aborted"} 1`,
		`package_r_tus_uploads_total{result="failed"} 1`,
		`package_r_presign_requests_total{result="success"} 1`,
		`package_r_presign_requests_total{result="failure"} 1`,
		`package_r_vfs_cache_bytes 1024`,
		`package_r_vfs_cache_errored_files 2`,
		`package_r_vfs_uploads_active 3`,
		`package_r_vfs_uploads_queued 4`,
		`package_r_vfs_cache_out_of_space 1`,
		`rclone_bytes_transferred_total`,
		`rclone_errors_total`,
	}
	for _, value := range expected {
		if !strings.Contains(output, value) {
			t.Errorf("metrics output does not contain %q", value)
		}
	}
	if strings.Contains(output, "private-object-name") {
		t.Fatal("metrics output contains a request path value")
	}
}
