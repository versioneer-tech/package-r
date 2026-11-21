package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/mholt/archives"
	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/rules"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

func TestRawGetRequiresPermissionAndLogsDelivery(t *testing.T) {
	store := afero.NewMemMapFs()
	if err := afero.WriteFile(store, "/test.txt", []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	d := &data{
		settings: &settings.Settings{},
		server:   &settings.Server{},
		user: &users.User{
			Username: "test-user",
			Fs:       store,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/test.txt", http.NoBody)
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	status, err := rawGetHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err != nil {
		t.Fatalf("expected denied raw read, status=%d err=%v", status, err)
	}
	if strings.Contains(output.String(), "[DOWNLOAD]") {
		t.Fatalf("denied raw read produced a download log: %s", output.String())
	}

	d.user.Perm.Download = true
	recorder := httptest.NewRecorder()
	status, err = rawGetHandler(recorder, req, d)
	if status != 0 || err != nil {
		t.Fatalf("expected permitted raw read, status=%d err=%v", status, err)
	}
	if recorder.Body.String() != "test" {
		t.Fatalf("unexpected body %q", recorder.Body.String())
	}
	if !strings.Contains(output.String(), `[DOWNLOAD] user="test-user" path="/test.txt" delivery=package_r`) {
		t.Fatalf("missing download audit log: %s", output.String())
	}
}

func TestRawDirectoryArchives(t *testing.T) {
	for algorithm, extension := range map[string]string{
		"zip": ".zip", "tar": ".tar", "targz": ".tar.gz", "tarbz2": ".tar.bz2",
		"tarxz": ".tar.xz", "tarlz4": ".tar.lz4", "tarsz": ".tar.sz",
	} {
		t.Run(algorithm, func(t *testing.T) {
			store := afero.NewMemMapFs()
			content := strings.Repeat("test first", 1024)
			for name, content := range map[string]string{
				"/test/first.txt":         content,
				"/test/nested/second.txt": "test second",
				"/test/blocked/third.txt": "test blocked",
			} {
				if err := store.MkdirAll(path.Dir(name), 0755); err != nil {
					t.Fatal(err)
				}
				if err := afero.WriteFile(store, name, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.MkdirAll("/test/empty", 0755); err != nil {
				t.Fatal(err)
			}
			d := &data{
				settings: &settings.Settings{},
				user:     &users.User{Fs: store, Rules: []rules.Rule{{Path: "/test/blocked", Allow: false}}},
			}
			r := httptest.NewRequest("GET", "/test?algo="+algorithm, http.NoBody)
			w := httptest.NewRecorder()
			status, err := rawDirHandler(w, r, d, &files.FileInfo{Path: "/test", Name: "test"})
			if err != nil || status != 0 {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if algorithm != "tar" && w.Body.Len() >= len(content) {
				t.Fatalf("archive did not compress the input: %d bytes", w.Body.Len())
			}
			if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "test"+extension) {
				t.Fatalf("unexpected disposition: %s", got)
			}
			format, input, err := archives.Identify(r.Context(), "test"+extension, bytes.NewReader(w.Body.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			extractor, ok := format.(archives.Extractor)
			if !ok {
				t.Fatalf("format %T cannot extract", format)
			}
			got := map[string]string{}
			err = extractor.Extract(r.Context(), input, func(_ context.Context, file archives.FileInfo) error {
				name := strings.TrimSuffix(file.NameInArchive, "/")
				if file.IsDir() {
					got[name] = ""
					return nil
				}
				reader, localErr := file.Open()
				if localErr != nil {
					return localErr
				}
				defer func() {
					if closeErr := reader.Close(); closeErr != nil {
						t.Error(closeErr)
					}
				}()
				content, localErr := io.ReadAll(reader)
				got[name] = string(content)
				return localErr
			})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"first.txt": content, "nested": "", "nested/second.txt": "test second", "empty": ""}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("archive contents: got %v, want %v", got, want)
			}
		})
	}
}

func TestCollectArchiveFilesRejectsCancellationAndOutsidePaths(t *testing.T) {
	store := afero.NewMemMapFs()
	if err := afero.WriteFile(store, "/test.txt", []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	d := &data{settings: &settings.Settings{}, user: &users.User{Fs: store}}
	var entries []archives.FileInfo
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := collectArchiveFiles(ctx, d, "/test.txt", "/", &entries); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if err := collectArchiveFiles(context.Background(), d, "/test.txt", "/other", &entries); err == nil {
		t.Fatal("expected outside path to fail")
	}
	if len(entries) != 0 {
		t.Fatal("rejected paths were added to the archive")
	}
}
