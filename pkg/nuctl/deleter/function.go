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

package deleter

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/nuctl"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionDeleter struct {
	nuctl.KubeConsumer
	logger  nuclio.Logger
}

func NewFunctionDeleter(parentLogger nuclio.Logger, kubeconfigPath string) (*FunctionDeleter, error) {
	var err error

	newFunctionDeleter := &FunctionDeleter{
		logger:  parentLogger.GetChild("deleter").(nuclio.Logger),
	}

	// get kube stuff
	_, err = newFunctionDeleter.GetClients(newFunctionDeleter.logger, kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionDeleter, nil
}

func (fd *FunctionDeleter) Execute(options *Options) error {
	var err error

	resourceName, _, err := nuctl.ParseResourceIdentifier(options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	err = fd.FunctioncrClient.Delete(options.Common.Namespace, resourceName, &meta_v1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to delete function CR")
	}

	fd.logger.InfoWith("Function deleted", "name", resourceName)

	return nil
}
