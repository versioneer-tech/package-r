package objectstorage

import (
	"errors"
	"testing"
)

func TestLoadUsesProcessSettings(t *testing.T) {
	for _, key := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_ENDPOINT_URL",
		"AWS_REGION",
		"PACKAGE_R_ROOT",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("AWS_REGION", "process-region")
	t.Setenv("PACKAGE_R_ROOT", "process-bucket")

	t.Setenv("AWS_ACCESS_KEY_ID", "process-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "process-secret")
	t.Setenv("AWS_SESSION_TOKEN", "process-token")
	t.Setenv("AWS_ENDPOINT_URL", "https://objects.example.invalid")
	config := Load()

	if config.AccessKeyID != "process-access" ||
		config.SecretAccessKey != "process-secret" ||
		config.SessionToken != "process-token" {
		t.Fatal("process credentials were not loaded")
	}
	if config.Region != "process-region" {
		t.Fatalf("expected process region fallback, got %q", config.Region)
	}
	if config.Root() != "process-bucket" {
		t.Fatalf("unexpected object root %q", config.Root())
	}
}

func TestValidateFilesystem(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   error
	}{
		{name: "bucket", config: Config{Bucket: "bucket", AccessKeyID: "access", SecretAccessKey: "secret"}},
		{name: "service root", config: Config{AccessKeyID: "access", SecretAccessKey: "secret"}},
		{name: "ambient credentials", config: Config{Bucket: "bucket"}},
		{name: "missing secret key", config: Config{Bucket: "bucket", AccessKeyID: "access"}, want: ErrMissingCredentials},
		{name: "missing access key", config: Config{Bucket: "bucket", SecretAccessKey: "secret"}, want: ErrMissingCredentials},
		{name: "bucket path", config: Config{Bucket: "bucket/other", AccessKeyID: "access", SecretAccessKey: "secret"}, want: ErrInvalidRoot},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.config.ValidateFilesystem(); !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestUsesAmbientCredentials(t *testing.T) {
	for _, test := range []struct {
		name   string
		config Config
		want   bool
	}{
		{name: "empty", config: Config{}, want: true},
		{name: "whitespace", config: Config{AccessKeyID: " ", SecretAccessKey: "\t"}, want: true},
		{name: "static", config: Config{AccessKeyID: "access", SecretAccessKey: "secret"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.config.UsesAmbientCredentials(); got != test.want {
				t.Fatalf("expected ambient credentials %t, got %t", test.want, got)
			}
		})
	}
}

func TestUsesAWSServiceRootWithoutRegion(t *testing.T) {
	for _, test := range []struct {
		name   string
		config Config
		want   bool
	}{
		{name: "service root", config: Config{}, want: true},
		{name: "service root with whitespace", config: Config{Endpoint: " ", Region: "\t"}, want: true},
		{name: "configured region", config: Config{Region: "eu-central-1"}},
		{name: "configured bucket", config: Config{Bucket: "reports"}},
		{name: "custom endpoint", config: Config{Endpoint: "https://objects.example.invalid"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.config.UsesAWSServiceRootWithoutRegion(); got != test.want {
				t.Fatalf("expected warning condition %t, got %t", test.want, got)
			}
		})
	}
}

func TestLoadRoot(t *testing.T) {
	for _, test := range []struct {
		name string
		root string
		want string
	}{
		{name: "default", want: ""},
		{name: "service root", root: "/", want: ""},
		{name: "bucket", root: "reports", want: "reports"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("PACKAGE_R_ROOT", test.root)
			config := Load()
			if config.Root() != test.want {
				t.Fatalf("expected root %q, got %q", test.want, config.Root())
			}
		})
	}
}

func TestValidateUserDir(t *testing.T) {
	if err := (Config{}).ValidateUserDir(false); err != nil {
		t.Fatal(err)
	}
	if err := (Config{Bucket: "reports"}).ValidateUserDir(true); err != nil {
		t.Fatal(err)
	}
	if err := (Config{}).ValidateUserDir(true); !errors.Is(err, ErrUserDirNeedsBucket) {
		t.Fatalf("expected %v, got %v", ErrUserDirNeedsBucket, err)
	}
}

func TestObjectPathIncludesUserScope(t *testing.T) {
	tests := []struct {
		name      string
		scope     string
		path      string
		want      string
		wantError bool
	}{
		{name: "nested", scope: "/team/alice", path: "/reports/data.csv", want: "/team/alice/reports/data.csv"},
		{name: "root scope", scope: "/", path: "/reports/data.csv", want: "/reports/data.csv"},
		{name: "scope root", scope: "/team/alice", path: "/", want: "/team/alice"},
		{name: "parent escape", scope: "/team/alice", path: "../../other/secret", wantError: true},
		{name: "sibling prefix", scope: "/team/alice", path: "../alice2/secret", wantError: true},
		{name: "nul", scope: "/team/alice", path: "report\x00.csv", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ObjectPath(test.scope, test.path)
			if test.wantError {
				if !errors.Is(err, ErrInvalidObjectPath) {
					t.Fatalf("expected invalid object path, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
