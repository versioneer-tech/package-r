package cmd

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSharesAddKeepsCatalogEmptyByDefault(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "my-password")
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "shares", "add", "admin", "my-share", "/files")

	link, err := openTestStorage(t, dbPath).Share.GetByHash("my-share")
	if err != nil {
		t.Fatal(err)
	}
	if link.CatalogURL != "" {
		t.Fatalf("expected no catalog, got %q", link.CatalogURL)
	}
	if len(link.AssetMappings) != 0 {
		t.Fatalf("expected no asset mappings, got %#v", link.AssetMappings)
	}
	if link.PasswordHash != "" || link.Token != "" {
		t.Fatal("expected an unprotected share")
	}
}

func TestSharesAddStoresOptionalPasswordAndCatalog(t *testing.T) {
	dbPath, configPath, rootPath := newConfigTestDB(t)

	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "config", "init", "--root", rootPath)
	runPackageRCommand(t, "--config", configPath, "--database", dbPath, "users", "add", "admin", "my-password")
	runPackageRCommand(
		t,
		"--config", configPath,
		"--database", dbPath,
		"shares", "add", "admin", "my-share", "/files",
		"--password", "my-password",
		"--catalog-name", "catalogs/items.parquet",
		"--asset-mappings", `[{"from":"s3://data/","to":"."}]`,
	)

	link, err := openTestStorage(t, dbPath).Share.GetByHash("my-share")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte("my-password")); err != nil {
		t.Fatalf("configured password does not match the stored password hash: %v", err)
	}
	if link.Token == "" {
		t.Fatal("expected a protected share token")
	}
	if link.CatalogURL != "/files/catalogs/items.parquet" {
		t.Fatalf("unexpected catalog URL: %q", link.CatalogURL)
	}
	if len(link.AssetMappings) != 1 || link.AssetMappings[0].From != "s3://data/" || link.AssetMappings[0].To != "." {
		t.Fatalf("unexpected catalog asset mappings: %#v", link.AssetMappings)
	}
}
