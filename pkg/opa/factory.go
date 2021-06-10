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

	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
)

// CreateOpaClient creates a opa client based on a requested
func CreateOpaClient(parentLogger logger.Logger, opaConfiguration *platformconfig.OpaConfig) Client {

	var newOpaClient Client

	switch opaConfiguration.ClientKind {
	case platformconfig.OpaClientKindHTTP:
		newOpaClient = NewHTTPClient(parentLogger,
			opaConfiguration.Address,
			opaConfiguration.PermissionQueryPath,
			time.Duration(opaConfiguration.RequestTimeout)*time.Second)

	case platformconfig.OpaClientKindMock:
		newOpaClient = &MockClient{}

	case platformconfig.OpaClientKindNop:
		newOpaClient = NewNopClient(parentLogger)

	default:
		newOpaClient = NewNopClient(parentLogger)
	}

	return newOpaClient
}
