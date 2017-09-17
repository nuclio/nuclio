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

package getter

import (
	"io"
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/util/renderer"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionGetter struct {
	nuctl.KubeConsumer
	logger  nuclio.Logger
}

func NewFunctionGetter(parentLogger nuclio.Logger, kubeconfigPath string) (*FunctionGetter, error) {
	var err error

	newFunctionGetter := &FunctionGetter{
		logger:  parentLogger.GetChild("getter").(nuclio.Logger),
	}

	// get kube stuff
	_, err = newFunctionGetter.GetClients(newFunctionGetter.logger, kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionGetter, nil
}

func (fg *FunctionGetter) Execute(options *Options, writer io.Writer) error {
	var err error

	resourceName, resourceVersion, err := nuctl.ParseResourceIdentifier(options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	functionsToRender := []functioncr.Function{}

	// if version is specified, get single function
	if resourceVersion != nil {

		// get specific function CR
		function, err := fg.FunctioncrClient.Get(options.Common.Namespace, resourceName)
		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}

		functionsToRender = append(functionsToRender, *function)

	} else {

		functions, err := fg.FunctioncrClient.List(options.Common.Namespace,
			&meta_v1.ListOptions{LabelSelector: options.Labels})

		if err != nil {
			return errors.Wrap(err, "Failed to list functions")
		}

		// convert []Function to []*Function
		functionsToRender = functions.Items
	}

	// render it
	return fg.renderFunctions(options, writer, functionsToRender)
}

func (fg *FunctionGetter) renderFunctions(options *Options,
	writer io.Writer,
	functions []functioncr.Function) error {

	rendererInstance := renderer.NewRenderer(writer)

	switch options.Format {
	case "text", "wide":
		header := []string{"Namespace", "Name", "Version", "State", "Local URL", "Node Port", "Replicas"}
		if options.Format == "wide" {
			header = append(header, "Labels")
		}

		functionRecords := [][]string{}

		// for each field
		for _, function := range functions {

			// get its fields
			functionFields := fg.getFunctionFields(options, &function, options.Format == "wide")

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

func (fg *FunctionGetter) getFunctionFields(options *Options, function *functioncr.Function, wide bool) []string {

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

	service, err := fg.Clientset.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	deployment, err := fg.Clientset.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	cport := strconv.Itoa(int(service.Spec.Ports[0].Port))
	nport := strconv.Itoa(int(service.Spec.Ports[0].NodePort))
	pods := strconv.Itoa(int(deployment.Status.AvailableReplicas)) + "/" + strconv.Itoa(int(*deployment.Spec.Replicas))
	line = append(line, []string{service.Spec.ClusterIP + ":" + cport, nport, pods}...)

	if options.Format == "wide" {
		line = append(line, common.StringMapToString(function.Labels))
	}

	return line
}
