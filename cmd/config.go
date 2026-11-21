package cmd

import (
	"encoding/json"
	nerrors "errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/versioneer-tech/package-r/auth"
	"github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/settings"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management utility",
	Long:  `Configuration management utility.`,
	Args:  cobra.NoArgs,
}

func addConfigFlags(flags *pflag.FlagSet) {
	addServerFlags(flags)
	addUserFlags(flags)
	flags.BoolP("signup", "s", false, "allow users to signup")
	flags.Bool("create-user-dir", false, "generate user's home directory automatically")
	flags.String("shell", "", "shell command to which other commands should be appended")

	flags.String("auth.method", string(auth.MethodJSONAuth), "authentication type")
	flags.String("auth.header", "", "HTTP header for auth.method=proxy")
	flags.String("auth.mapper", "", "(optional) HTTP header value mapping strategy for auth.method=proxy")
	flags.String("auth.jwt.jwks-url", "", "(optional) JWKS URL for verified JWT auth.method=proxy headers")
	flags.String("auth.jwt.issuer", "", "(optional) expected issuer for verified JWT auth.method=proxy headers")
	flags.String("auth.jwt.audience", "", "(optional) expected audience for verified JWT auth.method=proxy headers")
	flags.String("auth.jwt.algorithms", "", "(optional) comma-separated allowed JWT algorithms for auth.method=proxy; defaults to RS256")
	flags.String("auth.jwt.clock-skew", "", "(optional) JWT clock skew for auth.method=proxy; defaults to 1m")
	flags.String("auth.command", "", "command for auth.method=hook")

	flags.String("recaptcha.host", "https://www.google.com", "use another host for ReCAPTCHA. recaptcha.net might be useful in China")
	flags.String("recaptcha.key", "", "ReCaptcha site key")
	flags.String("recaptcha.secret", "", "ReCaptcha secret")

	flags.String("branding.name", "", "replace 'packageR' with this name")
	flags.String("branding.theme", "", "set the theme")
	flags.String("branding.color", "", "set the theme color")
	flags.String("branding.files", "", "path to directory with images and custom styles")
	flags.Bool("branding.disableExternal", false, "disable external links such as GitHub links")

	flags.String("stac-browser-url", "", "optional STAC Browser external-catalog URL")
}

func getAuthentication(flags *pflag.FlagSet, defaults ...interface{}) (settings.AuthMethod, auth.Auther) {
	method, defaultAuther := authenticationDefaults(flags, defaults...)
	switch method {
	case auth.MethodProxyAuth:
		return method, proxyAuthentication(flags, defaultAuther)
	case auth.MethodJSONAuth:
		return method, jsonAuthentication(flags, defaultAuther)
	case auth.MethodHookAuth:
		command := mustGetString(flags, "auth.command")

		if command == "" {
			command = defaultAuther["command"].(string)
		}

		if command == "" {
			checkErr(nerrors.New("you must set the flag 'auth.command' for method 'hook'"))
		}

		return method, &auth.HookAuth{Command: command}
	default:
		panic(errors.ErrInvalidAuthMethod)
	}
}

func authenticationDefaults(flags *pflag.FlagSet, defaults ...interface{}) (method settings.AuthMethod, defaultAuther map[string]interface{}) {
	method = settings.AuthMethod(mustGetString(flags, "auth.method"))

	if len(defaults) > 0 {
		if hasAuth := defaults[0]; hasAuth != true {
			for _, arg := range defaults {
				switch def := arg.(type) {
				case *settings.Settings:
					method = def.AuthMethod
				case auth.Auther:
					ms, err := json.Marshal(def)
					checkErr(err)
					err = json.Unmarshal(ms, &defaultAuther)
					checkErr(err)
				}
			}
		}
	}

	return method, defaultAuther
}

func proxyAuthentication(flags *pflag.FlagSet, defaultAuther map[string]interface{}) auth.Auther {
	header := proxyAuthString(flags, defaultAuther, "auth.header", "header")

	if header == "" {
		checkErr(nerrors.New("you must set the flag 'auth.header' for method 'proxy'"))
	}

	mapper := proxyAuthString(flags, defaultAuther, "auth.mapper", "mapper")
	jwtJwksURL := proxyAuthString(flags, defaultAuther, "auth.jwt.jwks-url", "jwtJwksURL")
	jwtIssuer := proxyAuthString(flags, defaultAuther, "auth.jwt.issuer", "jwtIssuer")
	jwtAudience := proxyAuthString(flags, defaultAuther, "auth.jwt.audience", "jwtAudience")
	jwtAlgorithms := proxyAuthString(flags, defaultAuther, "auth.jwt.algorithms", "jwtAlgorithms")
	jwtClockSkew := proxyAuthString(flags, defaultAuther, "auth.jwt.clock-skew", "jwtClockSkew")

	if jwtJwksURL != "" {
		if mapper == "" || mapper[0] != '.' {
			checkErr(nerrors.New("you must set 'auth.mapper' to '.<claim>' when using 'auth.jwt.jwks-url'"))
		}
		if jwtIssuer == "" {
			checkErr(nerrors.New("you must set 'auth.jwt.issuer' when using 'auth.jwt.jwks-url'"))
		}
	}

	return &auth.ProxyAuth{
		Header:        header,
		Mapper:        mapper,
		JWTJwksURL:    jwtJwksURL,
		JWTIssuer:     jwtIssuer,
		JWTAudience:   jwtAudience,
		JWTAlgorithms: jwtAlgorithms,
		JWTClockSkew:  jwtClockSkew,
	}
}

