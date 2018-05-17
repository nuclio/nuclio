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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type deployCommandeer struct {
	cmd                           *cobra.Command
	rootCommandeer                *RootCommandeer
	functionConfig                functionconfig.Config
	readinessTimeout              time.Duration
	volumes                       stringSliceFlag
	commands                      stringSliceFlag
	encodedDataBindings           string
	encodedTriggers               string
	encodedLabels                 string
	encodedRuntimeAttributes      string
	projectName                   string
	resourceLimits                stringSliceFlag
	resourceRequests              stringSliceFlag
	encodedEnv                    stringSliceFlag
	encodedFunctionPlatformConfig string
	encodedBuildRuntimeAttributes string
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

			// update build stuff
			if len(args) == 1 {
				commandeer.functionConfig.Meta.Name = args[0]
			}

			// parse volumes
			if err := parseVolumes(commandeer.volumes, commandeer.functionConfig.Spec.Volumes); err != nil {
				return errors.Wrap(err, "Failed to parse volumes")
			}

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
					return fmt.Errorf("Environment variable must be in the form of name=value: %s",
						encodedEnvNameAndValue)
				}

				commandeer.functionConfig.Spec.Env = append(commandeer.functionConfig.Spec.Env, v1.EnvVar{
					Name:  envNameAndValue[0],
					Value: envNameAndValue[1],
				})
			}

			// update function
			commandeer.functionConfig.Meta.Namespace = rootCommandeer.namespace
			commandeer.functionConfig.Spec.Build.Commands = commandeer.commands

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			_, err := rootCommandeer.platform.CreateFunction(&platform.CreateFunctionOptions{
				Logger:           rootCommandeer.loggerInstance,
				FunctionConfig:   commandeer.functionConfig,
				ReadinessTimeout: &commandeer.readinessTimeout,
			})

			return err
		},
	}

	addDeployFlags(cmd, &commandeer.functionConfig, commandeer)

	commandeer.cmd = cmd

	return commandeer
}

func addDeployFlags(cmd *cobra.Command,
	functionConfig *functionconfig.Config,
	commandeer *deployCommandeer) {
	addBuildFlags(cmd, functionConfig, &commandeer.commands, &commandeer.encodedBuildRuntimeAttributes)

	cmd.Flags().StringVar(&functionConfig.Spec.Description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&commandeer.encodedLabels, "labels", "l", "", "Additional function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.Flags().VarP(&commandeer.encodedEnv, "env", "e", "Environment variables env1=val1")
	cmd.Flags().BoolVarP(&functionConfig.Spec.Disabled, "disabled", "d", false, "Start the function as disabled (don't run yet)")
	cmd.Flags().IntVarP(&functionConfig.Spec.Replicas, "replicas", "", 1, "Set to 1 to use a static number of replicas")
	cmd.Flags().IntVar(&functionConfig.Spec.MinReplicas, "min-replicas", 0, "Minimal number of function replicas")
	cmd.Flags().IntVar(&functionConfig.Spec.MaxReplicas, "max-replicas", 0, "Maximal number of function replicas")
	cmd.Flags().IntVar(&functionConfig.Spec.TargetCPU, "target-cpu", 75, "Target CPU when auto-scaling, in percentage")
	cmd.Flags().BoolVar(&functionConfig.Spec.Publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(&commandeer.encodedDataBindings, "data-bindings", "{}", "JSON-encoded data bindings for the function")
	cmd.Flags().StringVar(&commandeer.encodedTriggers, "triggers", "{}", "JSON-encoded triggers for the function")
	cmd.Flags().StringVar(&commandeer.encodedFunctionPlatformConfig, "platform-config", "{}", "JSON-encoded platform specific configuration")
	cmd.Flags().StringVar(&functionConfig.Spec.Image, "run-image", "", "Name of an existing image to deploy (default - build a new image to deploy)")
	cmd.Flags().StringVar(&functionConfig.Spec.RunRegistry, "run-registry", os.Getenv("NUCTL_RUN_REGISTRY"), "URL of a registry for pulling the image, if differs from -r/--registry (env: NUCTL_RUN_REGISTRY)")
	cmd.Flags().StringVar(&commandeer.encodedRuntimeAttributes, "runtime-attrs", "{}", "JSON-encoded runtime attributes for the function")
	cmd.Flags().DurationVar(&commandeer.readinessTimeout, "readiness-timeout", 30*time.Second, "maximum wait time for the function to be ready")
	cmd.Flags().StringVar(&commandeer.projectName, "project-name", "", "name of project to which this function belongs to")
	cmd.Flags().Var(&commandeer.volumes, "volume", "Volumes for the function (src1=dest1[,src2=dest2,...])")
	cmd.Flags().Var(&commandeer.resourceLimits, "resource-limit", "Limits resources in the format of resource-name=quantity (e.g. cpu=3)")
	cmd.Flags().Var(&commandeer.resourceRequests, "resource-request", "Requests resources in the format of resource-name=quantity (e.g. cpu=3)")
}

func parseResourceAllocations(values stringSliceFlag, resources *v1.ResourceList) error {
	for _, value := range values {

		// split the value @ =
		resourceNameAndQuantity := strings.Split(value, "=")

		// must be exactly 2 (resource name, quantity)
		if len(resourceNameAndQuantity) != 2 {
			return fmt.Errorf("Resource allocation %s not in the format of resource-name=quantity", value)
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

func parseVolumes(volumes stringSliceFlag, originVolumes []functionconfig.Volume) error {
	for volumeIndex, volume := range volumes {

		// decode volumes
		volumeSrcAndDestination := strings.Split(volume, ":")

		// must be exactly 2 (resource name, quantity)
		if len(volumeSrcAndDestination) != 2 || len(volumeSrcAndDestination[0]) == 0 || len(volumeSrcAndDestination[1]) == 0 {
			return fmt.Errorf("Volume format %s not in the format of volume-src:volume-destination", volumeSrcAndDestination)
		}

		// generate simple volume name
		volumeName := fmt.Sprintf("volume-%v", volumeIndex+1)

		// if originVolumes is nil generate empty one
		if originVolumes == nil {
			originVolumes = []functionconfig.Volume{}
		}

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

	return nil
}
