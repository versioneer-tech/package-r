package settings

import (
	"testing"

	"github.com/versioneer-tech/package-r/rules"
	"github.com/versioneer-tech/package-r/users"
)

func TestApplyUserDefaultsAddsUserDirBaseRules(t *testing.T) {
	set := Settings{
		Defaults: UserDefaults{Locale: "en"},
	}
	user := &users.User{Username: "alice"}

	set.ApplyUserDefaults(user)

	if user.Locale != "en" {
		t.Fatalf("expected locale default to be applied, got %q", user.Locale)
	}
	if len(user.Rules) != 2 {
		t.Fatalf("expected 2 user dir base rules, got %d", len(user.Rules))
	}

	assertRegexRule(t, user.Rules[0], false, `^/home/.+`)
	assertRegexRule(t, user.Rules[1], true, `^/home/alice(/|$)`)

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
	assertRegexRule(t, sanitizedUser.Rules[1], true, `^/home/-bob(/|$)`)
	assertMatches(t, sanitizedUser.Rules[1], map[string]bool{"/home/bob": false})
}

func TestApplyUserDirBaseRulesPrependsGeneratedRules(t *testing.T) {
	set := Settings{}
	user := &users.User{
		Username: "alice",
		Rules:    []rules.Rule{{Allow: true, Path: "/custom"}},
	}

	set.ApplyUserDirBaseRules(user)
	set.ApplyUserDirBaseRules(user)

	if len(user.Rules) != 3 {
		t.Fatalf("expected 2 generated rules and 1 custom rule after repeated apply, got %d", len(user.Rules))
	}
	assertRegexRule(t, user.Rules[0], false, `^/home/.+`)
	assertRegexRule(t, user.Rules[1], true, `^/home/alice(/|$)`)
	if user.Rules[2].Path != "/custom" {
		t.Fatalf("expected custom rule to remain last, got %q", user.Rules[2].Path)
	}
}

func TestApplyUserDefaultsUsesConfiguredUserHomeBasePath(t *testing.T) {
	set := Settings{UserHomeBasePath: "users"}
	user := &users.User{Username: "alice"}

	set.ApplyUserDefaults(user)

	if len(user.Rules) != 2 {
		t.Fatalf("expected 2 user dir base rules, got %d", len(user.Rules))
	}
	assertRegexRule(t, user.Rules[0], false, `^/users/.+`)
	assertRegexRule(t, user.Rules[1], true, `^/users/alice(/|$)`)
}

func TestApplyUserDefaultsSkipsUserDirBaseRulesForAdmins(t *testing.T) {
	set := Settings{
		Defaults: UserDefaults{
			Perm: users.Permissions{Admin: true},
		},
	}
	user := &users.User{Username: "alice"}

	set.ApplyUserDefaults(user)

	if len(user.Rules) != 0 {
		t.Fatalf("expected admin to have no generated user dir rules, got %#v", user.Rules)
	}
}

func TestApplyUserDirBaseRulesRemovesGeneratedRulesForAdmins(t *testing.T) {
	set := Settings{}
	user := &users.User{
		Username: "alice",
		Perm:     users.Permissions{Admin: true},
		Rules: []rules.Rule{
			denyUserDirBaseRule("/home"),
			allowUserDirBaseRule("/home/alice"),
			{Allow: true, Path: "/custom"},
		},
	}

	set.ApplyUserDirBaseRules(user)

	if len(user.Rules) != 1 || user.Rules[0].Path != "/custom" {
		t.Fatalf("expected admin to keep only custom rules, got %#v", user.Rules)
	}
}

func assertRegexRule(t *testing.T, rule rules.Rule, allow bool, raw string) {
	t.Helper()

	if rule.Allow != allow || !rule.Regex || rule.Regexp == nil || rule.Regexp.Raw != raw {
		t.Fatalf("expected regex rule allow=%t raw=%q, got %#v", allow, raw, rule)
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
