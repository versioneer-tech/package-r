package cmd

import (
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
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

func TestUsersAddReconcilesExistingUser(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "old-password")

	runPackageRCommand(t,
		"--config", configPath,
		"--database", dbPath,
		"users", "add", "admin", "new-password",
		"--perm.create=true",
		"--locale=de",
	)

	store := openTestStorage(t, dbPath)
	updated, err := store.Users.Get(rootPath, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != 1 {
		t.Fatalf("expected the original user ID to stay unchanged, got %d", updated.ID)
	}
	all, err := store.Users.Gets(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one reconciled user, got %d", len(all))
	}
	if !updated.Perm.Create || updated.Locale != "de" {
		t.Fatalf("expected provided user configuration to be reconciled, got %#v", updated)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("new-password")); err != nil {
		t.Fatalf("expected reconciled password: %v", err)
	}
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
