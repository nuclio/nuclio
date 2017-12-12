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

package local

import (
	"fmt"
	"strconv"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
)

type function struct {
	platform.AbstractFunction
	container dockerclient.Container
}

func newFunction(parentLogger nuclio.Logger,
	parentPlatform platform.Platform,
	config *functionconfig.Config,
	container *dockerclient.Container) (*function, error) {
	newAbstractFunction, err := platform.NewAbstractFunction(parentLogger, parentPlatform, config)
	if err != nil {
		return nil, err
	}

	newFunction := &function{
		AbstractFunction: *newAbstractFunction,
		container:        *container,
	}

	return newFunction, nil
}

// Initialize does nothing, seeing how no fields require lazy loading
func (f *function) Initialize([]string) error {
	var err error

	f.Config.Spec.HTTPPort, err = strconv.Atoi(f.container.HostConfig.PortBindings["8080/tcp"][0].HostPort)

	return err
}

// GetState returns the state of the function
func (f *function) GetState() string {
	return "RUNNING"
}

// GetInvokeURL gets the IP of the cluster hosting the function
func (f *function) GetInvokeURL(invokeViaType platform.InvokeViaType) (string, error) {
	return fmt.Sprintf("%s:%d", "localhost", f.Config.Spec.HTTPPort), nil
}

// GetIngresses returns all ingresses for this function
func (f *function) GetIngresses() map[string]functionconfig.Ingress {

	// local platform doesn't support ingress
	return map[string]functionconfig.Ingress{}
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *function) GetReplicas() (int, int) {
	return 1, 1
}
