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
	"github.com/pkg/errors"
)

func newContext(parentLogger nuclio.Logger, configuration *Configuration) (*nuclio.Context, error) {
	newContext := &nuclio.Context{
		Logger:      parentLogger,
		DataBinding: map[string]nuclio.DataBinding{},
	}

	// create v3io context if applicable
	for dataBindingName, dataBinding := range configuration.DataBindings {
		if dataBinding.Class == "v3io" {

			// try to create a v3io client
			v3ioClient, err := v3ioclient.NewV3ioClient(parentLogger, dataBinding.Url)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to create v3io client for %s", dataBinding.Url)
			}

			newContext.DataBinding[dataBindingName] = v3ioClient
		}
	}

	return newContext, nil
}
