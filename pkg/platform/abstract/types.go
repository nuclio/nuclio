package abstract

type functionLogLine struct {
	Time    *string `json:"time"`
	Level   *string `json:"level"`
	Message *string `json:"message"`
	Name    *string `json:"name,omitempty"`
	More    *string `json:"more,omitempty"`

	// these fields may be filled by user function log lines
	Datetime *string           `json:"datetime"`
	With     map[string]string `json:"with,omitempty"`
}
