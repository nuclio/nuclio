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

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type HTTPClient struct {
	logger              logger.Logger
	address             string
	permissionQueryPath string
	requestTimeout      time.Duration
	logLevel            int
	overrideHeaderValue string
}

func NewHTTPClient(parentLogger logger.Logger,
	address string,
	permissionQueryPath string,
	requestTimeout time.Duration,
	logLevel int,
	overrideHeaderValue string) *HTTPClient {
	newClient := HTTPClient{
		logger:              parentLogger.GetChild("opa"),
		address:             address,
		permissionQueryPath: permissionQueryPath,
		requestTimeout:      requestTimeout,
		logLevel:            logLevel,
		overrideHeaderValue: overrideHeaderValue,
	}
	return &newClient
}

func (c *HTTPClient) QueryPermissions(resource string, action Action, permissionOptions *PermissionOptions) (bool, error) {

	// If the override header value matches the configured override header value, allow without checking
	if c.overrideHeaderValue != "" && permissionOptions.OverrideHeaderValue == c.overrideHeaderValue {
		return true, nil
	}

	requestURL := fmt.Sprintf("%s%s", c.address, c.permissionQueryPath)

	// send the request
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	request := PermissionRequest{Input: PermissionRequestInput{
		resource,
		string(action),
		permissionOptions.MemberIds,
	}}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return false, errors.Wrap(err, "Failed to generate request body")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Sending request to OPA",
			"requestBody", string(requestBody),
			"requestURL", requestURL)
	}
	responseBody, _, err := common.SendHTTPRequest(http.MethodPost,
		requestURL,
		requestBody,
		headers,
		[]*http.Cookie{},
		http.StatusOK,
		true,
		c.requestTimeout)
	if err != nil {
		if c.logLevel > 5 {
			c.logger.ErrorWith("Failed to send HTTP request to OPA",
				"err", errors.GetErrorStackString(err, 10))
		}
		return false, errors.Wrap(err, "Failed to send HTTP request to OPA")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Received response from OPA",
			"responseBody", string(responseBody))
	}

	permissionResponse := PermissionResponse{}
	if err := json.Unmarshal(responseBody, &permissionResponse); err != nil {
		return false, errors.Wrap(err, "Failed to unmarshal response body")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Successfully unmarshalled permission response",
			"permissionResponse", permissionResponse)
	}

	return permissionResponse.Result, nil
}
