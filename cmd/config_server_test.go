package cmd

import "testing"

func TestConfigInitDisablesServerProcessingByDefault(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if server.EnableThumbnails || server.ResizePreview || server.TypeDetectionByHeader {
		t.Fatalf("expected server-side preview and header processing to be disabled by default, got %#v", server)
	}
	if server.EnableExec {
		t.Fatalf("expected command execution to be disabled, got %#v", server)
	}
	if server.TokenExpirationTime != "2h" {
		t.Fatalf("expected token expiration 2h, got %q", server.TokenExpirationTime)
	}
}

func TestConfigInitStoresBucketCatalog(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "init",
		"--root", rootPath,
		"--buckets=xyz-data,xyz-archive",
	)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if server.Buckets != "xyz-data,xyz-archive" {
		t.Fatalf("unexpected bucket catalog %q", server.Buckets)
	}
}

func TestConfigInitStoresCORSAllowedOrigins(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "init",
		"--root", rootPath,
		"--cors-allowed-origins=https://viewer.example,https://portal.example",
	)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if server.CORSAllowedOrigins != "https://viewer.example,https://portal.example" {
		t.Fatalf("unexpected CORS allowed origins %q", server.CORSAllowedOrigins)
	}
}

func TestConfigInitStoresLogLevel(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "init",
		"--root", rootPath,
		"--log-level=DEBUG",
	)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if server.LogLevel != "DEBUG" {
		t.Fatalf("unexpected log level %q", server.LogLevel)
	}
}

func TestConfigInitStoresSTACBrowserURL(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "init",
		"--root", rootPath,
		"--stac-browser-url=http://localhost:8080/external/",
	)

	configured, err := openTestStorage(t, dbPath).Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	if configured.STACBrowserURL != "http://localhost:8080/external/" {
		t.Fatalf("unexpected STAC Browser URL: %q", configured.STACBrowserURL)
	}
}

func TestConfigSetCanEnableServerProcessing(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "set",
		"--disable-thumbnails=false",
		"--disable-preview-resize=false",
		"--disable-exec=true",
		"--disable-type-detection-by-header=false",
		"--token-expiration-time=30m",
		"--cors-allowed-origins=https://viewer.example",
	)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if !server.EnableThumbnails || !server.ResizePreview || server.EnableExec || !server.TypeDetectionByHeader {
		t.Fatalf("expected server processing to be enabled explicitly, got %#v", server)
	}
	if server.TokenExpirationTime != "30m" {
		t.Fatalf("expected token expiration 30m, got %q", server.TokenExpirationTime)
	}
	if server.CORSAllowedOrigins != "https://viewer.example" {
		t.Fatalf("unexpected CORS allowed origins %q", server.CORSAllowedOrigins)
	}
}
