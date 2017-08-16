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

package v3ioclient

import (
	"net/http"

	"github.com/nuclio/nuclio-sdk"

	"github.com/iguazio/v3io"
)

// thin wrapper for v3iow
type V3ioClient struct {
	v3io.V3iow
	logger nuclio.Logger
}

func NewV3ioClient(parentLogger nuclio.Logger, url string) *V3ioClient {

	newV3ioClient := &V3ioClient{
		V3iow: v3io.V3iow{
			Url:        url,
			Tr:         &http.Transport{},
			DebugState: true,
		},
		logger: parentLogger.GetChild("v3io").(nuclio.Logger),
	}

	return newV3ioClient
}

func (vc *V3ioClient) logSink(formatted string) {
	vc.logger.Debug(formatted)
}
