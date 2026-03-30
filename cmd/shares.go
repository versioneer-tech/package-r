package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/versioneer-tech/package-r/share"
)

func init() {
	rootCmd.AddCommand(sharesCmd)
}

var sharesCmd = &cobra.Command{
	Use:   "shares",
	Short: "Shares management utility",
	Long:  `Shares management utility.`,
	Args:  cobra.NoArgs,
}

func printShares(links []*share.Link) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Hash\tPath\tUser ID\tExpire\tDescription")

	for _, link := range links {
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t\n",
			link.Hash,
			link.Path,
			link.UserID,
			link.Expire,
			link.Description,
		)
	}

	w.Flush()
}
