package settings

import (
	"path"
	"regexp"
	"strings"

	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/rules"
	"github.com/versioneer-tech/package-r/users"
)

// UserDefaults is a type that holds the default values
// for some fields on User.
type UserDefaults struct {
	Scope        string             `json:"scope"`
	Locale       string             `json:"locale"`
	ViewMode     users.ViewMode     `json:"viewMode"`
	SingleClick  bool               `json:"singleClick"`
	Sorting      files.Sorting      `json:"sorting"`
	Perm         users.Permissions  `json:"perm"`
	Commands     []string           `json:"commands"`
	HideDotfiles bool               `json:"hideDotfiles"`
	DateFormat   bool               `json:"dateFormat"`
	Envs         *map[string]string `json:"envs,omitempty"`
}

// Apply applies the default options to a user.
func (d *UserDefaults) Apply(u *users.User) {
	u.Scope = d.Scope
	u.Locale = d.Locale
	u.ViewMode = d.ViewMode
	u.SingleClick = d.SingleClick
	u.Perm = d.Perm
	u.Sorting = d.Sorting
	u.Commands = d.Commands
	u.HideDotfiles = d.HideDotfiles
	u.DateFormat = d.DateFormat
	u.Envs = d.Envs
}

// ApplyUserDefaults applies settings defaults to a newly created user.
func (s *Settings) ApplyUserDefaults(u *users.User) {
	s.Defaults.Apply(u)
	s.ApplyUserDirBaseRules(u)
}

// ApplyUserDirBaseRules applies generated user dir base rules to a new user.
func (s *Settings) ApplyUserDirBaseRules(u *users.User) {
	generatedRules := userDirBaseRules(s.userHomeBasePath(), u.Username)
	if len(generatedRules) == 0 {
		return
	}

	existingRules := withoutRules(u.Rules, generatedRules)
	if u.Perm.Admin {
		u.Rules = existingRules
		return
	}

	u.Rules = make([]rules.Rule, 0, len(generatedRules)+len(existingRules))
	u.Rules = append(u.Rules, generatedRules...)
	u.Rules = append(u.Rules, existingRules...)
}

func (s *Settings) IsGeneratedUserDirBaseRule(username string, rule rules.Rule) bool {
	if s == nil {
		return false
	}
	return containsRule(userDirBaseRules(s.userHomeBasePath(), username), rule)
}

func (s *Settings) userHomeBasePath() string {
	if s == nil || strings.TrimSpace(s.UserHomeBasePath) == "" {
		return DefaultUsersHomeBasePath
	}
	return s.UserHomeBasePath
}

func userDirBaseRules(userDirBase, username string) []rules.Rule {
	userDirBase = normalizeUserDirBase(userDirBase)
	if userDirBase == "" {
		return nil
	}

	userPath := userDirBaseFor(userDirBase, username)
	if userPath == "" {
		return []rules.Rule{denyUserDirBaseRule(userDirBase)}
	}

	return []rules.Rule{
		denyUserDirBaseRule(userDirBase),
		allowUserDirBaseRule(userPath),
	}
}

func normalizeUserDirBase(userDirBase string) string {
	userDirBase = strings.TrimSpace(userDirBase)
	if userDirBase == "" {
		return ""
	}
	if !strings.HasPrefix(userDirBase, "/") {
		userDirBase = "/" + userDirBase
	}
	return path.Clean(userDirBase)
}

func userDirBaseFor(userDirBase, username string) string {
	username = CleanUsername(username)
	if username == "" || username == "-" || username == "." {
		return ""
	}
	return path.Join(userDirBase, username)
}

func denyUserDirBaseRule(userDirBase string) rules.Rule {
	childPrefix := userDirBase
	if childPrefix != "/" {
		childPrefix += "/"
	}

	return rules.Rule{
		Regex:  true,
		Allow:  false,
		Regexp: &rules.Regexp{Raw: "^" + regexp.QuoteMeta(childPrefix) + ".+"},
	}
}

func allowUserDirBaseRule(userDirBase string) rules.Rule {
	return rules.Rule{
		Regex:  true,
		Allow:  true,
		Regexp: &rules.Regexp{Raw: "^" + regexp.QuoteMeta(userDirBase) + "(/|$)"},
	}
}

func withoutRules(existing, generated []rules.Rule) []rules.Rule {
	filtered := existing[:0]
	for _, rule := range existing {
		if containsRule(generated, rule) {
			continue
		}
		filtered = append(filtered, rule)
	}
	return filtered
}

func containsRule(rulez []rules.Rule, rule rules.Rule) bool {
	for _, candidate := range rulez {
		if sameRule(candidate, rule) {
			return true
		}
	}
	return false
}

func sameRule(a, b rules.Rule) bool {
	return a.Regex == b.Regex &&
		a.Allow == b.Allow &&
		a.Path == b.Path &&
		regexpRaw(a.Regexp) == regexpRaw(b.Regexp)
}

func regexpRaw(r *rules.Regexp) string {
	if r == nil {
		return ""
	}
	return r.Raw
}
