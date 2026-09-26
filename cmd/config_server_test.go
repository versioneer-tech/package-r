package cmd

import "testing"

func TestConfigInitStoresServerFeatureDefaults(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if !server.EnableThumbnails || !server.ResizePreview || !server.TypeDetectionByHeader {
		t.Fatalf("expected preview features to be enabled by default, got %#v", server)
	}
	if server.EnableExec {
		t.Fatalf("expected command execution to be disabled, got %#v", server)
	}
	if server.TokenExpirationTime != "2h" {
		t.Fatalf("expected token expiration 2h, got %q", server.TokenExpirationTime)
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
		"--stac-browser-url=https://browser.moregeo.it/external/",
	)

	configured, err := openTestStorage(t, dbPath).Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	if configured.STACBrowserURL != "https://browser.moregeo.it/external/" {
		t.Fatalf("unexpected STAC Browser URL: %q", configured.STACBrowserURL)
	}
}

func TestConfigSetStoresDisabledServerFeatures(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "set",
		"--disable-thumbnails=true",
		"--disable-preview-resize=true",
		"--disable-exec=true",
		"--disable-type-detection-by-header=true",
		"--token-expiration-time=30m",
	)

	server, err := openTestStorage(t, dbPath).Settings.GetServer()
	if err != nil {
		t.Fatal(err)
	}
	if server.EnableThumbnails || server.ResizePreview || server.EnableExec || server.TypeDetectionByHeader {
		t.Fatalf("expected server features to be disabled, got %#v", server)
	}
	if server.TokenExpirationTime != "30m" {
		t.Fatalf("expected token expiration 30m, got %q", server.TokenExpirationTime)
	}
}
