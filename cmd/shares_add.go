package cmd

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"

	fbErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/users"
)

func init() {
	sharesCmd.AddCommand(sharesAddCmd)
}

var sharesAddCmd = &cobra.Command{
	Use:   "add <id|username> <hash> <path>",
	Short: "Create a new default share",
	Long:  `Create a new default share and add it to the database.`,
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
		}
		settings, err := d.store.Settings.Get()
		checkErr(err)
		server, err := d.store.Settings.GetServer()
		checkErr(err)

		opts := share.LinkOptions{
			Path:   args[2],
			UserID: owner.ID,
			Root:   server.Root,
		}
		if settings.Catalog.DefaultName != "" {
			body.CatalogName = settings.Catalog.DefaultName
			body.AssetsBaseURL = defaultShareAssetsBaseURL(args[2])
		}

		link, err := share.NewLink(body, opts)
		checkErr(err)

		existingByHash, err := d.store.Share.GetByHash(args[1])
		switch {
		case err == nil:
			if existingByHash.UserID == owner.ID && existingByHash.Path == args[2] {
				if shouldApplyDefaultCatalog(existingByHash, link) {
					existingByHash.CatalogURL = link.CatalogURL
					existingByHash.FiltersField = link.FiltersField
					existingByHash.AssetsBaseURL = link.AssetsBaseURL
					checkErr(d.store.Share.Update(existingByHash))
				}
				printShares([]*share.Link{existingByHash})
				return
			}
			checkErr(fmt.Errorf("share hash already exists: %s", args[1]))
		case !errors.Is(err, fbErrors.ErrNotExist):
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
		case !errors.Is(err, fbErrors.ErrNotExist):
			checkErr(err)
		}

		err = d.store.Share.Save(link)
		checkErr(err)
		printShares([]*share.Link{link})
	}, pythonConfig{}),
}

func defaultShareAssetsBaseURL(sharePath string) string {
	cleanSharePath := path.Clean("/" + strings.TrimPrefix(sharePath, "/"))
	parent := path.Dir(cleanSharePath)
	if parent == "/" || parent == "." {
		return ""
	}
	return parent
}

func shouldApplyDefaultCatalog(existing, target *share.Link) bool {
	return target.CatalogURL != "" &&
		existing.CatalogURL == "" &&
		existing.FiltersField == "" &&
		existing.AssetsBaseURL == ""
}
