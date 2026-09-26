package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
)

const DefaultCatalogName = "catalog.parquet"

// CatalogAssetMapping maps a catalog URL prefix to a path inside each share.
// It is an escape hatch for URLs whose paths do not contain the shared path.
type CatalogAssetMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Catalog struct {
	DefaultName   string                `json:"defaultName"`
	PreviewURL    string                `json:"previewURL"`
	AssetMappings []CatalogAssetMapping `json:"assetMappings,omitempty"`
}

// ApplyCatalogEnvironment applies process-level catalog overrides.
func ApplyCatalogEnvironment(catalog *Catalog) error {
	applyCatalogDefaults(catalog)
	if value, ok := os.LookupEnv("PACKAGE_R_CATALOG_DEFAULT_NAME"); ok {
		catalog.DefaultName = value
	}
	if value, ok := os.LookupEnv("PACKAGE_R_CATALOG_PREVIEW_URL"); ok {
		catalog.PreviewURL = value
	}
	if value, ok := os.LookupEnv("PACKAGE_R_CATALOG_ASSET_MAPPINGS"); ok {
		mappings, err := ParseCatalogAssetMappings(value)
		if err != nil {
			return err
		}
		catalog.AssetMappings = mappings
	}
	return nil
}

func applyCatalogDefaults(catalog *Catalog) {
	if catalog.DefaultName == "" {
		catalog.DefaultName = DefaultCatalogName
	}
}

// ParseCatalogAssetMappings parses and validates catalog asset mappings.
func ParseCatalogAssetMappings(value string) ([]CatalogAssetMapping, error) {
	if value == "" {
		return nil, nil
	}

	var mappings []CatalogAssetMapping
	if err := json.Unmarshal([]byte(value), &mappings); err != nil {
		return nil, err
	}
	for i, mapping := range mappings {
		if mapping.From == "" {
			return nil, fmt.Errorf("catalog asset mapping %d has an empty from value", i)
		}
		if !catalogMappingPathIsRelative(mapping.To) {
			return nil, fmt.Errorf("catalog asset mapping %d to value must stay inside the share", i)
		}
	}
	return mappings, nil
}

func catalogMappingPathIsRelative(value string) bool {
	if strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return false
	}
	clean := path.Clean(value)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}
