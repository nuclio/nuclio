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
	"time"

	"github.com/nuclio/logger"
)

// CreateOpaClient creates an OPA client by a given configuration
func CreateOpaClient(parentLogger logger.Logger, opaConfiguration *Config) Client {
	var newOpaClient Client

	switch opaConfiguration.ClientKind {
	case ClientKindHTTP:
		newOpaClient = NewHTTPClient(parentLogger,
			opaConfiguration.Address,
			opaConfiguration.PermissionQueryPath,
			time.Duration(opaConfiguration.RequestTimeout)*time.Second,
			opaConfiguration.LogLevel,
			opaConfiguration.OverrideHeaderValue)

	case ClientKindMock:
		newOpaClient = &MockClient{}

	case ClientKindNop:
		newOpaClient = NewNopClient(parentLogger, opaConfiguration.LogLevel)

	default:
		newOpaClient = NewNopClient(parentLogger, opaConfiguration.LogLevel)
	}

	return newOpaClient
}
