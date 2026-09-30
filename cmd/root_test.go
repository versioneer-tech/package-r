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

func TestGetParamUsesPackageLogLevelEnvironmentValue(t *testing.T) {
	v.Reset()
	t.Cleanup(v.Reset)
	if err := v.BindEnv("log-level", "PACKAGE_R_LOG_LEVEL"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PACKAGE_R_LOG_LEVEL", "DEBUG")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("log-level", "", "")

	value, set := getParamB(flags, "log-level")
	if !set {
		t.Fatal("expected environment value to be set")
	}
	if value != "DEBUG" {
		t.Fatalf("unexpected log level %q", value)
	}
}

func TestRcloneLogLevelEnvironmentTakesPrecedence(t *testing.T) {
	set := func(string) (string, bool) { return "DEBUG", true }
	if err := configureRcloneLogLevel("INVALID", set); err != nil {
		t.Fatalf("rclone environment override was not preserved: %v", err)
	}

	unset := func(string) (string, bool) { return "", false }
	if err := configureRcloneLogLevel("INVALID", unset); err == nil {
		t.Fatal("expected package log level validation when rclone environment override is absent")
	}
}
