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

package runtime

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/v3ioclient"
)

func newContext(logger nuclio.Logger, configuration *Configuration) *nuclio.Context {
	newContext := &nuclio.Context{
		Logger: logger,
	}

	// create v3io context if applicable
	for _, dataBinding := range configuration.DataBindings {
		if dataBinding.Class == "v3io" {
			newContext.DataBinding = v3ioclient.NewV3ioClient(logger, dataBinding.URL)
		}
	}

	return newContext
}
