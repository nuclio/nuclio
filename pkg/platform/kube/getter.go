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
	"io"
	"strconv"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/renderer"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	wideFormat = "wide"
)

type getter struct {
	logger       nuclio.Logger
	getOptions     *platform.GetOptions
	kubeCommonOptions *CommonOptions
	consumer          *consumer
	platform          platform.Platform
}

func newGetter(parentLogger nuclio.Logger, platform platform.Platform) (*getter, error) {
	newgetter := &getter{
		logger: parentLogger.GetChild("getter").(nuclio.Logger),
		platform: platform,
	}

	return newgetter, nil
}

func (g *getter) get(consumer *consumer, getOptions *platform.GetOptions, writer io.Writer) error {
	var err error

	// save options, consumer
	g.getOptions = getOptions
	g.kubeCommonOptions = getOptions.Common.Platform.(*CommonOptions)
	g.consumer = consumer

	resourceName, resourceVersion, err := nuctl.ParseResourceIdentifier(getOptions.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	functionsToRender := []functioncr.Function{}

	// if version is specified, get single function
	if resourceVersion != nil {

		// get specific function CR
		function, err := g.consumer.functioncrClient.Get(g.kubeCommonOptions.Namespace, resourceName)
		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}

		functionsToRender = append(functionsToRender, *function)

	} else {

		functions, err := g.consumer.functioncrClient.List(g.kubeCommonOptions.Namespace,
			&meta_v1.ListOptions{LabelSelector: getOptions.Labels})

		if err != nil {
			return errors.Wrap(err, "Failed to list functions")
		}

		// convert []Function to []*Function
		functionsToRender = functions.Items
	}

	// render it
	return g.renderFunctions(writer, functionsToRender)
}

func (g *getter) renderFunctions(writer io.Writer, functions []functioncr.Function) error {

	rendererInstance := renderer.NewRenderer(writer)

	switch g.getOptions.Format {
	case "text", wideFormat:
		header := []string{"Namespace", "Name", "Version", "State", "Local URL", "Node Port", "Replicas"}
		if g.getOptions.Format == wideFormat {
			header = append(header, "Labels")
		}

		functionRecords := [][]string{}

		// for each field
		for _, function := range functions {

			// get its fields
			functionFields := g.getFunctionFields(&function, g.getOptions.Format == wideFormat)

			// add to records
			functionRecords = append(functionRecords, functionFields)
		}

		rendererInstance.RenderTable(header, functionRecords)
	case "yaml":
		rendererInstance.RenderYAML(functions)
	case "json":
		rendererInstance.RenderJSON(functions)
	}

	return nil
}

func (g *getter) getFunctionFields(function *functioncr.Function, wide bool) []string {

	// populate stuff from function
	line := []string{function.Namespace,
		function.Labels["name"],
		function.Labels["version"],
		string(function.Status.State)}

	// add info from service & deployment
	// TODO: for lists we can get Service & Deployment info using .List get into a map to save http gets

	returnPartialFunctionFields := func() []string {
		return append(line, []string{"-", "-", "-"}...)
	}

	service, err := g.consumer.clientset.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	deployment, err := g.consumer.clientset.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	cport := strconv.Itoa(int(service.Spec.Ports[0].Port))
	nport := strconv.Itoa(int(service.Spec.Ports[0].NodePort))
	pods := strconv.Itoa(int(deployment.Status.AvailableReplicas)) + "/" + strconv.Itoa(int(*deployment.Spec.Replicas))
	line = append(line, []string{service.Spec.ClusterIP + ":" + cport, nport, pods}...)

	if g.getOptions.Format == wideFormat {
		line = append(line, common.StringMapToString(function.Labels))
	}

	return line
}
