package settings

import (
	"testing"
)

func TestResolveUserScopeKeepsGeneratedUserAtRoot(t *testing.T) {
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.ResolveUserScope("alice", "")
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", userScope)
	}
}

func TestResolveUserScopeTreatsDotAsLegacyHomeOnlyScope(t *testing.T) {
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.ResolveUserScope("alice", ".")
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/home/alice" {
		t.Fatalf("expected legacy dot scope to use generated home scope, got %q", userScope)
	}
}

func TestResolveUserScopeUsesConfiguredHomeBase(t *testing.T) {
	set := Settings{
		CreateUserDir:    true,
		UserHomeBasePath: "users",
	}

	userScope, err := set.ResolveUserScope("alice", ".")
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/users/alice" {
		t.Fatalf("expected legacy dot scope to use configured home scope, got %q", userScope)
	}
}

func TestResolveUserScopeTreatsRootAsGeneratedUserDir(t *testing.T) {
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.ResolveUserScope("alice", "/")
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/" {
		t.Fatalf("expected generated user to keep root scope, got %q", userScope)
	}
}

func TestResolveUserScopePreservesExplicitScope(t *testing.T) {
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.ResolveUserScope("alice", "/projects")
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/projects" {
		t.Fatalf("expected explicit scope /projects to be preserved, got %q", userScope)
	}
}
