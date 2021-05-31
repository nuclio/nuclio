package opa

import "fmt"

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

const (
	UserIDHeader       string = "x-user-id"
	UserGroupIdsHeader string = "x-user-group-ids"
)

type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

func GenerateProjectResourceString(projectName string) string {
	return fmt.Sprintf("/projects/%s", projectName)
}

func GenerateFunctionResourceString(projectName, functionName string) string {
	return fmt.Sprintf("/projects/%s/functions/%s", projectName, functionName)
}

func GenerateFunctionEventResourceString(projectName, functionName, functionEventName string) string {
	return fmt.Sprintf("/projects/%s/functions/%s/events/%s", projectName, functionName, functionEventName)
}
