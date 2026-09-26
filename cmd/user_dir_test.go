package cmd

import (
	"path/filepath"
	"testing"
)

func TestUsersAddCreatesGeneratedUserDirRootScope(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath, "--create-user-dir", "--scope", "/")
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

func TestUsersAddCreatesGeneratedUserDirHomeScope(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath, "--create-user-dir", "--scope", ".")
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "alice", "my-password")

	user, err := openTestStorage(t, dbPath).Users.Get(rootPath, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Scope != "/home/alice" {
		t.Fatalf("expected generated user to use home scope, got %q", user.Scope)
	}
	assertUserBrowseRoot(t, user.FullPath("/"), filepath.Join(rootPath, "home", "alice"))
}

func TestUsersAddSetsEachPermissionExplicitly(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t,
		"--config", configPath,
		"--database", dbPath,
		"users", "add", "initial", "my-password",
		"--perm.execute=true",
		"--perm.create=true",
		"--perm.rename=true",
		"--perm.modify=true",
		"--perm.delete=true",
		"--perm.download=true",
	)

	user, err := openTestStorage(t, dbPath).Users.Get(rootPath, "initial")
	if err != nil {
		t.Fatal(err)
	}
	permissions := user.Perm
	if !permissions.Execute || !permissions.Create || !permissions.Rename ||
		!permissions.Modify || !permissions.Delete || !permissions.Download {
		t.Fatalf("expected every explicit permission to be enabled, got %#v", permissions)
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
