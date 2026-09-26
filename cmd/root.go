package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/versioneer-tech/package-r/auth"
	"github.com/versioneer-tech/package-r/diskcache"
	"github.com/versioneer-tech/package-r/frontend"
	apphttp "github.com/versioneer-tech/package-r/http"
	"github.com/versioneer-tech/package-r/img"
	"github.com/versioneer-tech/package-r/objectstorage"
	"github.com/versioneer-tech/package-r/rclonefs"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/storage"
)

var (
	cfgFile string
)

func init() {
	cobra.OnInitialize(initConfig)
	cobra.MousetrapHelpText = ""

	rootCmd.SetVersionTemplate("packageR version {{printf \"%s\" .Version}}\n")

	flags := rootCmd.Flags()
	persistent := rootCmd.PersistentFlags()

	persistent.StringVarP(&cfgFile, "config", "c", "", "config file path")
	persistent.StringP("database", "d", "/tmp/package-r.db", "database path")

	addServerFlags(flags)
}

func addServerFlags(flags *pflag.FlagSet) {
	flags.StringP("address", "a", "127.0.0.1", "address to listen on")
	flags.StringP("log", "l", "stdout", "log output")
	flags.StringP("port", "p", "8888", "port to listen on")
	flags.StringP("cert", "t", "", "tls certificate")
	flags.StringP("key", "k", "", "tls key")
	flags.StringP("root", "r", "/", "S3 service root (/) or one bucket name")
	flags.String("socket", "", "socket to listen to (cannot be used with address, port, cert nor key flags)")
	flags.Uint32("socket-perm", 0666, "unix socket file permissions")
	flags.StringP("baseurl", "b", "", "base url")
	flags.String("cache-dir", "", "file cache directory (disabled if empty)")
	flags.String("token-expiration-time", "2h", "user session timeout")
	flags.Int("img-processors", 4, "image processors count")
	flags.Bool("disable-thumbnails", false, "disable image thumbnails")
	flags.Bool("disable-preview-resize", false, "disable resize of image previews")
	flags.Bool("disable-exec", false, "disables Command Runner feature")
	flags.Bool("disable-type-detection-by-header", false, "disables type detection by reading file headers")
}

var rootCmd = &cobra.Command{
	Use:   "package-r",
	Short: "Package and browse object-storage data",
	Long: `The packageR CLI manages the packageR database, users, and settings.
packageR stores its state in one local Bolt DB file. It does not need an
external database server.

For this specific command, all the flags you have available (except
"config" for the configuration file), can be given either through
environment variables or configuration files.

If you don't set "config", it will look for a configuration file called
.package-r.{json, toml, yaml, yml} in the following directories:

- ./
- $HOME/
- /etc/package-r/

The precedence of the configuration values are as follows:

- flags
- environment variables
- configuration file
- database values
- defaults

The environment variables are prefixed by "PACKAGE_R_" followed by the option
name in caps, with dots and dashes replaced by underscores. So to set
"database" via an env variable, you should set PACKAGE_R_DATABASE.

Before the first start, create the database with "package-r config init" and
add at least one user with "package-r users add".`,
	Run: python(func(cmd *cobra.Command, _ []string, d pythonData) {
		log.Println(cfgFile)

		// build img service
		workersCount, err := cmd.Flags().GetInt("img-processors")
		checkErr(err)
		if workersCount < 1 {
			log.Fatal("Image resize workers count could not be < 1")
		}
		imgSvc := img.New(workersCount)

		var fileCache diskcache.Interface = diskcache.NewNoOp()
		cacheDir, err := cmd.Flags().GetString("cache-dir")
		checkErr(err)
		if cacheDir != "" {
			if err := os.MkdirAll(cacheDir, 0700); err != nil {
				log.Fatalf("can't make directory %s: %s", cacheDir, err)
			}
			fileCache = diskcache.New(afero.NewOsFs(), cacheDir)
		}

		server := getRunParams(cmd.Flags(), d.store)
		setupLog(server.Log)

		applicationSettings, err := d.store.Settings.Get()
		checkErr(err)
		if applicationSettings.AuthMethod == auth.MethodProxyAuth {
			configuredAuth, err := d.store.Auth.Get(applicationSettings.AuthMethod)
			checkErr(err)
			if proxyAuth, ok := configuredAuth.(*auth.ProxyAuth); ok && proxyAuth.UsesUnverifiedClaimMapping() {
				log.Println("WARNING: proxy claim mapping is enabled without a JWKS URL; token claims are decoded but not validated. Trust only a header set by the upstream proxy.")
			}
		}
		storageConfig := objectstorage.Load()
		storageConfig.SetRoot(server.Root)
		checkErr(storageConfig.ValidateFilesystem())
		checkErr(storageConfig.ValidateUserDir(applicationSettings.CreateUserDir))
		if storageConfig.UsesAWSServiceRootWithoutRegion() {
			log.Println("WARNING: AWS_REGION is not set; rclone will use us-east-1. Buckets in other regions cannot be opened from PACKAGE_R_ROOT=/. Set AWS_REGION or select one bucket with PACKAGE_R_ROOT.")
		}

		objectFileSystems := rclonefs.NewManager(context.Background(), server.Root)
		defer func() { _ = objectFileSystems.Close() }()
		_, err = objectFileSystems.FileSystem()
		checkErr(err)
		d.store.Users = rclonefs.WrapUsers(d.store.Users, objectFileSystems, applicationSettings)
		server.EnableExec = false
		log.Println("Using native rclone object storage; command execution is disabled")

		adr := server.Address + ":" + server.Port

		var listener net.Listener

		switch {
		case server.Socket != "":
			listener, err = net.Listen("unix", server.Socket)
			checkErr(err)
			socketPerm, err := cmd.Flags().GetUint32("socket-perm")
			checkErr(err)
			err = os.Chmod(server.Socket, os.FileMode(socketPerm))
			checkErr(err)
		case server.TLSKey != "" && server.TLSCert != "":
			cer, err := tls.LoadX509KeyPair(server.TLSCert, server.TLSKey)
			checkErr(err)
			listener, err = tls.Listen("tcp", adr, &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cer}},
			)
			checkErr(err)
		default:
			listener, err = net.Listen("tcp", adr)
			checkErr(err)
		}

		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigc)

		assetsFs, err := fs.Sub(frontend.Assets(), "dist")
		if err != nil {
			panic(err)
		}

		handler, err := apphttp.NewHandler(imgSvc, fileCache, d.store, server, assetsFs)
		checkErr(err)

		defer listener.Close()

		log.Println("Listening on", listener.Addr().String())
		httpServer := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}
		serveErrors := make(chan error, 1)
		go func() {
			serveErrors <- httpServer.Serve(listener)
		}()

		select {
		case sig := <-sigc:
			log.Printf("Caught signal %s: shutting down.", sig)
			shutdownContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(shutdownContext); err != nil {
				_ = httpServer.Close()
				panic(err)
			}
			if err := <-serveErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		case err := <-serveErrors:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}
	}, pythonConfig{}),
}

