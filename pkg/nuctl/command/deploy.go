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

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
)

type deployCommandeer struct {
	cmd                 *cobra.Command
	rootCommandeer      *RootCommandeer
	deployOptions       *platform.DeployOptions
	commands            stringSliceFlag
	encodedDataBindings string
	encodedTriggers     string
	encodedIngresses    string
}

func newDeployCommandeer(rootCommandeer *RootCommandeer) *deployCommandeer {
	commandeer := &deployCommandeer{
		rootCommandeer: rootCommandeer,
	}

	commandeer.deployOptions = platform.NewDeployOptions(rootCommandeer.commonOptions)

	cmd := &cobra.Command{
		Use:   "deploy function-name",
		Short: "Build, deploy and run a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// decode the JSON data bindings
			if err := json.Unmarshal([]byte(commandeer.encodedDataBindings),
				&commandeer.deployOptions.DataBindings); err != nil {
				return errors.Wrap(err, "Failed to decode data bindings")
			}

			// decode the JSON triggers
			if err := json.Unmarshal([]byte(commandeer.encodedTriggers),
				&commandeer.deployOptions.Triggers); err != nil {
				return errors.Wrap(err, "Failed to decode triggers")
			}

			// decode the JSON ingresses
			if err := json.Unmarshal([]byte(commandeer.encodedIngresses),
				&commandeer.deployOptions.Ingresses); err != nil {
				return errors.Wrap(err, "Failed to decode ingresses")
			}

			// update build stuff
			commandeer.deployOptions.Build.Commands = commandeer.commands

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			err := prepareDeployerOptions(args,
				rootCommandeer.platform.GetDeployRequiresRegistry(),
				rootCommandeer.commonOptions,
				commandeer.deployOptions)

			if err != nil {
				return err
			}

			_, err = rootCommandeer.platform.DeployFunction(commandeer.deployOptions)
			return err
		},
	}

	addDeployFlags(cmd,
		commandeer.deployOptions,
		&commandeer.commands,
		&commandeer.encodedDataBindings,
		&commandeer.encodedTriggers,
		&commandeer.encodedIngresses)

	commandeer.cmd = cmd

	return commandeer
}

func prepareDeployerOptions(args []string,
	registryRequired bool,
	commonOptions *platform.CommonOptions,
	deployOptions *platform.DeployOptions) error {

	var functionName string
	var specRegistryURL, specImageName, specImageVersion string

	// name can either be a positional argument or passed in the spec
	if len(args) != 1 {
		return errors.New("Function run requires name")
	}

	functionName = args[0]

	// function can either be in the path or received inline
	if deployOptions.Build.Path == "" && deployOptions.ImageName == "" {
		return errors.New("Function code must be provided either in path or inline in a spec file. Alternatively, an image may be provided")
	}

	if deployOptions.Build.Registry == "" && registryRequired {
		return errors.New("Registry is required (can also be specified in spec.image or a NUCTL_REGISTRY env var")
	}

	if deployOptions.Build.ImageName == "" {

		// use the function name if image name not provided in specfile
		deployOptions.Build.ImageName = functionName
	}

	// if the image name was not provided in command line / env, take it from the spec image
	if deployOptions.Build.ImageName == "" {
		deployOptions.Build.ImageName = specImageName
	}

	// same for version
	if deployOptions.Build.ImageVersion == "latest" && specImageVersion != "" {
		deployOptions.Build.ImageVersion = specImageVersion
	}

	// same for push registry
	if deployOptions.Build.Registry == "" {
		deployOptions.Build.Registry = specRegistryURL
	}

	// if the run registry wasn't specified, take the build registry
	if deployOptions.RunRegistry == "" {
		deployOptions.RunRegistry = deployOptions.Build.Registry
	}

	// set function name
	deployOptions.Identifier = functionName

	return nil
}

func addDeployFlags(cmd *cobra.Command,
	options *platform.DeployOptions,
	commands *stringSliceFlag,
	encodedDataBindings *string,
	encodedTriggers *string,
	encodedIngresses *string) {
	addBuildFlags(cmd, &options.Build, commands)

	cmd.Flags().StringVar(&options.Description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&options.Labels, "labels", "l", "", "Additional function labels (lbl1=val1,lbl2=val2..)")
	cmd.Flags().StringVarP(&options.Env, "env", "e", "", "Environment variables (name1=val1,name2=val2..)")
	cmd.Flags().StringVar(&options.Data, "data", "", "Comma separated list of data bindings (in json)")
	cmd.Flags().BoolVarP(&options.Disabled, "disabled", "d", false, "Start function disabled (don't run yet)")
	cmd.Flags().IntVar(&options.HTTPPort, "port", 0, "Public HTTP port (node port)")
	cmd.Flags().IntVarP(&options.Replicas, "replicas", "", 1, "If set, number of replicas is static")
	cmd.Flags().IntVar(&options.MinReplicas, "min-replicas", 0, "Minimum number of function replicas")
	cmd.Flags().IntVar(&options.MaxReplicas, "max-replicas", 0, "Maximum number of function replicas")
	cmd.Flags().BoolVar(&options.Publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(encodedDataBindings, "data-bindings", "{}", "JSON encoded data bindings for the function")
	cmd.Flags().StringVar(encodedTriggers, "triggers", "{}", "JSON encoded triggers for the function")
	cmd.Flags().StringVar(encodedIngresses, "ingresses", "{}", "JSON encoded ingresses for the function")
	cmd.Flags().StringVar(&options.ImageName, "run-image", "", "If specified, this is the image that the deploy will use, rather than try to build one")
	cmd.Flags().StringVar(&options.RunRegistry, "run-registry", os.Getenv("NUCTL_RUN_REGISTRY"), "The registry URL to pull the image from, if differs from -r (env: NUCTL_RUN_REGISTRY)")
}
