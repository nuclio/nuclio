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
	logger       nuclio.Logger
	options      *Options
	kubeConsumer *nuctl.KubeConsumer
}

func NewFunctionDeleter(parentLogger nuclio.Logger) (*FunctionDeleter, error) {
	newFunctionDeleter := &FunctionDeleter{
		logger: parentLogger.GetChild("deleter").(nuclio.Logger),
	}

	return newFunctionDeleter, nil
}

func (fd *FunctionDeleter) Delete(kubeConsumer *nuctl.KubeConsumer, options *Options) error {
	var err error

	// save options, consumer
	fd.options = options
	fd.kubeConsumer = kubeConsumer

	resourceName, _, err := nuctl.ParseResourceIdentifier(options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	err = fd.kubeConsumer.FunctioncrClient.Delete(options.Common.Namespace, resourceName, &meta_v1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to delete function CR")
	}

	fd.logger.InfoWith("Function deleted", "name", resourceName)

	return nil
}
