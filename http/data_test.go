package http

import (
	"testing"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

func TestUserDirBaseRulesRestrictOnlySiblingUserDirs(t *testing.T) {
	set := &settings.Settings{}
	user := &users.User{Username: "alice"}
	set.ApplyUserDefaults(user)

	data := &data{
		settings: &settings.Settings{},
		user:     user,
	}

	for path, allowed := range map[string]bool{
		"/catalog.parquet":   true,
		"/home":              true,
		"/home-archive/file": true,
		"/home/alice":        true,
		"/home/alice/file":   true,
		"/home/alice-other":  false,
		"/home/bob/file.txt": false,
	} {
		if got := data.Check(path); got != allowed {
			t.Fatalf("expected Check(%q) to be %t, got %t", path, allowed, got)
		}
	}
}

func TestAdminBypassesUserDirBaseRules(t *testing.T) {
	set := &settings.Settings{}
	user := &users.User{Username: "admin"}
	set.ApplyUserDefaults(user)
	user.Perm.Admin = true

	data := &data{
		settings: &settings.Settings{},
		user:     user,
	}

	for _, path := range []string{
		"/home/bob/file.txt",
		"/home/alice-other",
	} {
		if !data.Check(path) {
			t.Fatalf("expected admin Check(%q) to be true", path)
		}
	}
}