func getRunParams(flags *pflag.FlagSet, st *storage.Storage) *settings.Server {
	server, err := st.Settings.GetServer()
	checkErr(err)

	if val, set := getParamB(flags, "root"); set {
		server.Root = val
	}

	if val, set := getParamB(flags, "baseurl"); set {
		server.BaseURL = val
	}

	if val, set := getParamB(flags, "log"); set {
		server.Log = val
	}

	isSocketSet := false
	isAddrSet := false

	if val, set := getParamB(flags, "address"); set {
		server.Address = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "port"); set {
		server.Port = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "key"); set {
		server.TLSKey = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "cert"); set {
		server.TLSCert = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "socket"); set {
		server.Socket = val
		isSocketSet = isSocketSet || set
	}

	if isAddrSet && isSocketSet {
		checkErr(errors.New("--socket flag cannot be used with --address, --port, --key nor --cert"))
	}

	// Do not use saved Socket if address was manually set.
	if isAddrSet && server.Socket != "" {
		server.Socket = ""
	}

	if disableThumbnails, set := getBoolParam(flags, "disable-thumbnails"); set {
		server.EnableThumbnails = !disableThumbnails
	}

	if disablePreviewResize, set := getBoolParam(flags, "disable-preview-resize"); set {
		server.ResizePreview = !disablePreviewResize
	}

	if disableTypeDetectionByHeader, set := getBoolParam(flags, "disable-type-detection-by-header"); set {
		server.TypeDetectionByHeader = !disableTypeDetectionByHeader
	}

	if disableExec, set := getBoolParam(flags, "disable-exec"); set {
		server.EnableExec = !disableExec
	}

	if val, set := getParamB(flags, "token-expiration-time"); set {
		server.TokenExpirationTime = val
	}

	return server
}

// getParamB returns a value and reports whether a flag, environment variable,
// or configuration file set it. Check flags first because Viper treats a
// bound flag with a default value as set.
func getParamB(flags *pflag.FlagSet, key string) (string, bool) {
	value, _ := flags.GetString(key)

	// If set on Flags, use it.
	if flags.Changed(key) {
		return value, true
	}

	// If set through viper (env, config), return it.
	if v.IsSet(key) {
		return v.GetString(key), true
	}

	// Otherwise use default value on flags.
	return value, false
}

func getParam(flags *pflag.FlagSet, key string) string {
	val, _ := getParamB(flags, key)
	return val
}

func getBoolParam(flags *pflag.FlagSet, key string) (bool, bool) {
	value, err := flags.GetBool(key)
	checkErr(err)

	if flags.Changed(key) {
		return value, true
	}

	if v.IsSet(key) {
		return v.GetBool(key), true
	}

	return value, false
}

func setupLog(logMethod string) {
	switch logMethod {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	case "":
		log.SetOutput(io.Discard)
	default:
		log.SetOutput(&lumberjack.Logger{
			Filename:   logMethod,
			MaxSize:    100,
			MaxAge:     14,
			MaxBackups: 10,
		})
	}
}

func initConfig() {
	if cfgFile == "" {
		home, err := os.UserHomeDir()
		checkErr(err)
		v.AddConfigPath(".")
		v.AddConfigPath(home)
		v.AddConfigPath("/etc/package-r/")
		v.SetConfigName(".package-r")
	} else {
		v.SetConfigFile(cfgFile)
	}

	v.SetEnvPrefix("PACKAGE_R")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if err := v.ReadInConfig(); err != nil {
		var configParseError v.ConfigParseError
		if errors.As(err, &configParseError) {
			panic(err)
		}
		cfgFile = "No config file used"
	} else {
		cfgFile = "Using config file: " + v.ConfigFileUsed()
	}
}
