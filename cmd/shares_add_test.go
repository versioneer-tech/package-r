package cmd

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestConfigStoresCatalogAssetMappings(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"config", "set",
		"--catalog.assetMappings", `[{"from":"https://imagery.example.org/openaerialmap/","to":"openaerialmap-assets"}]`,
	)

	configured, err := openTestStorage(t, dbPath).Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(configured.Catalog.AssetMappings) != 1 {
		t.Fatalf("expected one catalog asset mapping, got %#v", configured.Catalog.AssetMappings)
	}
	mapping := configured.Catalog.AssetMappings[0]
	if mapping.From != "https://imagery.example.org/openaerialmap/" || mapping.To != "openaerialmap-assets" {
		t.Fatalf("unexpected catalog asset mapping: %#v", mapping)
	}
}

func TestSharesAddStoresDefaultCatalogInSharedFolder(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "set", "--catalog.defaultName", "catalog.parquet")
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "password")
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "shares", "add", "admin", "my-share", "/files")

	link, err := openTestStorage(t, dbPath).Share.GetByHash("my-share")
	if err != nil {
		t.Fatal(err)
	}
	expectedCatalogURL := "/files/catalog.parquet"
	if link.CatalogURL != expectedCatalogURL {
		t.Fatalf("expected catalog in shared folder, got %q", link.CatalogURL)
	}
}

func TestSharesAddStoresConfiguredPassword(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "password")
	t.Setenv("PACKAGE_R_SHARE_PASSWORD", "1234")
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "shares", "add", "admin", "my-share", "/files")

	link, err := openTestStorage(t, dbPath).Share.GetByHash("my-share")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte("1234")); err != nil {
		t.Fatalf("configured password does not match the stored password hash: %v", err)
	}
	if link.Token == "" {
		t.Fatal("expected a protected share token")
	}
}
