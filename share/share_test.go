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
