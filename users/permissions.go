package users

// Permissions describe a user's permissions.
type Permissions struct {
	Admin    bool `json:"admin"`
	Execute  bool `json:"execute"`
	Connect  bool `json:"connect"`
	Stage    bool `json:"stage"`
	Create   bool `json:"create"`
	Rename   bool `json:"rename"`
	Modify   bool `json:"modify"`
	Delete   bool `json:"delete"`
	Share    bool `json:"share"`
	Download bool `json:"download"`
}
