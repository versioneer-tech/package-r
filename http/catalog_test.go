package http

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/users"
)

func TestRemoteCatalogURLDoesNotOutliveShare(t *testing.T) {
	memoryFS := afero.NewMemMapFs()
	if err := afero.WriteFile(memoryFS, "catalog.parquet", []byte("catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	linker := &recordingPublicLinkStore{url: "https://objects.example.invalid/catalog.parquet?signature=my-secret"}
	expire := time.Now().Add(time.Minute).Unix()

	got, release, err := remoteCatalogURL(
		context.Background(),
		linker,
		&users.User{Scope: "/team/alice"},
		memoryFS,
		"/public",
		"catalog.parquet",
		expire,
	)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()

	if got != linker.url {
		t.Fatalf("expected signed URL %q, got %q", linker.url, got)
	}
	if linker.name != "/public/catalog.parquet" {
		t.Fatalf("unexpected signed catalog path %q", linker.name)
	}
	if linker.expire <= 50*time.Second || linker.expire > time.Minute {
		t.Fatalf("expected signed URL to expire with the share, got %v", linker.expire)
	}
}

func TestRemoteCatalogURLRejectsOversizeCatalog(t *testing.T) {
	memoryFS := afero.NewMemMapFs()
	if err := afero.WriteFile(memoryFS, "catalog.parquet", []byte("catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	fsys := catalogSizeFS{Fs: memoryFS, size: maxRemoteCatalogSize + 1}
	linker := &recordingPublicLinkStore{url: "https://objects.example.invalid/catalog.parquet"}
	slotsBefore := len(catalogQuerySlots)

	_, release, err := remoteCatalogURL(
		context.Background(),
		linker,
		&users.User{},
		fsys,
		"/public",
		"catalog.parquet",
		0,
	)
	if err == nil {
		t.Fatal("expected an oversize catalog error")
	}
	if release != nil {
		t.Fatal("expected no release callback for a rejected catalog")
	}
	if slotsAfter := len(catalogQuerySlots); slotsAfter != slotsBefore {
		t.Fatalf("expected the query slot to be released, before=%d after=%d", slotsBefore, slotsAfter)
	}
	if linker.name != "" {
		t.Fatalf("expected no signed URL request, got path %q", linker.name)
	}
}

type catalogSizeFS struct {
	afero.Fs
	size int64
}

func (fsys catalogSizeFS) Stat(name string) (os.FileInfo, error) {
	info, err := fsys.Fs.Stat(name)
	if err != nil {
		return nil, err
	}
	return catalogSizeInfo{FileInfo: info, size: fsys.size}, nil
}

type catalogSizeInfo struct {
	os.FileInfo
	size int64
}

func (info catalogSizeInfo) Size() int64 {
	return info.size
}

func TestCatalogPathInShare(t *testing.T) {
	tests := []struct {
		name       string
		catalogURL string
		sharePath  string
		want       string
		wantError  bool
	}{
		{
			name:       "logical path",
			catalogURL: "/public/catalog.parquet",
			sharePath:  "/public",
			want:       "catalog.parquet",
		},
		{
			name:       "nested logical path",
			catalogURL: "/public/meta/catalog.parquet",
			sharePath:  "/public",
			want:       "meta/catalog.parquet",
		},
		{
			name:       "outside logical path",
			catalogURL: "/private/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
		{
			name:       "sibling prefix",
			catalogURL: "/publicity/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
		{
			name:       "traversal",
			catalogURL: "/public/../private/catalog.parquet",
			sharePath:  "/public",
			wantError:  true,
		},
		{
			name:       "catalog equals share",
			catalogURL: "/public",
			sharePath:  "/public",
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := catalogPathInShare(test.catalogURL, test.sharePath)
			if test.wantError {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
