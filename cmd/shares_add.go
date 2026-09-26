package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	appErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/users"
)

func init() {
	sharesCmd.AddCommand(sharesAddCmd)
	sharesAddCmd.Flags().String("password", "", "password required to open the share")
	sharesAddCmd.Flags().String("catalog-name", "", "relative catalog path inside the share")
	sharesAddCmd.Flags().String("asset-mappings", "", "JSON array of catalog asset URL-to-path mappings")
}

var sharesAddCmd = &cobra.Command{
	Use:   "add <id|username> <hash> <path>",
	Short: "Add a public share",
	Long:  `Add a public share to the runtime database.`,
	Args:  cobra.ExactArgs(3),
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
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

		assetMappings, err := share.ParseCatalogAssetMappings(mustGetString(cmd.Flags(), "asset-mappings"))
		checkErr(err)
		body := share.CreateBody{
			Hash:          args[1],
			Password:      mustGetString(cmd.Flags(), "password"),
			CatalogName:   mustGetString(cmd.Flags(), "catalog-name"),
			AssetMappings: assetMappings,
		}
		opts := share.LinkOptions{
			Path:   args[2],
			UserID: owner.ID,
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
				existingByHash.AssetMappings = link.AssetMappings
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
