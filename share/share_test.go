package share

import "testing"

func TestParseCatalogAssetMappings(t *testing.T) {
	mappings, err := ParseCatalogAssetMappings(`[{"from":"s3://data/","to":"."}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 1 || mappings[0].From != "s3://data/" || mappings[0].To != "." {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestParseCatalogAssetMappingsRejectsPathEscape(t *testing.T) {
	_, err := ParseCatalogAssetMappings(`[{"from":"s3://data/","to":"../private"}]`)
	if err == nil {
		t.Fatal("expected an invalid mapping error")
	}
}
