package cmd

import (
	"encoding/json"
	"fmt"
	"io"
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
	Short: "Public share management",
	Long:  `Manage public shares in the runtime database.`,
	Args:  cobra.NoArgs,
}

func printShares(links []*share.Link) {
	checkErr(writeShares(os.Stdout, links))
}

func writeShares(output io.Writer, links []*share.Link) error {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "Hash\tPath\tUser ID\tExpire\tPassword protected\tCatalog\tAsset mappings\tDescription"); err != nil {
		return err
	}

	for _, link := range links {
		assetMappings := ""
		if len(link.AssetMappings) > 0 {
			encoded, err := json.Marshal(link.AssetMappings)
			if err != nil {
				return err
			}
			assetMappings = string(encoded)
		}
		password := "no"
		if link.PasswordHash != "" {
			password = "yes"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\t\n",
			link.Hash,
			link.Path,
			link.UserID,
			link.Expire,
			password,
			link.CatalogURL,
			assetMappings,
			link.Description,
		); err != nil {
			return err
		}
	}

	return w.Flush()
}
