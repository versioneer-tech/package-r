package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
)

func TestGetBoolParamUsesFalseEnvironmentValue(t *testing.T) {
	v.Reset()
	t.Cleanup(v.Reset)
	v.SetEnvPrefix("PACKAGE_R")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	t.Setenv("PACKAGE_R_DISABLE_THUMBNAILS", "false")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("disable-thumbnails", false, "")

	value, set := getBoolParam(flags, "disable-thumbnails")
	if !set {
		t.Fatal("expected environment value to be set")
	}
	if value {
		t.Fatal("expected PACKAGE_R_DISABLE_THUMBNAILS=false to keep thumbnails enabled")
	}
}
