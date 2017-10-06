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
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	wideFormat = "wide"
)

type getter struct {
	logger            nuclio.Logger
	getOptions        *platform.GetOptions
	kubeCommonOptions *CommonOptions
	consumer          *consumer
	platform          platform.Platform
}

func newGetter(parentLogger nuclio.Logger, platform platform.Platform) (*getter, error) {
	newgetter := &getter{
		logger:   parentLogger.GetChild("getter").(nuclio.Logger),
		platform: platform,
	}

	return newgetter, nil
}

func (g *getter) get(consumer *consumer, getOptions *platform.GetOptions) ([]platform.Function, error) {
	var err error

	// save options, consumer
	g.getOptions = getOptions
	g.kubeCommonOptions = getOptions.Common.Platform.(*CommonOptions)
	g.consumer = consumer

	functioncrInstanceList, err := g.consumer.functioncrClient.List(g.kubeCommonOptions.Namespace,
		&meta_v1.ListOptions{LabelSelector: getOptions.Labels})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	functions := []platform.Function{}

	// convert []functioncr.Function -> function
	for _, functioncrInstance := range functioncrInstanceList.Items {
		functions = append(functions, &function{
			Function: functioncrInstance,
			consumer: g.consumer,
		})
	}

	// render it
	return functions, nil
}
