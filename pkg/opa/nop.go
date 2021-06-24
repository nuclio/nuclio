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
	"github.com/nuclio/logger"
)

type NopClient struct {
	logger   logger.Logger
	logLevel int
}

func NewNopClient(parentLogger logger.Logger, logLevel int) *NopClient {
	newClient := NopClient{
		logger:   parentLogger.GetChild("opa"),
		logLevel: logLevel,
	}
	return &newClient
}

func (c *NopClient) QueryPermissions(resource string, action Action, permissionOptions *PermissionOptions) (bool, error) {
	if c.logLevel > 5 {
		c.logger.InfoWith("Skipping permission query",
			"resource", resource,
			"action", action,
			"permissionOptions", permissionOptions)
	}
	return true, nil
}
