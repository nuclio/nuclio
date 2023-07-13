/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package opa

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common/headers"
)

type Client interface {
	QueryPermissions(string, Action, *PermissionOptions) (bool, error)
	QueryPermissionsMultiResources(context.Context, []string, Action, *PermissionOptions) ([]bool, error)
}

func GetUserAndGroupIdsFromAuthSession(session auth.Session) []string {
	if session == nil {
		return []string{}
	}
	ids := []string{
		session.GetUserID(),
	}
	ids = append(ids, session.GetGroupIDs()...)
	return ids
}

func GetUserAndGroupIdsFromHeaders(request *http.Request) []string {
	var ids []string

	userID := request.Header.Get(headers.UserID)
	userGroupIdsStr := request.Header.Get(headers.UserGroupIds)

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
