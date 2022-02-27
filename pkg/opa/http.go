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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type HTTPClient struct {
	logger               logger.Logger
	address              string
	permissionQueryPath  string
	permissionFilterPath string
	requestTimeout       time.Duration
	logLevel             int
	overrideHeaderValue  string
	httpClient           *http.Client
}

func NewHTTPClient(parentLogger logger.Logger,
	address string,
	permissionQueryPath string,
	permissionFilterPath string,
	requestTimeout time.Duration,
	logLevel int,
	overrideHeaderValue string) *HTTPClient {
	newClient := HTTPClient{
		logger:               parentLogger.GetChild("opa"),
		address:              address,
		permissionQueryPath:  permissionQueryPath,
		permissionFilterPath: permissionFilterPath,
		requestTimeout:       requestTimeout,
		logLevel:             logLevel,
		overrideHeaderValue:  overrideHeaderValue,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
	return &newClient
}

// QueryPermissionsMultiResources query permissions for multiple resources at once.
// The response is a list of booleans indicating for each resource if the action against such resource
// is allowed or not.
// Therefore, it is guaranteed that len(resources) and len(results) are equal and
// resources[i] query permission is at results[i]
func (c *HTTPClient) QueryPermissionsMultiResources(resources []string,
	action Action,
	permissionOptions *PermissionOptions) ([]bool, error) {

	// initialize results
	results := make([]bool, len(resources))

	// If the override header value matches the configured override header value, allow without checking
	if c.overrideHeaderValue != "" && permissionOptions.OverrideHeaderValue == c.overrideHeaderValue {

		// allow them all
		for i := 0; i < len(results); i++ {
			results[i] = true
		}

		return results, nil
	}

	requestURL := fmt.Sprintf("%s%s", c.address, c.permissionFilterPath)

	// send the request
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	request := PermissionFilterRequest{Input: PermissionFilterRequestInput{
		resources,
		string(action),
		permissionOptions.MemberIds,
	}}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate request body")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Sending request to OPA",
			"requestBody", string(requestBody),
			"requestURL", requestURL)
	}
	var responseBody []byte
	err = common.RetryUntilSuccessful(6*time.Second,
		2*time.Second,
		func() bool {
			responseBody, _, err = common.SendHTTPRequest(c.httpClient,
				http.MethodPost,
				requestURL,
				requestBody,
				headers,
				[]*http.Cookie{},
				http.StatusOK)
			if err != nil {
				c.logger.WarnWith("Failed to send HTTP request to OPA, retrying",
					"err", err)
				return false
			}
			return true
		})
	if err != nil {
		if c.logLevel > 5 {
			c.logger.ErrorWith("Failed to send HTTP request to OPA",
				"err", errors.GetErrorStackString(err, 10))
		}
		return nil, errors.Wrap(err, "Failed to send HTTP request to OPA")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Received response from OPA",
			"responseBody", string(responseBody))
	}

	permissionFilterResponse := PermissionFilterResponse{}
	if err := json.Unmarshal(responseBody, &permissionFilterResponse); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Successfully unmarshalled permission filter response",
			"permissionFilterResponse", permissionFilterResponse)
	}

	for resourceIdx, resource := range resources {
		if common.StringInSlice(resource, permissionFilterResponse.Result) {
			results[resourceIdx] = true
		}
	}
	return results, nil
}

func (c *HTTPClient) QueryPermissions(resource string,
	action Action,
	permissionOptions *PermissionOptions) (bool, error) {

	// If the override header value matches the configured override header value, allow without checking
	if c.overrideHeaderValue != "" && permissionOptions.OverrideHeaderValue == c.overrideHeaderValue {
		return true, nil
	}

	requestURL := fmt.Sprintf("%s%s", c.address, c.permissionQueryPath)

	// send the request
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	request := PermissionQueryRequest{Input: PermissionQueryRequestInput{
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
	var responseBody []byte
	err = common.RetryUntilSuccessful(6*time.Second,
		2*time.Second,
		func() bool {
			responseBody, _, err = common.SendHTTPRequest(c.httpClient,
				http.MethodPost,
				requestURL,
				requestBody,
				headers,
				[]*http.Cookie{},
				http.StatusOK)
			if err != nil {
				c.logger.WarnWith("Failed to send HTTP request to OPA, retrying",
					"err", err)
				return false
			}
			return true
		})
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

	permissionResponse := PermissionQueryResponse{}
	if err := json.Unmarshal(responseBody, &permissionResponse); err != nil {
		return false, errors.Wrap(err, "Failed to unmarshal response body")
	}

	if c.logLevel > 5 {
		c.logger.InfoWith("Successfully unmarshalled permission response",
			"permissionResponse", permissionResponse)
	}

	return permissionResponse.Result, nil
}
