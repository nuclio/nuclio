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

package platform

import (
	"context"
	"math/rand"
	"strconv"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Function interface {

	// Initialize instructs the function to load the fields specified by "fields". Some function implementations
	// are lazy-load - this ensures that the fields are populated properly. if "fields" is nil, all fields
	// are loaded
	Initialize(context.Context, []string) error

	// GetConfig will return the configuration of the function
	GetConfig() *functionconfig.Config

	// GetStatus returns the state of the function
	GetStatus() *functionconfig.Status

	// GetInvokeURL returns the URL on which the function can be invoked
	GetInvokeURL(context.Context, InvokeViaType) (string, error)

	// GetReplicas returns the current # of replicas and the configured # of replicas
	GetReplicas() (int, int)

	// GetVersion returns a string representing the version
	GetVersion() string

	// GetConfigWithStatus returns configuration and state of the function
	GetConfigWithStatus() *functionconfig.ConfigWithStatus
}

type AbstractFunction struct {
	Logger   logger.Logger
	Config   functionconfig.Config
	Status   functionconfig.Status
	Platform Platform
	function Function
}

func NewAbstractFunction(parentLogger logger.Logger,
	parentPlatform Platform,
	config *functionconfig.Config,
	status *functionconfig.Status,
	function Function) (*AbstractFunction, error) {

	return &AbstractFunction{
		Logger:   parentLogger.GetChild("function"),
		Config:   *config,
		Status:   *status,
		Platform: parentPlatform,
		function: function,
	}, nil
}

// Initialize instructs the function to load the fields specified by "fields". Some function implementations
// are lazy-load - this ensures that the fields are populated properly. if "fields" is nil, all fields
// are loaded
func (af *AbstractFunction) Initialize(context.Context, []string) error {
	return nil
}

// GetConfig will return the configuration of the function
func (af *AbstractFunction) GetConfig() *functionconfig.Config {
	return &af.Config
}

// GetVersion returns a string representing the version
func (af *AbstractFunction) GetVersion() string {
	if af.Config.Spec.Version == -1 {
		return "latest"
	}

	return strconv.Itoa(af.Config.Spec.Version)
}

// GetInvokeURL returns the URL on which the function can be invoked
func (af *AbstractFunction) GetInvokeURL(context.Context, InvokeViaType) (string, error) {
	return "", errors.New("Unsupported")
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (af *AbstractFunction) GetReplicas() (int, int) {
	return 0, 0
}

// GetStatus returns the state of the function
func (af *AbstractFunction) GetStatus() *functionconfig.Status {
	return &af.Status
}

// GetExternalIPInvocationURL returns function external invocation url
func (af *AbstractFunction) GetExternalIPInvocationURL() (string, int, error) {
	externalIPAddresses, err := af.Platform.GetExternalIPAddresses()
	if err != nil || len(externalIPAddresses) == 0 {
		return "", 0, errors.New("No external IP addresses found")
	}

	// get a random external IP address
	chosenExternalIPAddress := externalIPAddresses[rand.Intn(len(externalIPAddresses))]

	// return it and the port
	return chosenExternalIPAddress, af.function.GetStatus().HTTPPort, nil
}

// GetConfigWithStatus returns both function config and status
func (af *AbstractFunction) GetConfigWithStatus() *functionconfig.ConfigWithStatus {
	return &functionconfig.ConfigWithStatus{
		Config: *af.GetConfig(),
		Status: *af.GetStatus(),
	}
}
