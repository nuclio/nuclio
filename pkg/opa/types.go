package opa

import (
	"fmt"
	"net/http"
	"strings"
)

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

func GetUserAndGroupIdsFromHeaders(request *http.Request) []string {
	var ids []string

	userID := request.Header.Get(UserIDHeader)
	userGroupIdsStr := request.Header.Get(UserGroupIdsHeader)

	if userID != "" {
		ids = append(ids, userID)
	}

	if userGroupIdsStr != "" {
		ids = append(ids, strings.Split(userGroupIdsStr, ",")...)
	}

	return ids
}

func GenerateProjectResourceString(projectName string) string {
	return fmt.Sprintf("/projects/%s", projectName)
}

func GenerateFunctionResourceString(projectName, functionName string) string {
	return fmt.Sprintf("/projects/%s/functions/%s", projectName, functionName)
}

func GenerateFunctionEventResourceString(projectName, functionName, functionEventName string) string {
	return fmt.Sprintf("/projects/%s/functions/%s/function-events/%s", projectName, functionName, functionEventName)
}
