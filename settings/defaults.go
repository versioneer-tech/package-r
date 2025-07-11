package settings

import (
	"github.com/versioneer-tech/package-r/files"
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
