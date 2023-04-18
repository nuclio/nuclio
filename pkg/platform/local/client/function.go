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

package client

import (
	"context"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
)

type function struct {
	platform.AbstractFunction
}

func newFunction(parentLogger logger.Logger,
	parentPlatform platform.Platform,
	config *functionconfig.Config,
	status *functionconfig.Status) (*function, error) {

	newFunction := &function{}
	newAbstractFunction, err := platform.NewAbstractFunction(parentLogger, parentPlatform, config, status, newFunction)
	if err != nil {
		return nil, err
	}

	newFunction.AbstractFunction = *newAbstractFunction

	return newFunction, nil
}

// Initialize does nothing, seeing how no fields require lazy loading
func (f *function) Initialize(context.Context, []string) error {
	var err error

	return err
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *function) GetReplicas() (int, int) {
	return 1, 1
}
