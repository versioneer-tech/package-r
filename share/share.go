package share

type CreateBody struct {
	Password      string                `json:"password"`
	Expiration    string                `json:"expiration"`
	Description   string                `json:"description"`
	Hash          string                `json:"hash"`
	CatalogName   string                `json:"catalogName"`
	AssetMappings []CatalogAssetMapping `json:"assetMappings"`
}

// CatalogAssetMapping maps a catalog URL prefix to a path inside one share.
type CatalogAssetMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Link is the information needed to build a shareable link.
type Link struct {
	Hash          string                `json:"hash" storm:"id,index"`
	Path          string                `json:"path" storm:"index"`
	UserID        uint                  `json:"userID"`
	Expire        int64                 `json:"expire"`
	Description   string                `json:"description,omitempty"`
	CatalogURL    string                `json:"catalogURL,omitempty"`
	AssetMappings []CatalogAssetMapping `json:"assetMappings,omitempty"`
	PasswordHash  string                `json:"password_hash,omitempty"`
	// Token is a random URL-safe value for later requests to a
	// password-protected share.
	Token string `json:"token,omitempty"`
}
