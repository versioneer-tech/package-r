package settings

import "testing"

func TestApplyCatalogDefaults(t *testing.T) {
	catalog := Catalog{}
	applyCatalogDefaults(&catalog)

	if catalog.DefaultName != DefaultCatalogName {
		t.Fatalf("expected default catalog name %q, got %q", DefaultCatalogName, catalog.DefaultName)
	}
}

func TestApplyCatalogEnvironment(t *testing.T) {
	t.Setenv("PACKAGE_R_CATALOG_DEFAULT_NAME", "items/catalog.parquet")
	t.Setenv("PACKAGE_R_CATALOG_PREVIEW_URL", "https://viewer.example/")
	t.Setenv("PACKAGE_R_CATALOG_ASSET_MAPPINGS", `[{"from":"https://data.example/","to":"assets"}]`)

	catalog := Catalog{}
	if err := ApplyCatalogEnvironment(&catalog); err != nil {
		t.Fatal(err)
	}

	if catalog.DefaultName != "items/catalog.parquet" {
		t.Fatalf("unexpected catalog name: %q", catalog.DefaultName)
	}
	if catalog.PreviewURL != "https://viewer.example/" {
		t.Fatalf("unexpected preview URL: %q", catalog.PreviewURL)
	}
	if len(catalog.AssetMappings) != 1 {
		t.Fatalf("expected one asset mapping, got %d", len(catalog.AssetMappings))
	}
	if catalog.AssetMappings[0].From != "https://data.example/" || catalog.AssetMappings[0].To != "assets" {
		t.Fatalf("unexpected asset mapping: %#v", catalog.AssetMappings[0])
	}
}
