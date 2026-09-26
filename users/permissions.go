package users

// Permissions describe a user's permissions.
type Permissions struct {
	Execute  bool `json:"execute"`
	Create   bool `json:"create"`
	Rename   bool `json:"rename"`
	Modify   bool `json:"modify"`
	Delete   bool `json:"delete"`
	Download bool `json:"download"`
}
