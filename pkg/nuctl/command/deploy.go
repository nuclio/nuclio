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
	"encoding/json"
	"os"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
)

type deployCommandeer struct {
	cmd                      *cobra.Command
	rootCommandeer           *RootCommandeer
	functionConfig           functionconfig.Config
	commands                 stringSliceFlag
	encodedDataBindings      string
	encodedTriggers          string
	encodedLabels            string
	encodedEnv               string
	encodedRuntimeAttributes string
}

func newDeployCommandeer(rootCommandeer *RootCommandeer) *deployCommandeer {
	commandeer := &deployCommandeer{
		rootCommandeer: rootCommandeer,
		functionConfig: *functionconfig.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   "deploy function-name",
		Short: "Build and deploy a function, or deploy from an existing image",
		RunE: func(cmd *cobra.Command, args []string) error {

			// decode the JSON data bindings
			if err := json.Unmarshal([]byte(commandeer.encodedDataBindings),
				&commandeer.functionConfig.Spec.DataBindings); err != nil {
				return errors.Wrap(err, "Failed to decode data bindings")
			}

			// decode the JSON triggers
			if err := json.Unmarshal([]byte(commandeer.encodedTriggers),
				&commandeer.functionConfig.Spec.Triggers); err != nil {
				return errors.Wrap(err, "Failed to decode triggers")
			}

			// decode the JSON runtime attributes
			if err := json.Unmarshal([]byte(commandeer.encodedRuntimeAttributes),
				&commandeer.functionConfig.Spec.RuntimeAttributes); err != nil {
				return errors.Wrap(err, "Failed to decode runtime attributes")
			}

			// decode labels
			commandeer.functionConfig.Meta.Labels = common.StringToStringMap(commandeer.encodedLabels)

			// decode env
			for envName, envValue := range common.StringToStringMap(commandeer.encodedEnv) {
				commandeer.functionConfig.Spec.Env = append(commandeer.functionConfig.Spec.Env, v1.EnvVar{
					Name:  envName,
					Value: envValue,
				})
			}

			// update function
			commandeer.functionConfig.Meta.Namespace = rootCommandeer.namespace
			commandeer.functionConfig.Spec.Build.Commands = commandeer.commands

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			err := prepareFunctionConfig(args,
				rootCommandeer.platform.GetDeployRequiresRegistry(),
				&commandeer.functionConfig)

			if err != nil {
				return err
			}

			_, err = rootCommandeer.platform.DeployFunction(&platform.DeployOptions{
				Logger:         rootCommandeer.logger,
				FunctionConfig: commandeer.functionConfig,
			})

			return err
		},
	}

	addDeployFlags(cmd, &commandeer.functionConfig, commandeer)

	commandeer.cmd = cmd

	return commandeer
}

func prepareFunctionConfig(args []string,
	registryRequired bool,
	functionConfig *functionconfig.Config) error {

	var functionName string
	var specRegistryURL, specImageName, specImageVersion string

	// name can either be a positional argument or passed in the spec
	if len(args) != 1 {
		return errors.New("Function run requires a name")
	}

	functionName = args[0]

	// function can either be in the path, received inline or an executable via handler
	if functionConfig.Spec.Build.Path == "" &&
		functionConfig.Spec.ImageName == "" {

		if functionConfig.Spec.Runtime != "shell" {
			return errors.New("Function path must be provided when specified runtime isn't shell")

		}

		// did user give handler to an executable
		if functionConfig.Spec.Handler == "" {
			return errors.New("If shell runtime is specified, function path or handler name must be provided")
		}
	}

	if functionConfig.Spec.Build.Registry == "" && registryRequired {
		return errors.New("A registry is required; can also be specified in spec.image or via a NUCTL_REGISTRY environment variable")
	}

	if functionConfig.Spec.Build.ImageName == "" {

		// use the function name if image name not provided in specfile
		functionConfig.Spec.Build.ImageName = functionName
	}

	// if the image name was not provided in command line / env, take it from the spec image
	if functionConfig.Spec.Build.ImageName == "" {
		functionConfig.Spec.Build.ImageName = specImageName
	}

	// same for version
	if functionConfig.Spec.Build.ImageVersion == "latest" && specImageVersion != "" {
		functionConfig.Spec.Build.ImageVersion = specImageVersion
	}

	// same for push registry
	if functionConfig.Spec.Build.Registry == "" {
		functionConfig.Spec.Build.Registry = specRegistryURL
	}

	// if the run registry wasn't specified, take the build registry
	if functionConfig.Spec.RunRegistry == "" {
		functionConfig.Spec.RunRegistry = functionConfig.Spec.Build.Registry
	}

	// set function name
	functionConfig.Meta.Name = functionName

	return nil
}

func addDeployFlags(cmd *cobra.Command,
	functionConfig *functionconfig.Config,
	commandeer *deployCommandeer) {
	addBuildFlags(cmd, functionConfig, &commandeer.commands)

	cmd.Flags().StringVar(&functionConfig.Spec.Description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&commandeer.encodedLabels, "labels", "l", "", "Additional function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.Flags().StringVarP(&commandeer.encodedEnv, "env", "e", "", "Environment variables (env1=val1[,env2=val2,...])")
	cmd.Flags().BoolVarP(&functionConfig.Spec.Disabled, "disabled", "d", false, "Start the function as disabled (don't run yet)")
	cmd.Flags().IntVar(&functionConfig.Spec.HTTPPort, "port", 0, "Public HTTP port (NodePort)")
	cmd.Flags().IntVarP(&functionConfig.Spec.Replicas, "replicas", "", 1, "Set to 1 to use a static number of replicas")
	cmd.Flags().IntVar(&functionConfig.Spec.MinReplicas, "min-replicas", 0, "Minimal number of function replicas")
	cmd.Flags().IntVar(&functionConfig.Spec.MaxReplicas, "max-replicas", 0, "Maximal number of function replicas")
	cmd.Flags().BoolVar(&functionConfig.Spec.Publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(&commandeer.encodedDataBindings, "data-bindings", "{}", "JSON-encoded data bindings for the function")
	cmd.Flags().StringVar(&commandeer.encodedTriggers, "triggers", "{}", "JSON-encoded triggers for the function")
	cmd.Flags().StringVar(&functionConfig.Spec.ImageName, "run-image", "", "Name of an existing image to deploy (default - build a new image to deploy)")
	cmd.Flags().StringVar(&functionConfig.Spec.RunRegistry, "run-registry", os.Getenv("NUCTL_RUN_REGISTRY"), "URL of a registry for pulling the image, if differs from -r/--registry (env: NUCTL_RUN_REGISTRY)")
	cmd.Flags().StringVar(&commandeer.encodedRuntimeAttributes, "runtime-attrs", "{}", "JSON-encoded runtime attributes for the function")
}
