package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	fbErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/share"
)

func init() {
	sharesCmd.AddCommand(sharesLsCmd)
}

var sharesLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all shares.",
	Args:  cobra.NoArgs,
	Run: python(func(_ *cobra.Command, _ []string, d pythonData) {
		list, err := d.store.Share.All()
		if errors.Is(err, fbErrors.ErrNotExist) {
			list = []*share.Link{}
		} else {
			checkErr(err)
		}

		printShares(list)
	}, pythonConfig{}),
}
