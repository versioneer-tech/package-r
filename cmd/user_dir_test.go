package cmd

import (
	"path/filepath"
	"testing"
)

func TestUsersAddDefaultsGeneratedUserDirToRootScope(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath, "--create-user-dir")
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "alice", "my-password")

	user, err := openTestStorage(t, dbPath).Users.Get(rootPath, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Scope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", user.Scope)
	}
	assertUserBrowseRoot(t, user.FullPath("/"), rootPath)
}

func TestUsersAddSetsSupportedPermissionsExplicitly(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t,
		"--config", configPath,
		"--database", dbPath,
		"users", "add", "initial", "my-password",
		"--perm.create=true",
		"--perm.rename=true",
		"--perm.modify=true",
		"--perm.delete=true",
	)

	user, err := openTestStorage(t, dbPath).Users.Get(rootPath, "initial")
	if err != nil {
		t.Fatal(err)
	}
	permissions := user.Perm
	if !permissions.Create || !permissions.Rename ||
		!permissions.Modify || !permissions.Delete {
		t.Fatalf("expected each supported permission to be enabled, got %#v", permissions)
	}
	if permissions.Execute || permissions.Download {
		t.Fatalf("expected internal permissions to stay disabled, got %#v", permissions)
	}
}

func assertUserBrowseRoot(t *testing.T, got, want string) {
	t.Helper()

	got = filepath.Clean(got)
	want = filepath.Clean(want)
	if got != want {
		t.Fatalf("expected user browse root %q, got %q", want, got)
	}
}
