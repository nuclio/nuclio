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

package common

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/renderer"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	OutputFormatText = "text"
	OutputFormatWide = "wide"
	OutputFormatJSON = "json"
	OutputFormatYAML = "yaml"
)

func RenderFunctions(ctx context.Context,
	logger logger.Logger,
	functions []platform.Function,
	format string,
	writer io.Writer,
	renderCallback func(functions []platform.Function, renderer func(interface{}) error, exportOptions *common.ExportFunctionOptions) error,
	exportOptions *common.ExportFunctionOptions) error {

	errGroup, errGroupCtx := errgroup.WithContext(ctx, logger)
	var renderNodePort bool

	// iterate over each function and make sure it's initialized
	for _, function := range functions {
		function := function
		errGroup.Go("initialize function", func() error {
			if err := function.Initialize(errGroupCtx, nil); err != nil {
				logger.DebugWith("Failed to initialize function", "err", err.Error())
			}
			if function.GetStatus().HTTPPort > 0 {
				renderNodePort = true
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "Failed to initialize functions")
	}

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case OutputFormatText, OutputFormatWide:
		header := []interface{}{"Namespace", "Name", "Project", "State", "Replicas"}
		if renderNodePort {
			header = append(header, "Node Port")
		}
		if format == OutputFormatWide {
			header = append(header, []interface{}{
				"Labels",
				"Internal Invocation URL",
				"External Invocation URLs",
			}...)
		}

		var functionRecords [][]interface{}

		// for each field
		for _, function := range functions {
			availableReplicas, specifiedReplicas := function.GetReplicas()

			// get its fields
			functionFields := []interface{}{
				function.GetConfig().Meta.Namespace,
				function.GetConfig().Meta.Name,
				function.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName],
				encodeFunctionState(function),
				fmt.Sprintf("%d/%d", availableReplicas, specifiedReplicas),
			}

			if renderNodePort {
				if function.GetStatus().HTTPPort > 0 {
					nodePortStr := strconv.Itoa(function.GetStatus().HTTPPort)
					functionFields = append(functionFields, nodePortStr)
				} else {
					functionFields = append(functionFields, "")
				}
			}

			// add fields for wide view
			if format == OutputFormatWide {
				functionFields = append(functionFields, []interface{}{
					common.StringMapToString(function.GetConfig().Meta.Labels),
					strings.Join(function.GetStatus().InternalInvocationURLs, ", "),
					strings.Join(function.GetStatus().ExternalInvocationURLs, ", "),
				}...)
			}

			// add to records
			functionRecords = append(functionRecords, functionFields)
		}

		rendererInstance.RenderTable(header, functionRecords)
	case OutputFormatYAML:
		return renderCallback(functions, rendererInstance.RenderYAML, exportOptions)
	case OutputFormatJSON:
		return renderCallback(functions, rendererInstance.RenderJSON, exportOptions)
	}

	return nil
}

func RenderFunctionEvents(functionEvents []platform.FunctionEvent,
	format string,
	writer io.Writer,
	renderCallback func(functions []platform.FunctionEvent, renderer func(interface{}) error) error) error {

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case OutputFormatText, OutputFormatWide:
		header := []interface{}{"Namespace", "Name", "Display Name", "Function", "Trigger Name", "Trigger Kind"}
		if format == OutputFormatWide {
			header = append(header, []interface{}{
				"Body",
			}...)
		}

		var functionEventRecords [][]interface{}

		// for each field
		for _, functionEvent := range functionEvents {

			// get its fields
			functionEventFields := []interface{}{
				functionEvent.GetConfig().Meta.Namespace,
				functionEvent.GetConfig().Meta.Name,
				functionEvent.GetConfig().Spec.DisplayName,
				functionEvent.GetConfig().Meta.Labels["nuclio.io/function-name"],
				functionEvent.GetConfig().Spec.TriggerName,
				functionEvent.GetConfig().Spec.TriggerKind,
			}

			// add fields for wide view
			if format == OutputFormatWide {
				functionEventFields = append(functionEventFields, []interface{}{
					functionEvent.GetConfig().Spec.Body,
				}...)
			}

			// add to records
			functionEventRecords = append(functionEventRecords, functionEventFields)
		}

		rendererInstance.RenderTable(header, functionEventRecords)
	case OutputFormatYAML:
		return renderCallback(functionEvents, rendererInstance.RenderYAML)
	case OutputFormatJSON:
		return renderCallback(functionEvents, rendererInstance.RenderJSON)
	}

	return nil
}

