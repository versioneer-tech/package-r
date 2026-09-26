package cmd

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestConfigStoresCatalogAssetMappings(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runFilebrowserCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "set",
		"--catalog.assetMappings", `[{"from":"https://cdn.example/releases/","to":"packages"}]`,
	)

	configured, err := openTestStorage(t, dbPath).Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(configured.Catalog.AssetMappings) != 1 {
		t.Fatalf("expected one catalog asset mapping, got %#v", configured.Catalog.AssetMappings)
	}
	mapping := configured.Catalog.AssetMappings[0]
	if mapping.From != "https://cdn.example/releases/" || mapping.To != "packages" {
		t.Fatalf("unexpected catalog asset mapping: %#v", mapping)
	}
}

func TestSharesAddStoresDefaultCatalogInSharedFolder(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "set", "--catalog.defaultName", "catalog.parquet")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "password")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "shares", "add", "admin", "my-share", "/files")

	link, err := openTestStorage(t, dbPath).Share.GetByHash("my-share")
	if err != nil {
		t.Fatal(err)
	}
	expectedCatalogURL := "/files/catalog.parquet"
	if link.CatalogURL != expectedCatalogURL {
		t.Fatalf("expected catalog in shared folder, got %q", link.CatalogURL)
	}
}

func TestSharesAddStoresConfiguredPIN(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "password")
	t.Setenv("FB_SHARE_PIN", "1234")
	runFilebrowserCommand(t, "--config", configPath, "--database", dbPath, "shares", "add", "admin", "my-share", "/files")

	link, err := openTestStorage(t, dbPath).Share.GetByHash("my-share")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte("1234")); err != nil {
		t.Fatalf("configured PIN does not match the stored password hash: %v", err)
	}
	if link.Token == "" {
		t.Fatal("expected a protected share token")
	}
}
