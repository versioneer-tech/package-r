package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/versioneer-tech/package-r/share"
)

func TestWriteSharesIncludesShareConfiguration(t *testing.T) {
	var output bytes.Buffer
	links := []*share.Link{{
		Hash:         "my-share",
		Path:         "/",
		UserID:       1,
		CatalogURL:   "/openaerialmap-assets.parquet",
		PasswordHash: "secret-hash",
		Token:        "secret-token",
		Description:  "Example data",
		AssetMappings: []share.CatalogAssetMapping{{
			From: "s3://data/",
			To:   ".",
		}},
	}}

	if err := writeShares(&output, links); err != nil {
		t.Fatal(err)
	}

	value := output.String()
	for _, expected := range []string{
		"Password protected",
		"Catalog",
		"Asset mappings",
		"Description",
		"yes",
		"Example data",
		"/openaerialmap-assets.parquet",
		`[{"from":"s3://data/","to":"."}]`,
	} {
		if !strings.Contains(value, expected) {
			t.Fatalf("expected share output to contain %q, got:\n%s", expected, value)
		}
	}
	if strings.Contains(value, "secret-hash") || strings.Contains(value, "secret-token") {
		t.Fatalf("share output contains a secret:\n%s", value)
	}
}
