package settings

type Catalog struct {
	BaseURL     string `json:"baseURL"`
	DefaultName string `json:"defaultName"`
	PreviewURL  string `json:"previewURL"`
}
