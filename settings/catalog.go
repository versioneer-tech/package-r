package settings

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
