package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsersAddCreatesGeneratedUserDirRootScope(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath, "--create-user-dir", "--scope", "/")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "alice", "password")

	user, err := openTestStorage(t, dbPath).Users.Get(rootPath, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Scope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", user.Scope)
	}
	assertUserBrowseRoot(t, user.FullPath("/"), rootPath)
	if _, err := os.Stat(filepath.Join(rootPath, "home", "alice", ".keep")); err != nil {
		t.Fatal(err)
	}
}

func TestUsersAddCreatesGeneratedUserDirHomeScope(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath, "--create-user-dir", "--scope", ".")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "alice", "password")

	user, err := openTestStorage(t, dbPath).Users.Get(rootPath, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Scope != "/home/alice" {
		t.Fatalf("expected generated user to use home scope, got %q", user.Scope)
	}
	assertUserBrowseRoot(t, user.FullPath("/"), filepath.Join(rootPath, "home", "alice"))
	if _, err := os.Stat(filepath.Join(rootPath, "home", "alice", ".keep")); err != nil {
		t.Fatal(err)
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
