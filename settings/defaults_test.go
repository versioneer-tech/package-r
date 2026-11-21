package settings

import (
	"testing"

	"github.com/versioneer-tech/package-r/rules"
	"github.com/versioneer-tech/package-r/users"
)

func TestApplyUserDefaultsAddsUserDirBaseRules(t *testing.T) {
	set := Settings{
		CreateUserDir: true,
		Defaults:      UserDefaults{Locale: "en"},
	}
	user := &users.User{Username: "alice"}

	set.ApplyUserDefaults(user)

	if user.Locale != "en" {
		t.Fatalf("expected locale default to be applied, got %q", user.Locale)
	}
	if len(user.Rules) != 2 {
		t.Fatalf("expected 2 user dir base rules, got %d", len(user.Rules))
	}

	assertMatches(t, user.Rules[0], map[string]bool{
		"/home":             false,
		"/home/bob":         true,
		"/home-archive/bob": false,
	})
	assertMatches(t, user.Rules[1], map[string]bool{
		"/home/alice":          true,
		"/home/alice/file.txt": true,
		"/home/alice-other":    false,
	})

	sanitizedUser := &users.User{Username: "../bob"}
	set.ApplyUserDefaults(sanitizedUser)
	assertMatches(t, sanitizedUser.Rules[1], map[string]bool{"/home/-bob": true, "/home/bob": false})
}

func TestApplyUserDirBaseRulesPrependsGeneratedRules(t *testing.T) {
	set := Settings{CreateUserDir: true}
	user := &users.User{
		Username: "alice",
		Rules:    []rules.Rule{{Allow: true, Path: "/custom"}},
	}

	set.ApplyUserDirBaseRules(user)
	set.ApplyUserDirBaseRules(user)

	if len(user.Rules) != 3 {
		t.Fatalf("expected 2 generated rules and 1 custom rule after repeated apply, got %d", len(user.Rules))
	}
	if user.Rules[2].Path != "/custom" {
		t.Fatalf("expected custom rule to remain last, got %q", user.Rules[2].Path)
	}
}

func TestApplyUserDefaultsUsesConfiguredUserHomeBasePath(t *testing.T) {
	set := Settings{CreateUserDir: true, UserHomeBasePath: "users"}
	user := &users.User{Username: "alice"}

	set.ApplyUserDefaults(user)

	if len(user.Rules) != 2 {
		t.Fatalf("expected 2 user dir base rules, got %d", len(user.Rules))
	}
	assertMatches(t, user.Rules[0], map[string]bool{"/users/bob": true, "/home/bob": false})
	assertMatches(t, user.Rules[1], map[string]bool{"/users/alice/file.txt": true, "/users/bob": false})
}

func TestApplyUserDefaultsSkipsUserDirBaseRulesWhenDisabled(t *testing.T) {
	set := Settings{}
	user := &users.User{Username: "alice"}

	set.ApplyUserDefaults(user)

	if len(user.Rules) != 0 {
		t.Fatalf("expected no generated rules, got %#v", user.Rules)
	}
}

func assertMatches(t *testing.T, rule rules.Rule, cases map[string]bool) {
	t.Helper()

	for path, want := range cases {
		if got := rule.Matches(path); got != want {
			t.Fatalf("expected rule.Matches(%q) to be %t, got %t", path, want, got)
		}
	}
}
