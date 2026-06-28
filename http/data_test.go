package http

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

func TestUserDirBaseRulesRestrictOnlySiblingUserDirs(t *testing.T) {
	set := &settings.Settings{}
	user := &users.User{Username: "alice"}
	set.ApplyUserDefaults(user)

	data := &data{
		settings: &settings.Settings{},
		user:     user,
	}

	for path, allowed := range map[string]bool{
		"/catalog.parquet":   true,
		"/home":              true,
		"/home-archive/file": true,
		"/home/alice":        true,
		"/home/alice/file":   true,
		"/home/alice-other":  false,
		"/home/bob/file.txt": false,
	} {
		if got := data.Check(path); got != allowed {
			t.Fatalf("expected Check(%q) to be %t, got %t", path, allowed, got)
		}
	}
}

func TestGeneratedUserDirRulesAllowRootMountedContentAndHideSiblingHomes(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"bucket-data",
		"home/alice",
		"home/bob",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	set := &settings.Settings{}
	user := &users.User{
		Username: "alice",
		Scope:    "/",
		Fs:       afero.NewBasePathFs(afero.NewOsFs(), root),
	}
	set.ApplyUserDefaults(user)

	data := &data{
		settings: set,
		user:     user,
	}

	rootInfo, err := files.NewFileInfo(&files.FileOptions{
		Fs:      user.Fs,
		Path:    "/",
		Expand:  true,
		Checker: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertListingNames(t, rootInfo.Listing, []string{"bucket-data", "home"})

	homeInfo, err := files.NewFileInfo(&files.FileOptions{
		Fs:      user.Fs,
		Path:    "/home",
		Expand:  true,
		Checker: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertListingNames(t, homeInfo.Listing, []string{"alice"})

	_, err = files.NewFileInfo(&files.FileOptions{
		Fs:      user.Fs,
		Path:    "/home/bob",
		Expand:  true,
		Checker: data,
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected sibling home to be denied, got %v", err)
	}
}

func TestAdminBypassesUserDirBaseRules(t *testing.T) {
	set := &settings.Settings{}
	user := &users.User{Username: "admin"}
	set.ApplyUserDefaults(user)
	user.Perm.Admin = true

	data := &data{
		settings: &settings.Settings{},
		user:     user,
	}

	for _, path := range []string{
		"/home/bob/file.txt",
		"/home/alice-other",
	} {
		if !data.Check(path) {
			t.Fatalf("expected admin Check(%q) to be true", path)
		}
	}
}

func assertListingNames(t *testing.T, listing *files.Listing, want []string) {
	t.Helper()

	if listing == nil {
		t.Fatal("expected listing, got nil")
	}
	got := make([]string, 0, len(listing.Items))
	for _, item := range listing.Items {
		got = append(got, item.Name)
	}
	if len(got) != len(want) {
		t.Fatalf("expected listing names %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected listing names %v, got %v", want, got)
		}
	}
}
