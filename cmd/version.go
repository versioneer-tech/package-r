package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/versioneer-tech/package-r/v2/version"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("packageR v" + version.Version + "/" + version.CommitSHA)
	},
}
