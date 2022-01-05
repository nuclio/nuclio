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

const DefaultReadBufferSize = 16 * 1024
const DefaultMaxRequestBodySize = 4 * 1024 * 1024
const InternalHealthPath = "/__internal/health"

type Configuration struct {
	trigger.Configuration
	ReadBufferSize int

	// NOTE: Modifying the max request body size affect with gradually memory consumption increasing
	// as the entire request being read into the memory
	// https://github.com/valyala/fasthttp/issues/667#issuecomment-540965683
	MaxRequestBodySize int
	ReduceMemoryUsage  bool
	CORS               *cors.CORS
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)

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

	if newConfiguration.MaxRequestBodySize == 0 {
		newConfiguration.MaxRequestBodySize = DefaultMaxRequestBodySize
	}

	if newConfiguration.CORS != nil && newConfiguration.CORS.Enabled {
		newConfiguration.CORS = createCORSConfiguration(newConfiguration.CORS)
	}
	return &newConfiguration, nil
}

func createCORSConfiguration(corsConfiguration *cors.CORS) *cors.CORS {

	// take defaults
	corsInstance := cors.NewCORS()

	// override with custom configuration if provided
	if len(corsConfiguration.AllowHeaders) > 0 {
		corsInstance.AllowHeaders = corsConfiguration.AllowHeaders
	}

	if len(corsConfiguration.AllowMethods) > 0 {
		corsInstance.AllowMethods = corsConfiguration.AllowMethods
	}

	if len(corsConfiguration.AllowOrigins) > 0 {
		corsInstance.AllowOrigins = corsConfiguration.AllowOrigins
	}

	if len(corsConfiguration.ExposeHeaders) > 0 {
		corsInstance.ExposeHeaders = corsConfiguration.ExposeHeaders
	}

	if corsConfiguration.AllowCredentials {
		corsInstance.AllowCredentials = corsConfiguration.AllowCredentials
	}

	if corsConfiguration.PreflightMaxAgeSeconds != nil {
		corsInstance.PreflightMaxAgeSeconds = corsConfiguration.PreflightMaxAgeSeconds
	}

	return corsInstance

}

func (c *Configuration) corsEnabled() bool {
	return c.CORS != nil && c.CORS.Enabled
}
