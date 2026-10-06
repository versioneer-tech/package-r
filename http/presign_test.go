package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
		url:   "https://objects.example.invalid/data.tif?signature=xyz",
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

func TestPublicSharePresignConcurrentHierarchy(t *testing.T) {
	const (
		levels          = 8
		sharePassword   = "test-password"
		publicLinkDelay = 20 * time.Millisecond
		retryDelay      = 25 * time.Millisecond
		testDeadline    = 31 * time.Second
		scatterInterval = 30 * time.Second
	)
	workloads := []struct {
		name     string
		requests int
	}{
		{name: "w1000", requests: 1000},
		{name: "w10000", requests: 10000},
	}

	root, store, user := newPresignTestStorage(t)
	paths := make([]string, 0, levels*levels*levels)
	for first := 1; first <= levels; first++ {
		for second := 1; second <= levels; second++ {
			for third := 1; third <= levels; third++ {
				relativePath := fmt.Sprintf("%d/%d/%d.txt", first, second, third)
				content := fmt.Sprintf("test%d%d%d", first, second, third)
				writePresignTestContent(t, root, filepath.Join("files", relativePath), content)
				paths = append(paths, relativePath)
			}
		}
	}
	for relativePath, expected := range map[string]string{"1/2/3.txt": "test123", "3/5/2.txt": "test352"} {
		content, err := os.ReadFile(filepath.Join(root, "files", relativePath))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != expected {
			t.Fatalf("unexpected content for %s: %q", relativePath, content)
		}
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(sharePassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Share.Save(&share.Link{
		Hash:         "my-share",
		Path:         "/files",
		UserID:       user.ID,
		PasswordHash: string(passwordHash),
		Token:        "test-token",
	}); err != nil {
		t.Fatal(err)
	}

	linker := &concurrentPublicLinkStore{Store: store.Users, delay: publicLinkDelay}
	store.Users = linker
	telemetry := appmetrics.New(context.Background(), nil)
	applicationSettings, err := store.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}

	totalPublicLinks := 0
	for _, presignLimit := range []int{32, 8} {
		server := &settings.Server{Root: root, PublicPresignConcurrency: presignLimit}
		handler := newPublicShareHandler(server)
		request := func(relativePath string) (int, time.Duration, error) {
			started := time.Now()
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodGet,
				"http://localhost/my-share/"+relativePath+"?presign=true&follow=true",
				http.NoBody,
			)
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/")
			req.Header.Set("X-SHARE-PASSWORD", url.QueryEscape(sharePassword))
			status, err := handler(recorder, req, &data{store: store, server: server, settings: applicationSettings, metrics: telemetry})
			return status, time.Since(started), err
		}

		if status, _, err := request(paths[0]); status != http.StatusTemporaryRedirect || err != nil {
			t.Fatalf("limit=%d: warm-up request returned status=%d err=%v", presignLimit, status, err)
		}
		totalPublicLinks++
		for _, workload := range workloads {
			logicalRequests := workload.requests
			linker.reset()

			random := rand.New(rand.NewSource(int64(logicalRequests + presignLimit))) //nolint:gosec
			results := make(chan presignResult, logicalRequests)
			start := make(chan struct{})
			var requests sync.WaitGroup
			for index := range logicalRequests {
				delay := time.Duration(random.Int63n(int64(scatterInterval)))
				requests.Add(1)
				go func() {
					defer requests.Done()
					<-start
					time.Sleep(delay)
					result := presignResult{}
					for {
						result.attempts++
						result.status, result.duration, result.err = request(paths[index%len(paths)])
						if result.status != http.StatusTooManyRequests || result.err != nil {
							break
						}
						result.throttled++
						time.Sleep(retryDelay)
					}
					results <- result
				}()
			}
			started := time.Now()
			close(start)
			requests.Wait()
			totalDuration := time.Since(started)
			close(results)

			acceptedDurations := make(chan time.Duration, logicalRequests)
			totalAttempts := 0
			throttled := 0
			errors := 0
			for result := range results {
				totalAttempts += result.attempts
				throttled += result.throttled
				if result.err != nil || result.status != http.StatusTemporaryRedirect {
					errors++
					continue
				}
				acceptedDurations <- result.duration
			}
			close(acceptedDurations)
			accepted := collectDurations(acceptedDurations)
			linkStats := linker.stats()
			if accepted.count != logicalRequests || errors != 0 {
				t.Fatalf("accepted=%d errors=%d", accepted.count, errors)
			}
			if linkStats.maxInFlight > presignLimit {
				t.Fatalf("max in flight=%d, limit=%d", linkStats.maxInFlight, presignLimit)
			}
			if totalDuration >= testDeadline {
				t.Fatalf("workload exceeded %s: %s", testDeadline, totalDuration)
			}
			totalPublicLinks += accepted.count
			t.Logf(
				"presign workload: name=%s limit=%d downloads=%d client_attempts=%d throttled=%d errors=%d total=%s presign_min=%s presign_avg=%s presign_max=%s max_in_flight=%d",
				workload.name, presignLimit, logicalRequests, totalAttempts, throttled, errors, totalDuration,
				accepted.min, accepted.average(), accepted.max, linkStats.maxInFlight,
			)
		}
	}

	metricsRecorder := httptest.NewRecorder()
	telemetry.Handler().ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	metricsBody, err := io.ReadAll(metricsRecorder.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	expectedCount := fmt.Sprintf("package_r_presign_stage_duration_seconds_count{stage=\"public_link\"} %d", totalPublicLinks)
	if !strings.Contains(string(metricsBody), expectedCount) {
		t.Fatalf("metrics do not contain %q", expectedCount)
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
	linker := &recordingPublicLinkStore{
		Store: store.Users,
		url:   "https://objects.example.invalid/data.txt?signature=xyz",
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
	defer result.Body.Close()
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

type durationStats struct {
	count       int
	min         time.Duration
	max         time.Duration
	total       time.Duration
	maxInFlight int
}

type presignResult struct {
	status    int
	duration  time.Duration
	err       error
	attempts  int
	throttled int
}

func (s durationStats) average() time.Duration {
	if s.count == 0 {
		return 0
	}
	return s.total / time.Duration(s.count)
}

func collectDurations(durations <-chan time.Duration) durationStats {
	var stats durationStats
	for duration := range durations {
		stats.count++
		stats.total += duration
		if stats.min == 0 || duration < stats.min {
			stats.min = duration
		}
		if duration > stats.max {
			stats.max = duration
		}
	}
	return stats
}

type concurrentPublicLinkStore struct {
	users.Store
	delay time.Duration

	active atomic.Int64
	mu     sync.Mutex
	statsV durationStats
}

func (s *concurrentPublicLinkStore) PublicLink(ctx context.Context, _ *users.User, _ string, _ time.Duration) (string, error) {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	started := time.Now()
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	duration := time.Since(started)

	s.mu.Lock()
	s.statsV.count++
	s.statsV.total += duration
	if s.statsV.min == 0 || duration < s.statsV.min {
		s.statsV.min = duration
	}
	if duration > s.statsV.max {
		s.statsV.max = duration
	}
	if int(active) > s.statsV.maxInFlight {
		s.statsV.maxInFlight = int(active)
	}
	s.mu.Unlock()
	return "https://objects.example.invalid/data.txt", nil
}

func (s *concurrentPublicLinkStore) reset() {
	s.mu.Lock()
	s.statsV = durationStats{}
	s.mu.Unlock()
}

func (s *concurrentPublicLinkStore) stats() durationStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statsV
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
