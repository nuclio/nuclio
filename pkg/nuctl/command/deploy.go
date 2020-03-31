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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"

	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type deployCommandeer struct {
	cmd                             *cobra.Command
	rootCommandeer                  *RootCommandeer
	functionConfig                  functionconfig.Config
	volumes                         stringSliceFlag
	commands                        stringSliceFlag
	encodedDataBindings             string
	encodedTriggers                 string
	encodedLabels                   string
	encodedRuntimeAttributes        string
	projectName                     string
	resourceLimits                  stringSliceFlag
	resourceRequests                stringSliceFlag
	encodedEnv                      stringSliceFlag
	encodedFunctionPlatformConfig   string
	encodedBuildRuntimeAttributes   string
	encodedBuildCodeEntryAttributes string
	inputImageFile                  string
	loggerLevel                     string
	replicas                        int
	minReplicas                     int
	maxReplicas                     int
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
			var err error

			// update build stuff
			if len(args) == 1 {
				commandeer.functionConfig.Meta.Name = args[0]
			}

			// parse volumes
			volumes, err := parseVolumes(commandeer.volumes)
			if err != nil {
				return errors.Wrap(err, "Failed to parse volumes")
			}
			commandeer.functionConfig.Spec.Volumes = append(commandeer.functionConfig.Spec.Volumes, volumes...)

			// parse resource limits
			if err := parseResourceAllocations(commandeer.resourceLimits,
				&commandeer.functionConfig.Spec.Resources.Limits); err != nil {
				return errors.Wrap(err, "Failed to parse resource limits")
			}

			// parse resource requests
			if err := parseResourceAllocations(commandeer.resourceRequests,
				&commandeer.functionConfig.Spec.Resources.Requests); err != nil {
				return errors.Wrap(err, "Failed to parse resource requests")
			}

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

			// decode the JSON function platform configuration
			if err := json.Unmarshal([]byte(commandeer.encodedFunctionPlatformConfig),
				&commandeer.functionConfig.Spec.Platform); err != nil {
				return errors.Wrap(err, "Failed to decode function platform configuration")
			}

			// decode the JSON runtime attributes
			if err := json.Unmarshal([]byte(commandeer.encodedRuntimeAttributes),
				&commandeer.functionConfig.Spec.RuntimeAttributes); err != nil {
				return errors.Wrap(err, "Failed to decode runtime attributes")
			}

			// decode the JSON build runtime attributes
			if err := json.Unmarshal([]byte(commandeer.encodedBuildRuntimeAttributes),
				&commandeer.functionConfig.Spec.Build.RuntimeAttributes); err != nil {
				return errors.Wrap(err, "Failed to decode build runtime attributes")
			}

			// decode the JSON build code entry attributes
			if err := json.Unmarshal([]byte(commandeer.encodedBuildCodeEntryAttributes),
				&commandeer.functionConfig.Spec.Build.CodeEntryAttributes); err != nil {
				return errors.Wrap(err, "Failed to decode code entry attributes")
			}

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			// decode labels
			commandeer.functionConfig.Meta.Labels = common.StringToStringMap(commandeer.encodedLabels, "=")

			// if the project name was set, add it as a label
			if commandeer.projectName != "" {
				commandeer.functionConfig.Meta.Labels["nuclio.io/project-name"] = commandeer.projectName
			}

			// decode env
			for _, encodedEnvNameAndValue := range commandeer.encodedEnv {
				envNameAndValue := strings.SplitN(encodedEnvNameAndValue, "=", 2)
				if len(envNameAndValue) != 2 {
					return errors.Errorf("Environment variable must be in the form of name=value: %s",
						encodedEnvNameAndValue)
				}

				commandeer.functionConfig.Spec.Env = append(commandeer.functionConfig.Spec.Env, v1.EnvVar{
					Name:  envNameAndValue[0],
					Value: envNameAndValue[1],
				})
			}

			// check if logger level is set
			if commandeer.loggerLevel != "" {
				commandeer.functionConfig.Spec.LoggerSinks = []functionconfig.LoggerSink{
					{Level: commandeer.loggerLevel},
				}
			}

			// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.Replicas as nil)
			if commandeer.replicas >= 0 {
				commandeer.functionConfig.Spec.Replicas = &commandeer.replicas
			}

			// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.MinReplicas as nil)
			if commandeer.minReplicas >= 0 {
				commandeer.functionConfig.Spec.MinReplicas = &commandeer.minReplicas
			}

			// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.MaxReplicas as nil)
			if commandeer.maxReplicas >= 0 {
				commandeer.functionConfig.Spec.MaxReplicas = &commandeer.maxReplicas
			}

			// update function
			commandeer.functionConfig.Meta.Namespace = rootCommandeer.namespace
			commandeer.functionConfig.Spec.Build.Commands = commandeer.commands

			_, err = rootCommandeer.platform.CreateFunction(&platform.CreateFunctionOptions{
				Logger:         rootCommandeer.loggerInstance,
				FunctionConfig: commandeer.functionConfig,
				InputImageFile: commandeer.inputImageFile,
			})

			return err
		},
	}

	addDeployFlags(cmd, &commandeer.functionConfig, commandeer)
	cmd.Flags().StringVarP(&commandeer.inputImageFile, "input-image-file", "", "", "Path to input of docker archive")

	commandeer.cmd = cmd

	return commandeer
}

