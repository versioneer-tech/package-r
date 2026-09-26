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

func TestResolveUserScopeTreatsDotAsRoot(t *testing.T) {
	set := Settings{
		CreateUserDir: true,
	}

	userScope, err := set.ResolveUserScope("alice", ".")
	if err != nil {
		t.Fatal(err)
	}
	if userScope != "/" {
		t.Fatalf("expected dot scope to resolve to root, got %q", userScope)
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
