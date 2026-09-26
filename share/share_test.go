package share

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLinkDropsLegacyCatalogFieldsOnRewrite(t *testing.T) {
	legacy := []byte(`{
		"hash":"my-share",
		"path":"/deliverables/26-06",
		"userID":1,
		"catalogURL":"/deliverables/26-06/catalog.parquet",
		"filtersField":"data",
		"assetsBaseURL":"/deliverables"
	}`)

	var link Link
	if err := json.Unmarshal(legacy, &link); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(&link)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "filtersField") || strings.Contains(string(encoded), "assetsBaseURL") {
		t.Fatalf("legacy catalog fields remain after rewrite: %s", encoded)
	}
}

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
