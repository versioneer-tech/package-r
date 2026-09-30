package rclonefs

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	rclone "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/objectstorage"
)

func TestMain(m *testing.M) {
	cacheDir, err := os.MkdirTemp("", "package-r-rclone-test-cache-*")
	if err != nil {
		panic(err)
	}
	if err := config.SetCacheDir(cacheDir); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(cacheDir)
	os.Exit(code)
}

func TestRcloneContextUsesServerModificationTime(t *testing.T) {
	ctx, err := newRcloneContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !rclone.GetConfig(ctx).UseServerModTime {
		t.Fatal("expected rclone to use the modification time from S3 listings")
	}
}

func TestNewS3UsesProgrammaticConfiguration(t *testing.T) {
	fileSystem, err := NewS3(context.Background(), "my-constructor", objectstorage.Config{
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		Endpoint:        "http://127.0.0.1:1",
		Region:          "us-east-1",
		Bucket:          "bucket",
	})
	if err != nil {
		t.Fatal(err)
	}
	link, err := fileSystem.PublicLink(context.Background(), "/team/alice/report.txt", 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/bucket/team/alice/report.txt" {
		t.Fatalf("unexpected public link path %q", parsed.Path)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("public link is not signed: %q", link)
	}
	if err := fileSystem.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewS3UsesAmbientCredentialChain(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ambient-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ambient-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")

	fileSystem, err := NewS3(context.Background(), "my-ambient", objectstorage.Config{
		Endpoint: "http://127.0.0.1:1",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileSystem.Close() })

	link, err := fileSystem.PublicLink(context.Background(), "/report.txt", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if credential := parsed.Query().Get("X-Amz-Credential"); !strings.HasPrefix(credential, "ambient-access/") {
		t.Fatalf("public link did not use ambient credentials: %q", credential)
	}
}

func TestS3ConfigValuesSelectCredentialSource(t *testing.T) {
	static := s3ConfigValues(objectstorage.Config{
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		SessionToken:    "token",
	})
	if static["env_auth"] != "false" || static["access_key_id"] != "access" ||
		static["secret_access_key"] != "secret" || static["session_token"] != "token" {
		t.Fatalf("unexpected static credential settings: %#v", static)
	}

	ambient := s3ConfigValues(objectstorage.Config{SessionToken: "ignored"})
	if ambient["env_auth"] != "true" || ambient["access_key_id"] != "" ||
		ambient["secret_access_key"] != "" || ambient["session_token"] != "" {
		t.Fatalf("unexpected ambient credential settings: %#v", ambient)
	}
}

func TestManagerReusesConfigurationAndSeparatesRotation(t *testing.T) {
	for _, key := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_ENDPOINT_URL",
		"AWS_REGION",
		"PACKAGE_R_ROOT",
		"PACKAGE_R_BUCKETS",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:1")
	t.Setenv("AWS_REGION", "us-east-1")
	manager := NewManager(context.Background(), "bucket")
	first, err := manager.FileSystem()
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.FileSystem()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected the same filesystem for the same configuration")
	}

	t.Setenv("AWS_SESSION_TOKEN", "next-generation")
	third, err := manager.FileSystem()
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("expected a new filesystem after credential rotation")
	}

	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.FileSystem(); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("expected closed manager error, got %v", err)
	}
}

func TestManagerUsesConfiguredRootInsteadOfUserOrProcessRoot(t *testing.T) {
	t.Setenv("PACKAGE_R_ROOT", "process-bucket")
	t.Setenv("AWS_ACCESS_KEY_ID", "access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:1")
	t.Setenv("AWS_REGION", "us-east-1")
	manager := NewManager(context.Background(), "configured-bucket")
	t.Cleanup(func() { _ = manager.Close() })

	link, err := manager.PublicLink(context.Background(), "/report.txt", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/configured-bucket/report.txt" {
		t.Fatalf("unexpected public link path %q", parsed.Path)
	}
}

func TestManagerBuildsConfiguredBucketCatalog(t *testing.T) {
	for _, key := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_ENDPOINT_URL",
		"AWS_REGION",
		"PACKAGE_R_ROOT",
		"PACKAGE_R_BUCKETS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:1")
	t.Setenv("AWS_REGION", "us-east-1")

	manager := NewManager(context.Background(), "/", "xyz-data,xyz-archive")
	t.Cleanup(func() { _ = manager.Close() })
	fileSystem, err := manager.FileSystem()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := afero.ReadDir(fileSystem, "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "xyz-archive" || entries[1].Name() != "xyz-data" {
		t.Fatalf("unexpected bucket catalog: %#v", entries)
	}

	link, err := manager.PublicLink(context.Background(), "/xyz-archive/report.txt", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/xyz-archive/report.txt" {
		t.Fatalf("unexpected public link path %q", parsed.Path)
	}
	if _, err := manager.PublicLink(context.Background(), "/xyz-missing/report.txt", time.Hour); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing bucket error, got %v", err)
	}
}
