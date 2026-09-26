package cmd

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/versioneer-tech/package-r/settings"
)

func init() {
	configCmd.AddCommand(configSetCmd)
	addConfigFlags(configSetCmd.Flags())
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Updates the configuration",
	Long: `Updates the configuration. Set the flags for the options
you want to change. Other options will remain unchanged.`,
	Args: cobra.NoArgs,
	Run: python(func(cmd *cobra.Command, _ []string, d pythonData) {
		flags := cmd.Flags()
		set, err := d.store.Settings.Get()
		checkErr(err)

		ser, err := d.store.Settings.GetServer()
		checkErr(err)

		hasAuth := false
		flags.Visit(func(flag *pflag.Flag) {
			switch flag.Name {
			case "baseurl":
				ser.BaseURL = mustGetString(flags, flag.Name)
			case "root":
				ser.Root = mustGetString(flags, flag.Name)
			case "socket":
				ser.Socket = mustGetString(flags, flag.Name)
			case "cert":
				ser.TLSCert = mustGetString(flags, flag.Name)
			case "key":
				ser.TLSKey = mustGetString(flags, flag.Name)
			case "address":
				ser.Address = mustGetString(flags, flag.Name)
			case "port":
				ser.Port = mustGetString(flags, flag.Name)
			case "log":
				ser.Log = mustGetString(flags, flag.Name)
			case "signup":
				set.Signup = mustGetBool(flags, flag.Name)
			case "auth.method":
				hasAuth = true
			case "shell":
				set.Shell = convertCmdStrToCmdArray(mustGetString(flags, flag.Name))
			case "create-user-dir":
				set.CreateUserDir = mustGetBool(flags, flag.Name)
			case "branding.name":
				set.Branding.Name = mustGetString(flags, flag.Name)
			case "branding.color":
				set.Branding.Color = mustGetString(flags, flag.Name)
			case "branding.theme":
				set.Branding.Theme = mustGetString(flags, flag.Name)
			case "branding.disableExternal":
				set.Branding.DisableExternal = mustGetBool(flags, flag.Name)
			case "branding.disableUsedPercentage":
				set.Branding.DisableUsedPercentage = mustGetBool(flags, flag.Name)
			case "branding.files":
				set.Branding.Files = mustGetString(flags, flag.Name)
			case "sharelink.defaultHash":
				set.ShareLink.DefaultHash = mustGetString(flags, flag.Name)
			case "catalog.defaultName":
				set.Catalog.DefaultName = mustGetString(flags, flag.Name)
			case "catalog.previewURL":
				set.Catalog.PreviewURL = mustGetString(flags, flag.Name)
			case "catalog.assetMappings":
				set.Catalog.AssetMappings = parseCatalogAssetMappings(mustGetString(flags, flag.Name))
			}
		})

		getUserDefaults(flags, &set.Defaults, false)

		// read the defaults
		auther, err := d.store.Auth.Get(set.AuthMethod)
		checkErr(err)

		// check if there are new flags for existing auth method
		set.AuthMethod, auther = getAuthentication(flags, hasAuth, set, auther)

		err = d.store.Auth.Save(auther)
		checkErr(err)
		err = d.store.Settings.Save(set)
		checkErr(err)
		err = d.store.Settings.SaveServer(ser)
		checkErr(err)
		printSettings(ser, set, auther)
	}, pythonConfig{}),
}

func parseCatalogAssetMappings(value string) []settings.CatalogAssetMapping {
	if value == "" {
		return nil
	}

	var mappings []settings.CatalogAssetMapping
	checkErr(json.Unmarshal([]byte(value), &mappings))
	for i, mapping := range mappings {
		if mapping.From == "" {
			checkErr(fmt.Errorf("catalog asset mapping %d has an empty from value", i))
		}
		if !catalogMappingPathIsRelative(mapping.To) {
			checkErr(fmt.Errorf("catalog asset mapping %d to value must stay inside the share", i))
		}
	}
	return mappings
}

func catalogMappingPathIsRelative(value string) bool {
	if strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return false
	}
	clean := path.Clean(value)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}
