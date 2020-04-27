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

package http

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/cors"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

type Configuration struct {
	trigger.Configuration
	ReadBufferSize int
	CORS           cors.CORS
}

const DefaultReadBufferSize = 16 * 1024

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.URL == "" {
		newConfiguration.URL = ":8080"
	}

	if newConfiguration.ReadBufferSize == 0 {
		newConfiguration.ReadBufferSize = DefaultReadBufferSize
	}

	if newConfiguration.CORS.Enabled {

		// take defaults
		_cors := cors.NewCORS()

		// override with custom configuration if provided
		if len(newConfiguration.CORS.AllowHeaders) > 0 {
			_cors.AllowHeaders = newConfiguration.CORS.AllowHeaders
		}

		if len(newConfiguration.CORS.AllowMethods) > 0 {
			_cors.AllowMethods = newConfiguration.CORS.AllowMethods
		}

		if newConfiguration.CORS.AllowOrigin != "" {
			_cors.AllowOrigin = newConfiguration.CORS.AllowOrigin
		}

		if newConfiguration.CORS.AllowCredentials {
			_cors.AllowCredentials = newConfiguration.CORS.AllowCredentials
		}

		_cors.PreflightMaxAgeSeconds = newConfiguration.CORS.PreflightMaxAgeSeconds

		newConfiguration.CORS = *_cors
	}

	return &newConfiguration, nil
}
