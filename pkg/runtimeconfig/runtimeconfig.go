package runtimeconfig

type Config struct {
	Python *Python `json:"python,omitempty"`
}

type Python struct {
	BuildArgs map[string]string `json:"buildArgs,omitempty"`
}
