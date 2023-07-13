/*
Copyright 2023 The Nuclio Authors.

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

package platform

import (
	"github.com/nuclio/logger"
)

type APIGateway interface {

	// GetConfig returns the api gateway config
	GetConfig() *APIGatewayConfig
}

type AbstractAPIGateway struct {
	Logger           logger.Logger
	Platform         Platform
	APIGatewayConfig APIGatewayConfig
}

func NewAbstractAPIGateway(parentLogger logger.Logger,
	parentPlatform Platform,
	apiGatewayConfig APIGatewayConfig) (*AbstractAPIGateway, error) {

	return &AbstractAPIGateway{
		Logger:           parentLogger.GetChild("api gateway"),
		Platform:         parentPlatform,
		APIGatewayConfig: apiGatewayConfig,
	}, nil
}

// GetConfig returns the api gateway config
func (ap *AbstractAPIGateway) GetConfig() *APIGatewayConfig {
	return &ap.APIGatewayConfig
}
