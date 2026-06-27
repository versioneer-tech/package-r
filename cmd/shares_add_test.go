package cmd

import (
	"path/filepath"
	"testing"
)

func TestSharesAddStoresDefaultCatalogInSharedFolder(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "set", "--catalog.defaultName", "catalog.parquet")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "password")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "shares", "add", "admin", "public-share", "/files")

	link, err := openTestStorage(t, dbPath).Share.GetByHash("public-share")
	if err != nil {
		t.Fatal(err)
	}
	expectedCatalogURL := filepath.Join(rootPath, "files", "catalog.parquet")
	if link.CatalogURL != expectedCatalogURL {
		t.Fatalf("expected catalog in shared folder, got %q", link.CatalogURL)
	}
	if link.AssetsBaseURL != "" || link.FiltersField != "" {
		t.Fatalf("expected package-root asset defaults, got assetsBaseURL=%q filtersField=%q", link.AssetsBaseURL, link.FiltersField)
	}
}
