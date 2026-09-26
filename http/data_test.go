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
	set := &settings.Settings{CreateUserDir: true}
	user := &users.User{Username: "alice"}
	set.ApplyUserDefaults(user)

	data := &data{
		settings: &settings.Settings{},
		user:     user,
	}

	for path, allowed := range map[string]bool{
		"/catalog.parquet":              true,
		"/home":                         true,
		"/home-archive/file":            true,
		"/home/alice":                   true,
		"/home/alice/file":              true,
		"/home/alice-other":             false,
		"/home/bob/file.txt":            false,
		"/home/alice/../bob/file.txt":   false,
		"/home/alice/../../catalog.txt": true,
	} {
		if got := data.Check(path); got != allowed {
			t.Fatalf("expected Check(%q) to be %t, got %t", path, allowed, got)
		}
	}
}

func TestUserDirIsolationAppliesToExistingUsers(t *testing.T) {
	set := &settings.Settings{CreateUserDir: true}
	data := &data{
		settings: set,
		user:     &users.User{Username: "alice"},
	}

	if data.Check("/home/bob/file.txt") {
		t.Fatal("expected existing user to be denied access to a sibling home")
	}
	if !data.Check("/home/alice/file.txt") {
		t.Fatal("expected existing user to access their own home")
	}
}

func TestUserDirWritesProtectBaseAndSiblingHomes(t *testing.T) {
	set := &settings.Settings{
		CreateUserDir:    true,
		UserHomeBasePath: "/users",
	}
	data := &data{
		settings: set,
		user:     &users.User{Username: "alice"},
	}

	for path, allowed := range map[string]bool{
		"/":                              false,
		"/catalog.parquet":               true,
		"/users":                         false,
		"/users/":                        false,
		"/users/alice":                   true,
		"/users/alice/file.txt":          true,
		"/users/bob/file.txt":            false,
		"/users/alice/../bob/file.txt":   false,
		"/users/alice/../../catalog.txt": true,
		"/users/alice/../..":             false,
	} {
		if got := data.CheckWrite(path); got != allowed {
			t.Fatalf("expected CheckWrite(%q) to be %t, got %t", path, allowed, got)
		}
	}
}

func TestDisabledUserDirDoesNotRestrictHomePaths(t *testing.T) {
	set := &settings.Settings{}
	user := &users.User{Username: "alice"}
	set.ApplyUserDefaults(user)
	data := &data{settings: set, user: user}

	if !data.Check("/home/bob/file.txt") {
		t.Fatal("expected user-dir rules to be disabled")
	}
}

func TestGeneratedUserDirRulesAllowRootObjectContentAndHideSiblingHomes(t *testing.T) {
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

	set := &settings.Settings{CreateUserDir: true}
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
