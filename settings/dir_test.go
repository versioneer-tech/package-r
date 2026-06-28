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
	if userScope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", userScope)
	}

	content, err := os.ReadFile(filepath.Join(root, "home", "alice", ".keep"))
	if err != nil {
		t.Fatal(err)
	}

	want := "created by package-r test-version automatically - please keep!"
	if string(content) != want {
		t.Fatalf("expected .keep marker %q, got %q", want, string(content))
	}
}

func TestMakeUserDirTreatsDotScopeAsLegacyHomeOnlyScope(t *testing.T) {
	root := t.TempDir()
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.MakeUserDir("alice", ".", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/home/alice" {
		t.Fatalf("expected legacy dot scope to use generated home scope, got %q", userScope)
	}

	if _, err := os.Stat(filepath.Join(root, "home", "alice", ".keep")); err != nil {
		t.Fatal(err)
	}
}

func TestMakeUserDirTreatsDotScopeAsLegacyHomeOnlyScopeWithConfiguredBase(t *testing.T) {
	root := t.TempDir()
	set := Settings{
		CreateUserDir:    true,
		UserHomeBasePath: "users",
	}

	userScope, err := set.MakeUserDir("alice", ".", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/users/alice" {
		t.Fatalf("expected legacy dot scope to use configured home scope, got %q", userScope)
	}

	if _, err := os.Stat(filepath.Join(root, "users", "alice", ".keep")); err != nil {
		t.Fatal(err)
	}
}

func TestMakeUserDirTreatsRootScopeAsGeneratedUserDir(t *testing.T) {
	root := t.TempDir()
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.MakeUserDir("alice", "/", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", userScope)
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
	if userScope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", userScope)
	}

	if _, err := os.Stat(filepath.Join(root, "users", "alice", ".keep")); err != nil {
		t.Fatal(err)
	}
}

func TestMakeUserDirPreservesExplicitScope(t *testing.T) {
	root := t.TempDir()
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.MakeUserDir("alice", "/projects", root)
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/projects" {
		t.Fatalf("expected explicit scope /projects to be preserved, got %q", userScope)
	}
	if _, err := os.Stat(filepath.Join(root, "projects")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "home", "alice", ".keep")); !os.IsNotExist(err) {
		t.Fatalf("expected no generated home marker for explicit scope, got %v", err)
	}
}
