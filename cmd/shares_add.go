package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	appErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/users"
)

func init() {
	sharesCmd.AddCommand(sharesAddCmd)
}

var sharesAddCmd = &cobra.Command{
	Use:   "add <id|username> <hash> <path>",
	Short: "Add a configured bootstrap share",
	Long:  `Add a configured share to the ephemeral runtime database.`,
	Args:  cobra.ExactArgs(3),
	Run: python(func(_ *cobra.Command, args []string, d pythonData) {
		username, id := parseUsernameOrID(args[0])

		var (
			owner *users.User
			err   error
		)
		if username != "" {
			owner, err = d.store.Users.Get("", username)
		} else {
			owner, err = d.store.Users.Get("", id)
		}
		checkErr(err)

		body := share.CreateBody{
			Hash:        args[1],
			Description: "default share",
			Password:    os.Getenv("PACKAGE_R_SHARE_PASSWORD"),
		}
		settings, err := d.store.Settings.Get()
		checkErr(err)
		opts := share.LinkOptions{
			Path:   args[2],
			UserID: owner.ID,
		}
		if settings.Catalog.DefaultName != "" {
			body.CatalogName = settings.Catalog.DefaultName
		}

		link, err := share.NewLink(body, opts)
		checkErr(err)

		existingByHash, err := d.store.Share.GetByHash(args[1])
		switch {
		case err == nil:
			if existingByHash.UserID == owner.ID && existingByHash.Path == args[2] {
				existingByHash.Expire = link.Expire
				existingByHash.Description = link.Description
				existingByHash.CatalogURL = link.CatalogURL
				existingByHash.PasswordHash = link.PasswordHash
				existingByHash.Token = link.Token
				checkErr(d.store.Share.Update(existingByHash))
				printShares([]*share.Link{existingByHash})
				return
			}
			checkErr(fmt.Errorf("share hash already exists: %s", args[1]))
		case !errors.Is(err, appErrors.ErrNotExist):
			checkErr(err)
		}

		existingPermanent, err := d.store.Share.GetPermanent(args[2], owner.ID)
		switch {
		case err == nil:
			if existingPermanent.Hash == args[1] {
				printShares([]*share.Link{existingPermanent})
				return
			}
			checkErr(fmt.Errorf("permanent share already exists for path %s and user %s", args[2], owner.Username))
		case !errors.Is(err, appErrors.ErrNotExist):
			checkErr(err)
		}

		err = d.store.Share.Save(link)
		checkErr(err)
		printShares([]*share.Link{link})
	}, pythonConfig{}),
}
