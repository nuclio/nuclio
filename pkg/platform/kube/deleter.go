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

package kube

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deleter struct {
	logger   nuclio.Logger
	platform platform.Platform
}

func newDeleter(parentLogger nuclio.Logger, platform platform.Platform) (*deleter, error) {
	newdeleter := &deleter{
		logger:   parentLogger.GetChild("deleter"),
		platform: platform,
	}

	return newdeleter, nil
}

func (d *deleter) delete(consumer *consumer, deleteOptions *platform.DeleteOptions) error {
	var err error

	resourceName, _, err := nuctl.ParseResourceIdentifier(deleteOptions.FunctionConfig.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	err = consumer.functioncrClient.Delete(deleteOptions.FunctionConfig.Meta.Namespace, resourceName, &meta_v1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to delete function CR")
	}

	d.logger.InfoWith("Function deleted", "name", resourceName)

	return nil
}
