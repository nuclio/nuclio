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

package command

import (
	"context"
	"encoding/json"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"

	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
)

type createCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

func newCreateCommandeer(rootCommandeer *RootCommandeer) *createCommandeer {
	commandeer := &createCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"cre"},
		Short:   "Create resources",
	}

	createProjectCommand := newCreateProjectCommandeer(commandeer).cmd
	createFunctionEventCommand := newCreateFunctionEventCommandeer(commandeer).cmd
	createAPIGatewayCommand := newCreateAPIGatewayCommandeer(commandeer).cmd

	cmd.AddCommand(
		createProjectCommand,
		createFunctionEventCommand,
		createAPIGatewayCommand,
	)

	commandeer.cmd = cmd

	return commandeer
}

type createProjectCommandeer struct {
	*createCommandeer
	projectConfig platform.ProjectConfig
}

func newCreateProjectCommandeer(createCommandeer *createCommandeer) *createProjectCommandeer {
	commandeer := &createProjectCommandeer{
		createCommandeer: createCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "project name",
		Aliases: []string{"proj", "prj"},
		Short:   "Create a new project",
		Long:    `Create a new Nuclio project.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Project create requires an identifier")
			}

			// initialize root
			if err := createCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.projectConfig.Meta.Name = args[0]
			commandeer.projectConfig.Meta.Namespace = createCommandeer.rootCommandeer.namespace

			if err := createCommandeer.rootCommandeer.platform.CreateProject(context.Background(), &platform.CreateProjectOptions{
				ProjectConfig: &commandeer.projectConfig,
			}); err != nil {
				return err
			}

			commandeer.rootCommandeer.loggerInstance.InfoWith("Project created",
				"Name",
				commandeer.projectConfig.Meta.Name,
				"Namespace",
				commandeer.projectConfig.Meta.Namespace)
			return nil
		},
	}

	cmd.Flags().StringVar(&commandeer.projectConfig.Spec.Description, "description", "", "Project description")
	cmd.Flags().StringVar(&commandeer.projectConfig.Spec.Owner, "owner", "", "Project owner")
	commandeer.cmd = cmd

	return commandeer
}

type createAPIGatewayCommandeer struct {
	*createCommandeer
	apiGatewayConfig   platform.APIGatewayConfig
	project            string
	host               string
	description        string
	path               string
	authenticationMode string
	basicAuthUsername  string
	basicAuthPassword  string
	function           string
	canaryFunction     string
	canaryPercentage   int
	encodedAttributes  string
}

func newCreateAPIGatewayCommandeer(createCommandeer *createCommandeer) *createAPIGatewayCommandeer {
	commandeer := &createAPIGatewayCommandeer{
		createCommandeer: createCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "apigateway name",
		Aliases: []string{"agw"},
		Short:   "Create api gateways",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Api gateway create requires an identifier")
			}

			// initialize root
			if err := createCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// decode the JSON attributes
			if err := json.Unmarshal([]byte(commandeer.encodedAttributes),
				&commandeer.apiGatewayConfig); err != nil {
				return errors.Wrap(err, "Failed to decode a function's event attributes")
			}

			commandeer.apiGatewayConfig.Meta.Name = args[0]
			commandeer.apiGatewayConfig.Meta.Namespace = createCommandeer.rootCommandeer.namespace

			if commandeer.project != "" {
				commandeer.apiGatewayConfig.Meta.Labels = map[string]string{
					common.NuclioResourceLabelKeyProjectName: commandeer.project,
				}
			}

			// enrich api gateway spec with commandeer input
			commandeer.apiGatewayConfig.Spec.Host = commandeer.host
			commandeer.apiGatewayConfig.Spec.Description = commandeer.description
			commandeer.apiGatewayConfig.Spec.Path = commandeer.path

			// enrich authentication mode
			if commandeer.authenticationMode != "" {
				commandeer.apiGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationMode(commandeer.authenticationMode)
			} else {
				commandeer.apiGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeNone
			}

			// enrich basic-auth spec if it was specified
			if commandeer.apiGatewayConfig.Spec.AuthenticationMode == ingress.AuthenticationModeBasicAuth {
				if commandeer.basicAuthUsername == "" || commandeer.basicAuthPassword == "" {
					return errors.New("Basic auth username and password must be specified")
				}

				commandeer.apiGatewayConfig.Spec.Authentication = &platform.APIGatewayAuthenticationSpec{
					BasicAuth: &platform.BasicAuth{
						Username: commandeer.basicAuthUsername,
						Password: commandeer.basicAuthPassword,
					},
				}
			}

			// validate a primary function was specified
			if commandeer.function == "" {
				return errors.New("A primary function must be specified")
			}

			commandeer.apiGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{
				{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: commandeer.function,
					},
				},
			}

			if commandeer.canaryFunction != "" {
				if commandeer.canaryPercentage == 0 {
					return errors.New("Canary function percentage must be specified")
				}

				canaryUpstream := platform.APIGatewayUpstreamSpec{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: commandeer.canaryFunction,
					},
					Percentage: commandeer.canaryPercentage,
				}

				commandeer.apiGatewayConfig.Spec.Upstreams = append(commandeer.apiGatewayConfig.Spec.Upstreams, canaryUpstream)
			}

			commandeer.apiGatewayConfig.Status.State = platform.APIGatewayStateWaitingForProvisioning

			if err := createCommandeer.rootCommandeer.platform.CreateAPIGateway(context.Background(),
				&platform.CreateAPIGatewayOptions{
					APIGatewayConfig: &commandeer.apiGatewayConfig,
				}); err != nil {
				return err
			}

			commandeer.rootCommandeer.loggerInstance.InfoWith("API gateway created",
				"Name",
				commandeer.apiGatewayConfig.Meta.Name,
				"Namespace",
				commandeer.apiGatewayConfig.Meta.Namespace)
			return nil
		},
	}

	cmd.Flags().StringVar(&commandeer.project, "project", "project", "The project the api gateway should be created in")
	cmd.Flags().StringVar(&commandeer.host, "host", "", "Api gateway host address")
	cmd.Flags().StringVar(&commandeer.description, "description", "", "Api gateway description")
	cmd.Flags().StringVar(&commandeer.path, "path", "", "Api gateway path (the URI that'll be concatenated to the host as an endpoint)")
	cmd.Flags().StringVar(&commandeer.authenticationMode, "authentication-mode", "", "Api gateway authentication mode. ['none', 'basicAuth', 'accessKey']")
	cmd.Flags().StringVar(&commandeer.basicAuthUsername, "basic-auth-username", "", "The basic-auth username")
	cmd.Flags().StringVar(&commandeer.basicAuthPassword, "basic-auth-password", "", "The basic-auth password")
	cmd.Flags().StringVar(&commandeer.function, "function", "", "The api gateway primary function")
	cmd.Flags().StringVar(&commandeer.canaryFunction, "canary-function", "", "The api gateway canary function")
	cmd.Flags().IntVar(&commandeer.canaryPercentage, "canary-percentage", 0, "The canary function percentage")
	cmd.Flags().StringVar(&commandeer.encodedAttributes, "attrs", "{}", "JSON-encoded attributes for the api gateway (overrides all the rest)")

	commandeer.cmd = cmd

	return commandeer
}

type createFunctionEventCommandeer struct {
	*createCommandeer
	functionEventConfig platform.FunctionEventConfig
	encodedAttributes   string
	functionName        string
}

func newCreateFunctionEventCommandeer(createCommandeer *createCommandeer) *createFunctionEventCommandeer {
	commandeer := &createFunctionEventCommandeer{
		createCommandeer: createCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "functionevent name",
		Aliases: []string{"fe"},
		Short:   "Create function events",
		RunE: func(cmd *cobra.Command, args []string) error {

			// if we got positional arguments
			if len(args) != 1 {
				return errors.New("Function event create requires an identifier")
			}

			if commandeer.functionName == "" {
				return errors.New("Function event must belong to a function")
			}

			// initialize root
			if err := createCommandeer.rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			commandeer.functionEventConfig.Meta.Name = args[0]
			commandeer.functionEventConfig.Meta.Namespace = createCommandeer.rootCommandeer.namespace
			commandeer.functionEventConfig.Meta.Labels = map[string]string{
				"nuclio.io/function-name": commandeer.functionName,
			}

			// decode the JSON attributes
			if err := json.Unmarshal([]byte(commandeer.encodedAttributes),
				&commandeer.functionEventConfig.Spec.Attributes); err != nil {
				return errors.Wrap(err, "Failed to decode a function's event attributes")
			}

			if err := createCommandeer.rootCommandeer.platform.CreateFunctionEvent(context.Background(),
				&platform.CreateFunctionEventOptions{
					FunctionEventConfig: commandeer.functionEventConfig,
				}); err != nil {
				return err
			}

			commandeer.rootCommandeer.loggerInstance.InfoWith("Function event created",
				"Name",
				commandeer.functionEventConfig.Meta.Name,
				"Namespace",
				commandeer.functionEventConfig.Meta.Namespace)
			return nil
		},
	}

	cmd.Flags().StringVar(&commandeer.functionName, "function", "", "function this event belongs to")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.DisplayName, "display-name", "", "display name, if different than name (optional)")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.TriggerName, "trigger-name", "", "trigger name to invoke (optional)")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.TriggerKind, "trigger-kind", "", "trigger kind to invoke (optional)")
	cmd.Flags().StringVar(&commandeer.functionEventConfig.Spec.Body, "body", "", "body content to invoke the function with")
	cmd.Flags().StringVar(&commandeer.encodedAttributes, "attrs", "{}", "JSON-encoded attributes for the function event")

	commandeer.cmd = cmd

	return commandeer
}
