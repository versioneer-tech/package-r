package share

type CreateBody struct {
	Password      string `json:"password"`
	Expires       string `json:"expires"`
	Unit          string `json:"unit"`
	Description   string `json:"description"`
	Hash          string `json:"hash"`
	CatalogName   string `json:"catalogName"`
	FiltersField  string `json:"filtersField"`
	AssetsBaseURL string `json:"assetsBaseURL"`
}

// Link is the information needed to build a shareable link.
type Link struct {
	Hash          string `json:"hash" storm:"id,index"`
	Path          string `json:"path" storm:"index"`
	UserID        uint   `json:"userID"`
	Expire        int64  `json:"expire"`
	Description   string `json:"description,omitempty"`
	CatalogURL    string `json:"catalogURL,omitempty"`
	FiltersField  string `json:"filtersField,omitempty"`
	AssetsBaseURL string `json:"assetsBaseURL,omitempty"`
	PasswordHash  string `json:"password_hash,omitempty"`
	// Token is a random value that will only be set when PasswordHash is set. It is
	// URL-Safe and is used to download links in password-protected shares via a
	// query arg.
	Token string `json:"token,omitempty"`
}