func RenderProjects(ctx context.Context,
	projects []platform.Project,
	format string,
	writer io.Writer,
	renderCallback func(ctx context.Context, functions []platform.Project, renderer func(interface{}) error) error,
	skipSpecCleanup bool) error {

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case OutputFormatText, OutputFormatWide:
		header := []interface{}{"Namespace", "Name"}
		if format == OutputFormatWide {
			header = append(header, []interface{}{
				"Description",
				"Owner",
				"DefaultNodeSelector",
			}...)
		}

		var projectRecords [][]interface{}

		// for each field
		for _, project := range projects {

			// get its fields
			projectFields := []interface{}{
				project.GetConfig().Meta.Namespace,
				project.GetConfig().Meta.Name,
			}

			// add fields for wide view
			if format == OutputFormatWide {
				projectFields = append(projectFields, []interface{}{
					project.GetConfig().Spec.Description,
					project.GetConfig().Spec.Owner,
				}...)
			}

			// add to records
			projectRecords = append(projectRecords, projectFields)
		}

		rendererInstance.RenderTable(header, projectRecords)
	case OutputFormatYAML:
		return renderCallback(ctx, projects, rendererInstance.RenderYAML)
	case OutputFormatJSON:
		return renderCallback(ctx, projects, rendererInstance.RenderJSON)
	}

	return nil
}

func RenderAPIGateways(apiGateways []platform.APIGateway,
	format string,
	writer io.Writer,
	renderCallback func(apiGateways []platform.APIGateway, renderer func(interface{}) error) error) error {

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case OutputFormatText, OutputFormatWide:
		header := []interface{}{"Namespace", "Name", "Host", "Path", "Primary", "Canary", "Percentage"}
		if format == OutputFormatWide {
			header = append(header, []interface{}{
				"Body",
			}...)
		}

		var apiGatewayRecords [][]interface{}

		// for each field
		for _, apiGateway := range apiGateways {

			// primary function
			primaryFunction := apiGateway.GetConfig().Spec.Upstreams[0].NuclioFunction.Name

			// get canaryFunction if it exists
			canaryFunction := ""
			canaryPercentage := 0
			if len(apiGateway.GetConfig().Spec.Upstreams) == 2 {
				canaryFunction = apiGateway.GetConfig().Spec.Upstreams[1].NuclioFunction.Name
				canaryPercentage = apiGateway.GetConfig().Spec.Upstreams[1].Percentage
			}

			// get its fields
			apiGatewayFields := []interface{}{
				apiGateway.GetConfig().Meta.Namespace,
				apiGateway.GetConfig().Meta.Name,
				apiGateway.GetConfig().Spec.Host,
				apiGateway.GetConfig().Spec.Path,
				primaryFunction,
				canaryFunction,
				fmt.Sprint(canaryPercentage),
			}

			// add fields for wide view
			if format == OutputFormatWide {
				apiGatewayFields = append(apiGatewayFields, []interface{}{
					apiGateway.GetConfig().Spec.Description,
				}...)
			}

			// add to records
			apiGatewayRecords = append(apiGatewayRecords, apiGatewayFields)
		}

		rendererInstance.RenderTable(header, apiGatewayRecords)
	case OutputFormatYAML:
		return renderCallback(apiGateways, rendererInstance.RenderYAML)
	case OutputFormatJSON:
		return renderCallback(apiGateways, rendererInstance.RenderJSON)
	}

	return nil
}

func encodeFunctionState(function platform.Function) string {
	functionStatus := function.GetStatus()
	functionSpec := function.GetConfig().Spec
	if functionStatus.State == functionconfig.FunctionStateReady && functionSpec.Disable {

		// same state as UI
		return "standby"
	}
	return string(functionStatus.State)
}
