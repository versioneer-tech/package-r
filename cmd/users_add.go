package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	appErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

func init() {
	usersCmd.AddCommand(usersAddCmd)
	addUserFlags(usersAddCmd.Flags())
}

var usersAddCmd = &cobra.Command{
	Use:   "add <username> <password>",
	Short: "Add or reconcile a user",
	Long:  `Add a user or reconcile an existing user with the provided password and flags.`,
	Args:  cobra.ExactArgs(2),
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		s, err := d.store.Settings.Get()
		checkErr(err)
		getUserDefaults(cmd.Flags(), &s.Defaults, false)

		password, err := users.HashPwd(args[1])
		checkErr(err)

		user, err := d.store.Users.Get("", args[0])
		if err == nil {
			defaults := settings.UserDefaults{
				Scope:       user.Scope,
				Locale:      user.Locale,
				ViewMode:    user.ViewMode,
				SingleClick: user.SingleClick,
				Perm:        user.Perm,
				Sorting:     user.Sorting,
				Commands:    user.Commands,
			}
			getUserDefaults(cmd.Flags(), &defaults, false)
			user.Scope = defaults.Scope
			user.Locale = defaults.Locale
			user.ViewMode = defaults.ViewMode
			user.SingleClick = defaults.SingleClick
			user.Perm = defaults.Perm
			user.Commands = defaults.Commands
			user.Sorting = defaults.Sorting
			user.Password = password
			userScope, resolveErr := s.ResolveUserScope(user.Username, user.Scope)
			checkErr(resolveErr)
			user.Scope = userScope
			checkErr(d.store.Users.Update(user))
			printUsers([]*users.User{user})
			return
		}
		if !errors.Is(err, appErrors.ErrNotExist) {
			checkErr(err)
		}

		user = &users.User{
			Username: args[0],
			Password: password,
		}

		s.ApplyUserDefaults(user)

		userScope, err := s.ResolveUserScope(user.Username, user.Scope)
		checkErr(err)
		user.Scope = userScope

		checkErr(d.store.Users.Save(user))
		printUsers([]*users.User{user})
	}, pythonConfig{}),
}
