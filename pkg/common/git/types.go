package git

type Attributes struct {
	Branch    string `json:"branch,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Reference string `json:"reference,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}
