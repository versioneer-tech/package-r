package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/versioneer-tech/package-r/version"
)

func TestMakeUserDirCreatesKeepMarkerForGeneratedUserDir(t *testing.T) {
	previousVersion := version.Version
	version.Version = "test-version"
	t.Cleanup(func() {
		version.Version = previousVersion
	})

	root := t.TempDir()
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.MakeUserDir("alice", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/home/alice" {
		t.Fatalf("expected generated user scope /home/alice, got %q", userScope)
	}

	content, err := os.ReadFile(filepath.Join(root, "home", "alice", ".keep"))
	if err != nil {
		t.Fatal(err)
	}

	want := "created by package-r vtest-version automatically - please keep!"
	if string(content) != want {
		t.Fatalf("expected .keep marker %q, got %q", want, string(content))
	}
}

func TestMakeUserDirTreatsDefaultScopeAsGeneratedUserDir(t *testing.T) {
	root := t.TempDir()
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.MakeUserDir("alice", ".", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/home/alice" {
		t.Fatalf("expected generated user scope /home/alice, got %q", userScope)
	}

	if _, err := os.Stat(filepath.Join(root, "home", "alice", ".keep")); err != nil {
		t.Fatal(err)
	}
}

func TestMakeUserDirUsesConfiguredUserHomeBasePath(t *testing.T) {
	root := t.TempDir()
	set := Settings{
		CreateUserDir:    true,
		UserHomeBasePath: "users",
	}

	userScope, err := set.MakeUserDir("alice", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/users/alice" {
		t.Fatalf("expected generated user scope /users/alice, got %q", userScope)
	}

	if _, err := os.Stat(filepath.Join(root, "users", "alice", ".keep")); err != nil {
		t.Fatal(err)
	}
}
