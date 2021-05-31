package opa

type PermissionRequestInput struct {
	Resource string   `json:"resource,omitempty"`
	Action   string   `json:"action,omitempty"`
	Ids      []string `json:"ids,omitempty"`
}

type PermissionRequest struct {
	Input PermissionRequestInput `json:"input,omitempty"`
}

type PermissionResponse struct {
	Result bool `json:"result,omitempty"`
}
