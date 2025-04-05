package share

type CreateBody struct {
	Password    string `json:"password"`
	Expires     string `json:"expires"`
	Description string `json:"description,omitempty"`
	Unit        string `json:"unit"`
	Grant       string `json:"grant"`
}

// Link is the information needed to build a shareable link.
type Link struct {
	Hash         string `json:"hash" storm:"id,index"`
	Path         string `json:"path" storm:"index"`
	UserID       uint   `json:"userID"`
	Expire       int64  `json:"expire"`
	Description  string `json:"description,omitempty"`
	Creation     int64  `json:"creationTime,omitempty"`
	PasswordHash string `json:"password_hash,omitempty"`
	// Token is a random value that will only be set when PasswordHash is set. It is
	// URL-Safe and is used to download links in password-protected shares via a
	// query arg.
	Token string `json:"token,omitempty"`
	Grant string `json:"grant,omitempty"`
}