func addDeployFlags(cmd *cobra.Command,
	functionConfig *functionconfig.Config,
	commandeer *deployCommandeer) {
	addBuildFlags(cmd, functionConfig, &commandeer.commands, &commandeer.encodedBuildRuntimeAttributes, &commandeer.encodedBuildCodeEntryAttributes)

	cmd.Flags().StringVar(&functionConfig.Spec.Description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&commandeer.encodedLabels, "labels", "l", "", "Additional function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.Flags().VarP(&commandeer.encodedEnv, "env", "e", "Environment variables env1=val1")
	cmd.Flags().BoolVarP(&functionConfig.Spec.Disable, "disable", "d", false, "Start the function as disabled (don't run yet)")
	cmd.Flags().IntVarP(&commandeer.replicas, "replicas", "", -1, "Set to any non-negative integer to use a static number of replicas")
	cmd.Flags().IntVar(&commandeer.minReplicas, "min-replicas", -1, "Minimal number of function replicas")
	cmd.Flags().IntVar(&commandeer.maxReplicas, "max-replicas", -1, "Maximal number of function replicas")
	cmd.Flags().IntVar(&functionConfig.Spec.TargetCPU, "target-cpu", abstract.DefaultTargetCPU, "Target CPU when auto-scaling, in percentage")
	cmd.Flags().BoolVar(&functionConfig.Spec.Publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(&commandeer.encodedDataBindings, "data-bindings", "{}", "JSON-encoded data bindings for the function")
	cmd.Flags().StringVar(&commandeer.encodedTriggers, "triggers", "{}", "JSON-encoded triggers for the function")
	cmd.Flags().StringVar(&commandeer.encodedFunctionPlatformConfig, "platform-config", "{}", "JSON-encoded platform specific configuration")
	cmd.Flags().StringVar(&functionConfig.Spec.Image, "run-image", "", "Name of an existing image to deploy (default - build a new image to deploy)")
	cmd.Flags().StringVar(&functionConfig.Spec.RunRegistry, "run-registry", os.Getenv("NUCTL_RUN_REGISTRY"), "URL of a registry for pulling the image, if differs from -r/--registry (env: NUCTL_RUN_REGISTRY)")
	cmd.Flags().StringVar(&commandeer.encodedRuntimeAttributes, "runtime-attrs", "{}", "JSON-encoded runtime attributes for the function")
	cmd.Flags().IntVar(&functionConfig.Spec.ReadinessTimeoutSeconds, "readiness-timeout", abstract.DefaultReadinessTimeoutSeconds, "maximum wait time for the function to be ready")
	cmd.Flags().StringVar(&commandeer.projectName, "project-name", "", "name of project to which this function belongs to")
	cmd.Flags().Var(&commandeer.volumes, "volume", "Volumes for the deployment function (src1=dest1[,src2=dest2,...])")
	cmd.Flags().Var(&commandeer.resourceLimits, "resource-limit", "Limits resources in the format of resource-name=quantity (e.g. cpu=3)")
	cmd.Flags().Var(&commandeer.resourceRequests, "resource-request", "Requests resources in the format of resource-name=quantity (e.g. cpu=3)")
	cmd.Flags().StringVar(&commandeer.loggerLevel, "logger-level", "", "One of debug, info, warn, error. By default, uses platform configuration")
}

func parseResourceAllocations(values stringSliceFlag, resources *v1.ResourceList) error {
	for _, value := range values {

		// split the value @ =
		resourceNameAndQuantity := strings.Split(value, "=")

		// must be exactly 2 (resource name, quantity)
		if len(resourceNameAndQuantity) != 2 {
			return errors.Errorf("Resource allocation %s not in the format of resource-name=quantity", value)
		}

		resourceName := v1.ResourceName(resourceNameAndQuantity[0])
		resourceQuantityString := resourceNameAndQuantity[1]

		resourceQuantity, err := resource.ParseQuantity(resourceQuantityString)
		if err != nil {
			return errors.Wrap(err, "Failed to parse quantity")
		}

		if *resources == nil {
			*resources = v1.ResourceList{}
		}

		// set resource
		(*resources)[resourceName] = resourceQuantity
	}

	return nil
}

func parseVolumes(volumes stringSliceFlag) ([]functionconfig.Volume, error) {
	var originVolumes []functionconfig.Volume
	for volumeIndex, volume := range volumes {

		// decode volumes
		volumeSrcAndDestination := strings.Split(volume, ":")

		// must be exactly 2 (resource name, quantity)
		if len(volumeSrcAndDestination) != 2 || len(volumeSrcAndDestination[0]) == 0 || len(volumeSrcAndDestination[1]) == 0 {
			return []functionconfig.Volume{}, errors.Errorf("Volume format %s not in the format of volume-src:volume-destination", volumeSrcAndDestination)
		}

		// generate simple volume name
		volumeName := fmt.Sprintf("volume-%v", volumeIndex+1)

		originVolumes = append(originVolumes,
			functionconfig.Volume{
				Volume: v1.Volume{
					Name: volumeName,
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: volumeSrcAndDestination[0],
						},
					},
				},
				VolumeMount: v1.VolumeMount{
					Name:      volumeName,
					MountPath: volumeSrcAndDestination[1],
				},
			},
		)

	}

	return originVolumes, nil
}
