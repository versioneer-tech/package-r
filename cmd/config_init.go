package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/versioneer-tech/package-r/settings"
)

func init() {
	configCmd.AddCommand(configInitCmd)
	addConfigFlags(configInitCmd.Flags())
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new database",
	Long: `Initialize a new packageR database. Use "package-r config set" to
change these settings later. User options become defaults for new users.`,
	Args: cobra.NoArgs,
	Run: python(func(cmd *cobra.Command, _ []string, d pythonData) {
		defaults := settings.UserDefaults{}
		flags := cmd.Flags()
		getUserDefaults(flags, &defaults, true)
		authMethod, auther := getAuthentication(flags)

		s := &settings.Settings{
			Key:              generateKey(),
			Signup:           mustGetBool(flags, "signup"),
			CreateUserDir:    mustGetBool(flags, "create-user-dir"),
			UserHomeBasePath: settings.DefaultUsersHomeBasePath,
			STACBrowserURL:   mustGetString(flags, "stac-browser-url"),
			Shell:            convertCmdStrToCmdArray(mustGetString(flags, "shell")),
			AuthMethod:       authMethod,
			Defaults:         defaults,
			Branding: settings.Branding{
				Name:            mustGetString(flags, "branding.name"),
				DisableExternal: mustGetBool(flags, "branding.disableExternal"),
				Theme:           mustGetString(flags, "branding.theme"),
				Files:           mustGetString(flags, "branding.files"),
			},
		}

		ser := &settings.Server{
			Address:               mustGetString(flags, "address"),
			Socket:                mustGetString(flags, "socket"),
			Root:                  mustGetString(flags, "root"),
			BaseURL:               mustGetString(flags, "baseurl"),
			TLSKey:                mustGetString(flags, "key"),
			TLSCert:               mustGetString(flags, "cert"),
			Port:                  mustGetString(flags, "port"),
			Log:                   mustGetString(flags, "log"),
			EnableThumbnails:      !mustGetBool(flags, "disable-thumbnails"),
			ResizePreview:         !mustGetBool(flags, "disable-preview-resize"),
			EnableExec:            false,
			TypeDetectionByHeader: !mustGetBool(flags, "disable-type-detection-by-header"),
			TokenExpirationTime:   mustGetString(flags, "token-expiration-time"),
		}

		err := d.store.Settings.Save(s)
		checkErr(err)
		err = d.store.Settings.SaveServer(ser)
		checkErr(err)
		err = d.store.Auth.Save(auther)
		checkErr(err)

		fmt.Printf(`
Congratulations! You've set up your database to use with packageR.
Now add your first user via 'package-r users add' and then you just
need to call the main command to boot up the server.
`)
		printSettings(ser, s, auther)
	}, pythonConfig{noDB: true}),
}
