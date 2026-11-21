//go:build loadtest

package http

import (
	"context"
	"fmt"
	"io"
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

	"golang.org/x/crypto/bcrypt"

	appmetrics "github.com/versioneer-tech/package-r/metrics"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/users"
)

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
	if saveErr := store.Share.Save(&share.Link{
		Hash:         "my-share",
		Path:         "/files",
		UserID:       user.ID,
		PasswordHash: string(passwordHash),
		Token:        "test-token",
	}); saveErr != nil {
		t.Fatal(saveErr)
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
			status, requestErr := handler(recorder, req, &data{store: store, server: server, settings: applicationSettings, metrics: telemetry})
			return status, time.Since(started), requestErr
		}

		if status, _, requestErr := request(paths[0]); status != http.StatusTemporaryRedirect || requestErr != nil {
			t.Fatalf("limit=%d: warm-up request returned status=%d err=%v", presignLimit, status, requestErr)
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
