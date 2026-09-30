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
	flags.Bool("disable-thumbnails", true, "")

	value, set := getBoolParam(flags, "disable-thumbnails")
	if !set {
		t.Fatal("expected environment value to be set")
	}
	if value {
		t.Fatal("expected PACKAGE_R_DISABLE_THUMBNAILS=false to keep thumbnails enabled")
	}
}

func TestGetParamUsesCORSOriginsEnvironmentValue(t *testing.T) {
	v.Reset()
	t.Cleanup(v.Reset)
	v.SetEnvPrefix("PACKAGE_R")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	t.Setenv("PACKAGE_R_CORS_ALLOWED_ORIGINS", "https://viewer.example")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("cors-allowed-origins", "", "")

	value, set := getParamB(flags, "cors-allowed-origins")
	if !set {
		t.Fatal("expected environment value to be set")
	}
	if value != "https://viewer.example" {
		t.Fatalf("unexpected CORS allowed origins %q", value)
	}
}