func jsonAuthentication(flags *pflag.FlagSet, defaultAuther map[string]interface{}) auth.Auther {
	jsonAuth := &auth.JSONAuth{}
	host := mustGetString(flags, "recaptcha.host")
	key := mustGetString(flags, "recaptcha.key")
	secret := mustGetString(flags, "recaptcha.secret")

	if key == "" {
		if kmap, ok := defaultAuther["recaptcha"].(map[string]interface{}); ok {
			key = kmap["key"].(string)
		}
	}

	if secret == "" {
		if smap, ok := defaultAuther["recaptcha"].(map[string]interface{}); ok {
			secret = smap["secret"].(string)
		}
	}

	if key != "" && secret != "" {
		jsonAuth.ReCaptcha = &auth.ReCaptcha{
			Host:   host,
			Key:    key,
			Secret: secret,
		}
	}
	return jsonAuth
}

func proxyAuthString(flags *pflag.FlagSet, defaultAuther map[string]interface{}, flagName, jsonName string) string {
	value := mustGetString(flags, flagName)
	if flags.Changed(flagName) || value != "" {
		return value
	}
	if defaultAuther == nil {
		return ""
	}
	if existing, ok := defaultAuther[jsonName].(string); ok {
		return existing
	}
	return ""
}

func printSettings(ser *settings.Server, set *settings.Settings, auther auth.Auther) {
	checkErr(writeSettings(os.Stdout, ser, set, auther))
}

func writeSettings(output io.Writer, ser *settings.Server, set *settings.Settings, auther auth.Auther) error {
	w := &tableWriter{Writer: tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)}

	w.printf("Sign up:\t%t\n", set.Signup)
	w.printf("Create User Dir:\t%t\n", set.CreateUserDir)
	w.printf("Auth method:\t%s\n", set.AuthMethod)
	w.printf("STAC Browser URL:\t%s\n", set.STACBrowserURL)
	w.printf("Shell:\t%s\t\n", strings.Join(set.Shell, " "))
	w.printf("\nBranding:\n")
	w.printf("\tName:\t%s\n", set.Branding.Name)
	w.printf("\tFiles override:\t%s\n", set.Branding.Files)
	w.printf("\tDisable external links:\t%t\n", set.Branding.DisableExternal)
	w.printf("\tColor:\t%s\n", set.Branding.Color)
	w.printf("\tTheme:\t%s\n", set.Branding.Theme)
	w.printf("\nServer:\n")
	w.printf("\tLog:\t%s\n", ser.Log)
	w.printf("\tLog level:\t%s\n", ser.LogLevel)
	w.printf("\tPort:\t%s\n", ser.Port)
	w.printf("\tBase URL:\t%s\n", ser.BaseURL)
	w.printf("\tRoot:\t%s\n", ser.Root)
	w.printf("\tSocket:\t%s\n", ser.Socket)
	w.printf("\tAddress:\t%s\n", ser.Address)
	w.printf("\tCORS allowed origins:\t%s\n", ser.CORSAllowedOrigins)
	w.printf("\tTLS Cert:\t%s\n", ser.TLSCert)
	w.printf("\tTLS Key:\t%s\n", ser.TLSKey)
	w.printf("\nDefaults:\n")
	w.printf("\tScope:\t%s\n", set.Defaults.Scope)
	w.printf("\tLocale:\t%s\n", set.Defaults.Locale)
	w.printf("\tView mode:\t%s\n", set.Defaults.ViewMode)
	w.printf("\tSingle Click:\t%t\n", set.Defaults.SingleClick)
	w.printf("\tSorting:\n")
	w.printf("\t\tBy:\t%s\n", set.Defaults.Sorting.By)
	w.printf("\t\tAsc:\t%t\n", set.Defaults.Sorting.Asc)
	w.printf("\tPermissions:\n")
	w.printf("\t\tCreate:\t%t\n", set.Defaults.Perm.Create)
	w.printf("\t\tRename:\t%t\n", set.Defaults.Perm.Rename)
	w.printf("\t\tModify:\t%t\n", set.Defaults.Perm.Modify)
	w.printf("\t\tDelete:\t%t\n", set.Defaults.Perm.Delete)
	if err := w.flush(); err != nil {
		return err
	}

	b, err := json.MarshalIndent(auther, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "\nAuthor configuration (raw):\n\n%s\n\n", string(b))
	return err
}
