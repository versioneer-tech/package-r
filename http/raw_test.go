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
	if err := afero.WriteFile(store, "/xyz.txt", []byte("xyz"), 0644); err != nil {
		t.Fatal(err)
	}
	d := &data{
		settings: &settings.Settings{},
		server:   &settings.Server{},
		user: &users.User{
			Username: "xyz-user",
			Fs:       store,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/xyz.txt", http.NoBody)

	status, err := rawGetHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err != nil {
		t.Fatalf("expected denied raw read, status=%d err=%v", status, err)
	}

	d.user.Perm.Download = true
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousOutput) })
	recorder := httptest.NewRecorder()
	status, err = rawGetHandler(recorder, req, d)
	if status != 0 || err != nil {
		t.Fatalf("expected permitted raw read, status=%d err=%v", status, err)
	}
	if recorder.Body.String() != "xyz" {
		t.Fatalf("unexpected body %q", recorder.Body.String())
	}
	if !strings.Contains(output.String(), `[DOWNLOAD] user="xyz-user" path="/xyz.txt" delivery=package_r`) {
		t.Fatalf("missing download audit log: %s", output.String())
	}
}

func TestRawDirectoryArchives(t *testing.T) {
	for _, algorithm := range []string{"zip", "tar", "targz", "tarbz2", "tarxz", "tarlz4", "tarsz"} {
		t.Run(algorithm, func(t *testing.T) {
			store := afero.NewMemMapFs()
			content := strings.Repeat("xyz first", 1024)
			for name, content := range map[string]string{
				"/xyz/first.txt":         content,
				"/xyz/nested/second.txt": "xyz second",
				"/xyz/blocked/third.txt": "xyz blocked",
			} {
				if err := store.MkdirAll(path.Dir(name), 0755); err != nil {
					t.Fatal(err)
				}
				if err := afero.WriteFile(store, name, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.MkdirAll("/xyz/empty", 0755); err != nil {
				t.Fatal(err)
			}
			d := &data{
				settings: &settings.Settings{},
				user:     &users.User{Fs: store, Rules: []rules.Rule{{Path: "/xyz/blocked", Allow: false}}},
			}
			r := httptest.NewRequest("GET", "/xyz?algo="+algorithm, nil)
			w := httptest.NewRecorder()
			status, err := rawDirHandler(w, r, d, &files.FileInfo{Path: "/xyz", Name: "xyz"})
			if err != nil || status != 0 {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if algorithm != "tar" && w.Body.Len() >= len(content) {
				t.Fatalf("archive did not compress the input: %d bytes", w.Body.Len())
			}
			extension, _, err := parseQueryAlgorithm(r)
			if err != nil {
				t.Fatal(err)
			}
			if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "xyz"+extension) {
				t.Fatalf("unexpected disposition: %s", got)
			}
			format, input, err := archives.Identify(r.Context(), "xyz"+extension, bytes.NewReader(w.Body.Bytes()))
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
				reader, err := file.Open()
				if err != nil {
					return err
				}
				defer reader.Close()
				content, err := io.ReadAll(reader)
				got[name] = string(content)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"first.txt": content, "nested": "", "nested/second.txt": "xyz second", "empty": ""}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("archive contents: got %v, want %v", got, want)
			}
		})
	}
}

func TestCollectArchiveFilesRejectsCancellationAndOutsidePaths(t *testing.T) {
	store := afero.NewMemMapFs()
	if err := afero.WriteFile(store, "/xyz.txt", []byte("xyz"), 0644); err != nil {
		t.Fatal(err)
	}
	d := &data{settings: &settings.Settings{}, user: &users.User{Fs: store}}
	var entries []archives.FileInfo
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := collectArchiveFiles(ctx, d, "/xyz.txt", "/", &entries); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if err := collectArchiveFiles(context.Background(), d, "/xyz.txt", "/other", &entries); err == nil {
		t.Fatal("expected outside path to fail")
	}
	if len(entries) != 0 {
		t.Fatal("rejected paths were added to the archive")
	}
}
