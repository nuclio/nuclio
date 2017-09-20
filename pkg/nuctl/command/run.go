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
	"fmt"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"

	"github.com/spf13/cobra"
)

type runCommandeer struct {
	cmd                 *cobra.Command
	rootCommandeer      *RootCommandeer
	runOptions          runner.Options
	encodedDataBindings string
}

func newRunCommandeer(rootCommandeer *RootCommandeer) *runCommandeer {
	commandeer := &runCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "run function-name",
		Short: "Build, deploy and run a function",
		RunE: func(cmd *cobra.Command, args []string) error {

			// decode the JSON data bindings
			if err := json.Unmarshal([]byte(commandeer.encodedDataBindings),
				&commandeer.runOptions.DataBindings); err != nil {
				return errors.Wrap(err, "Failed to decode data bindings")
			}

			err := prepareRunnerOptions(args, &rootCommandeer.commonOptions, &commandeer.runOptions)
			if err != nil {
				return err
			}

			// create logger
			logger, err := rootCommandeer.createLogger()
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			// create function runner and execute
			functionRunner, err := runner.NewFunctionRunner(logger)
			if err != nil {
				return errors.Wrap(err, "Failed to create function runner")
			}

			// create a kube consumer - a bunch of kubernetes clients
			kubeConsumer, err := nuctl.NewKubeConsumer(logger, commandeer.runOptions.Common.KubeconfigPath)
			if err != nil {
				return errors.Wrap(err, "Failed to create kubeconsumer")
			}

			_, err = functionRunner.Run(kubeConsumer, &commandeer.runOptions)
			return err
		},
	}

	addRunFlags(cmd, &commandeer.runOptions, &commandeer.encodedDataBindings)

	commandeer.cmd = cmd

	return commandeer
}

func prepareRunnerOptions(args []string,
	commonOptions *nuctl.CommonOptions,
	runOptions *runner.Options) error {

	functionName := ""
	var specRegistryURL, specImageName, specImageVersion string
	var err error

	// if the spec path was set, load the spec
	if runOptions.SpecPath != "" {
		err := functioncr.FromSpecFile(runOptions.SpecPath, &runOptions.Spec)
		if err != nil {
			return errors.Wrap(err, "Failed to read spec file")
		}
	}

	// name can either be a positional argument or passed in the spec
	if len(args) != 1 {
		if runOptions.Spec.ObjectMeta.Name == "" {
			return errors.New("Function run requires name")
		}

		// use name from spec
		functionName = runOptions.Spec.ObjectMeta.Name

	} else {
		functionName = args[0]
	}

	// function can either be in the path or received inline
	if runOptions.Build.Path == "" && runOptions.Spec.Spec.Code.Inline == "" {
		return errors.New("Function code must be provided either in path or inline in a spec file")
	}

	// the image in the specfile can hold both the image name and the push/run registry. check that if it's
	// empty, we have what we need from command line arguments
	if runOptions.Spec.Spec.Image == "" {

		if runOptions.Build.Registry == "" {
			return errors.New("Registry is required (can also be specified in spec.image or a NUCTL_REGISTRY env var")
		}

		if runOptions.Build.ImageName == "" {

			// use the function name if image name not provided in specfile
			runOptions.Build.ImageName = functionName
		}
	} else {

		// parse the image passed in the spec - we might need it
		specRegistryURL, specImageName, specImageVersion, err = parseImageURL(runOptions.Spec.Spec.Image)
		if err != nil {
			return fmt.Errorf("Failed to parse image URL: %s", err.Error())
		}
	}

	// if the image name was not provided in command line / env, take it from the spec image
	if runOptions.Build.ImageName == "" {
		runOptions.Build.ImageName = specImageName
	}

	// same for version
	if runOptions.Build.ImageVersion == "latest" && specImageVersion != "" {
		runOptions.Build.ImageVersion = specImageVersion
	}

	// same for push registry
	if runOptions.Build.Registry == "" {
		runOptions.Build.Registry = specRegistryURL
	}

	// if the run registry wasn't specified, take the build registry
	if runOptions.RunRegistry == "" {
		runOptions.RunRegistry = runOptions.Build.Registry
	}

	// set common
	runOptions.Build.Common = commonOptions
	runOptions.Common = commonOptions
	runOptions.Common.Identifier = functionName

	return nil
}

func parseImageURL(imageURL string) (url string, imageName string, imageVersion string, err error) {
	urlAndImageName := strings.SplitN(imageURL, "/", 2)

	if len(urlAndImageName) != 2 {
		err = errors.New("Failed looking for image splitter: /")
		return
	}

	url = urlAndImageName[0]
	imageNameAndVersion := strings.Split(urlAndImageName[1], ":")
	imageName = imageNameAndVersion[0]

	if len(imageNameAndVersion) == 1 {
		imageVersion = "latest"
	} else if len(imageNameAndVersion) == 2 {
		imageVersion = imageNameAndVersion[1]
	}

	return
}

func addRunFlags(cmd *cobra.Command, options *runner.Options, encodedDataBindings *string) {
	addBuildFlags(cmd, &options.Build)

	cmd.Flags().StringVarP(&options.SpecPath, "file", "f", "", "Function Spec File")
	cmd.Flags().StringVar(&options.Description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&options.Scale, "scale", "s", "1", "Function scaling (auto|number)")
	cmd.Flags().StringVarP(&options.Labels, "labels", "l", "", "Additional function labels (lbl1=val1,lbl2=val2..)")
	cmd.Flags().StringVarP(&options.Env, "env", "e", "", "Environment variables (name1=val1,name2=val2..)")
	cmd.Flags().StringVar(&options.Events, "events", "", "Comma separated list of event sources (in json)")
	cmd.Flags().StringVar(&options.Data, "data", "", "Comma separated list of data bindings (in json)")
	cmd.Flags().BoolVarP(&options.Disabled, "disabled", "d", false, "Start function disabled (don't run yet)")
	cmd.Flags().Int32Var(&options.HTTPPort, "port", 0, "Public HTTP port (node port)")
	cmd.Flags().Int32Var(&options.MinReplicas, "min-replica", 0, "Minimum number of function replicas")
	cmd.Flags().Int32Var(&options.MaxReplicas, "max-replica", 0, "Maximum number of function replicas")
	cmd.Flags().BoolVar(&options.Publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(encodedDataBindings, "data-bindings", "{}", "JSON encoded data bindings for the function")
	cmd.Flags().StringVar(&options.RunRegistry, "run-registry", os.Getenv("NUCTL_RUN_REGISTRY"), "The registry URL to pull the image from, if differs from -r (env: NUCTL_RUN_REGISTRY)")
}
