/*
Copyright 2017 The Nuclio Authors.

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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	DefaultRequestTimeout = 10 * time.Second

	PermissionQueryPath string = "/v1/data/iguazio/authz/allow"
)

type Client struct {
	logger  logger.Logger
	enabled bool
	address string
}

func NewClient(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) *Client {
	newClient := Client{
		enabled: false,
		logger:  parentLogger.GetChild("opa"),
	}
	newClient.address = platformConfiguration.OpaAddress

	if newClient.address != "" {
		newClient.enabled = true
	}

	return &newClient
}

func (c *Client) QueryPermissions(resource string, action Action, ids []string) (bool, error) {
	c.logger.DebugWith("Checking permissions in OPA",
		"resource", resource,
		"action", action,
		"ids", ids)

	if !c.enabled {
		c.logger.DebugWith("OPA is disabled, allowing by default",
			"resource", resource,
			"action", action,
			"ids", ids)
		return true, nil
	}

	// send the request
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	request := PermissionRequest{Input: PermissionRequestInput{
		resource,
		string(action),
		ids,
	}}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return false, errors.Wrap(err, "Failed to generate request body")
	}

	responseBody, _, err := common.SendHTTPRequest(http.MethodPost,
		fmt.Sprintf("%s%s", c.address, PermissionQueryPath),
		requestBody,
		headers,
		[]*http.Cookie{},
		http.StatusOK,
		true,
		DefaultRequestTimeout)
	if err != nil {
		return false, errors.Wrap(err, "Failed to send request to OPA")
	}

	permissionResponse := PermissionResponse{}
	if err := json.Unmarshal(responseBody, &permissionResponse); err != nil {
		return false, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return permissionResponse.Result, nil
}
