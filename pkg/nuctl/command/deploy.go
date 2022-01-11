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
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
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
	functionBuild                   functionconfig.Build
	functionName                    string
	functionConfigPath              string
	description                     string
	disable                         bool
	publish                         bool
	handler                         string
	runtime                         string
	image                           string
	targetCPU                       int
	runRegistry                     string
	readinessTimeoutSeconds         int
	volumes                         stringSliceFlag
	commands                        stringSliceFlag
	encodedDataBindings             string
	encodedTriggers                 string
	encodedLabels                   string
	encodedAnnotations              string
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
	priorityClassName               string
	preemptionPolicy                string
	nodeName                        string
	encodedNodeSelector             string
	runAsUser                       int64
	runAsGroup                      int64
	fsGroup                         int64
	overrideHTTPTriggerServiceType  string
}

func newDeployCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *deployCommandeer {
	commandeer := &deployCommandeer{
		rootCommandeer: rootCommandeer,
		functionConfig: *functionconfig.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   "deploy function-name",
		Short: "Build and deploy a function, or deploy from an existing image",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			var importedFunction platform.Function

			// update build stuff
			if len(args) == 1 {
				commandeer.functionName = args[0]

				importedFunction, err = commandeer.getImportedFunction(ctx, args[0])
				if err != nil {
					return errors.Wrap(err, "Failed getting the imported function's data")
				}
				if importedFunction != nil {
					commandeer.rootCommandeer.loggerInstance.Debug("Function was already imported, deploying it")
					commandeer.functionConfig = commandeer.prepareFunctionConfigForRedeploy(importedFunction)
				}
			}

			// If config file is provided
			if importedFunction == nil && commandeer.functionConfigPath != "" {
				commandeer.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Loading function config from file", "file", commandeer.functionConfigPath)
				functionConfigFile, err := nuctlcommon.OpenFile(commandeer.functionConfigPath)
				if err != nil {
					return errors.Wrap(err, "Failed opening function config file")
				}

				functionBody, err := ioutil.ReadAll(functionConfigFile)
				if err != nil {
					return errors.Wrap(err, "Failed reading function config file")
				}

				unmarshalFunc, err := nuctlcommon.GetUnmarshalFunc(functionBody)
				if err != nil {
					return errors.Wrap(err, "Failed identifying function config file format")
				}

				err = unmarshalFunc(functionBody, &commandeer.functionConfig)
				if err != nil {
					return errors.Wrap(err, "Failed parsing function config file")
				}

				commandeer.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Successfully loaded function config", "functionConfig", commandeer.functionConfig)
			}

			// Populate initial defaults in the function spec, but consider existing values
			// if the spec was brought from a file or from an already imported function.
			commandeer.populateDeploymentDefaults()

			// Populate HTTP Service type
			commandeer.populateHTTPServiceType()

			// Override basic fields from the config
			commandeer.functionConfig.Meta.Name = commandeer.functionName
			commandeer.functionConfig.Meta.Namespace = rootCommandeer.namespace

			commandeer.functionConfig.Spec.Build = commandeer.functionBuild
			commandeer.functionConfig.Spec.Build.Commands = commandeer.commands
			commandeer.functionConfig.Spec.Build.FunctionConfigPath = commandeer.functionConfigPath

			// Enrich function config with args
			commandeer.enrichConfigWithStringArgs()
			commandeer.enrichConfigWithIntArgs()
			commandeer.enrichConfigWithBoolArgs()
			err = commandeer.enrichConfigWithComplexArgs()
			if err != nil {
				return errors.Wrap(err, "Failed config with complex args")
			}

			// Ensure the skip-annotations never exist on deploy
			commandeer.functionConfig.Meta.RemoveSkipBuildAnnotation()
			commandeer.functionConfig.Meta.RemoveSkipDeployAnnotation()

			commandeer.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Deploying function", "functionConfig", commandeer.functionConfig)
			_, deployErr := rootCommandeer.platform.CreateFunction(ctx, &platform.CreateFunctionOptions{
				Logger:         rootCommandeer.loggerInstance,
				FunctionConfig: commandeer.functionConfig,
				InputImageFile: commandeer.inputImageFile,
			})

			// don't check deploy error yet, first try to save the logs either way, and then return the error if necessary
			commandeer.rootCommandeer.loggerInstance.Debug("Saving deployment logs")
			logSaveErr := rootCommandeer.platform.SaveFunctionDeployLogs(ctx, commandeer.functionName, rootCommandeer.namespace)

			if deployErr != nil {

				// preserve the error and let the root commandeer handle unwrapping it
				return deployErr
			}
			return logSaveErr
		},
	}

	addDeployFlags(cmd, commandeer)
	cmd.Flags().StringVarP(&commandeer.inputImageFile, "input-image-file", "", "", "Path to an input function-image Docker archive file")

	commandeer.cmd = cmd

	return commandeer
}

func addDeployFlags(cmd *cobra.Command,
	commandeer *deployCommandeer) {
	addBuildFlags(cmd, &commandeer.functionBuild, &commandeer.functionConfigPath, &commandeer.runtime, &commandeer.handler, &commandeer.commands, &commandeer.encodedBuildRuntimeAttributes, &commandeer.encodedBuildCodeEntryAttributes)

	cmd.Flags().StringVar(&commandeer.description, "desc", "", "Function description")
	cmd.Flags().StringVarP(&commandeer.encodedLabels, "labels", "l", "", "Additional function labels (lbl1=val1[,lbl2=val2,...])")
	cmd.Flags().StringVar(&commandeer.encodedAnnotations, "annotations", "", "Additional function annotations (ant1=val1[,ant2=val2,...])")
	cmd.Flags().StringVar(&commandeer.encodedNodeSelector, "nodeSelector", "", "Run function pod on a Node by key=value selection constraints (key1=val1[,key2=val2,...])")
	cmd.Flags().StringVar(&commandeer.nodeName, "nodeName", "", "Run function pod on a Node by name-matching selection constrain")
	cmd.Flags().StringVar(&commandeer.priorityClassName, "priorityClassName", "", "Indicates the importance of a function Pod relatively to other function pods")
	cmd.Flags().StringVar(&commandeer.preemptionPolicy, "preemptionPolicy", "", "Function pod preemption policy")
	cmd.Flags().VarP(&commandeer.encodedEnv, "env", "e", "Environment variables env1=val1")
	cmd.Flags().BoolVarP(&commandeer.disable, "disable", "d", false, "Start the function as disabled (don't run yet)")
	cmd.Flags().IntVarP(&commandeer.replicas, "replicas", "", -1, "Set to any non-negative integer to use a static number of replicas")
	cmd.Flags().IntVar(&commandeer.minReplicas, "min-replicas", -1, "Minimal number of function replicas")
	cmd.Flags().IntVar(&commandeer.maxReplicas, "max-replicas", -1, "Maximal number of function replicas")
	cmd.Flags().Int64Var(&commandeer.runAsUser, "run-as-user", -1, "Run function process with user ID")
	cmd.Flags().Int64Var(&commandeer.runAsGroup, "run-as-group", -1, "Run function process with group ID")
	cmd.Flags().Int64Var(&commandeer.fsGroup, "fsgroup", -1, "Run function process with supplementary groups")
	cmd.Flags().IntVar(&commandeer.targetCPU, "target-cpu", -1, "Target CPU-usage percentage when auto-scaling")
	cmd.Flags().BoolVar(&commandeer.publish, "publish", false, "Publish the function")
	cmd.Flags().StringVar(&commandeer.encodedDataBindings, "data-bindings", "", "JSON-encoded data bindings for the function")
	cmd.Flags().StringVar(&commandeer.encodedTriggers, "triggers", "", "JSON-encoded triggers for the function")
	cmd.Flags().StringVar(&commandeer.overrideHTTPTriggerServiceType, "http-trigger-service-type", "", "A Kubernetes ServiceType to apply to the HTTP trigger")
	cmd.Flags().StringVar(&commandeer.encodedFunctionPlatformConfig, "platform-config", "", "JSON-encoded platform specific configuration")
	cmd.Flags().StringVar(&commandeer.image, "run-image", "", "Name of an existing image to deploy (default - build a new image to deploy)")
	cmd.Flags().StringVar(&commandeer.runRegistry, "run-registry", "", "URL of a registry for pulling the image, if differs from -r/--registry (env: NUCTL_RUN_REGISTRY)")
	cmd.Flags().StringVar(&commandeer.encodedRuntimeAttributes, "runtime-attrs", "", "JSON-encoded runtime attributes for the function")
	cmd.Flags().IntVar(&commandeer.readinessTimeoutSeconds, "readiness-timeout", -1, "Maximum wait period for the function to be ready, in seconds")
	cmd.Flags().StringVar(&commandeer.projectName, "project-name", "", "The name of the function's parent project")
	cmd.Flags().Var(&commandeer.volumes, "volume", "Volumes for the deployment function (src1=dest1[,src2=dest2,...])")
	cmd.Flags().Var(&commandeer.resourceLimits, "resource-limit", "Resource restrictions of the format '<resource name>=<quantity>' (for example, 'cpu=3')")
	cmd.Flags().Var(&commandeer.resourceRequests, "resource-request", "Requested resources of the format '<resource name>=<quantity>' (for example, 'cpu=3')")
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

// If user runs deploy with a function name of a function that was already imported, this checks if that function
// exists and is imported. If so, returns that function, otherwise returns nil.
func (d *deployCommandeer) getImportedFunction(ctx context.Context, functionName string) (platform.Function, error) {
	functions, err := d.rootCommandeer.platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:      functionName,
		Namespace: d.rootCommandeer.namespace,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to check existing functions")
	}

	if len(functions) == 0 {
		return nil, nil
	}

	function := functions[0]
	if err := function.Initialize(ctx, nil); err != nil {

		// debug level, we don't want to spam user output when we just try to import an non-existent function
		d.rootCommandeer.loggerInstance.DebugWith("Failed to initialize function", "err", err.Error())
	}

	if function.GetStatus().State == functionconfig.FunctionStateImported {
		return function, nil
	}

	return nil, nil
}

func (d *deployCommandeer) prepareFunctionConfigForRedeploy(importedFunction platform.Function) functionconfig.Config {
	functionConfig := importedFunction.GetConfig()

	// Ensure RunRegistry is taken from the commandeer config
	functionConfig.CleanFunctionSpec()
	functionConfig.Spec.RunRegistry = d.functionConfig.Spec.RunRegistry

	return *functionConfig
}

func (d *deployCommandeer) populateDeploymentDefaults() {
	if d.functionConfig.Spec.TargetCPU == 0 {
		d.functionConfig.Spec.TargetCPU = abstract.DefaultTargetCPU
	}
	if d.functionConfig.Spec.RunRegistry == "" {
		d.functionConfig.Spec.RunRegistry = os.Getenv("NUCTL_RUN_REGISTRY")
	}

	if d.functionConfig.Spec.ReadinessTimeoutSeconds == 0 {
		d.functionConfig.Spec.ReadinessTimeoutSeconds = int(
			d.rootCommandeer.platformConfiguration.GetDefaultFunctionReadinessTimeout().Seconds())
	}

	if d.functionConfig.Spec.DataBindings == nil {
		d.functionConfig.Spec.DataBindings = map[string]functionconfig.DataBinding{}
	}
	if d.functionConfig.Spec.Triggers == nil {
		d.functionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	}
	if d.functionConfig.Spec.RuntimeAttributes == nil {
		d.functionConfig.Spec.RuntimeAttributes = map[string]interface{}{}
	}
}

func (d *deployCommandeer) populateHTTPServiceType() {
	overridingHTTPServiceType := v1.ServiceType(common.GetEnvOrDefaultString("NUCTL_DEFAULT_SERVICE_TYPE", ""))
	if d.overrideHTTPTriggerServiceType != "" {
		overridingHTTPServiceType = v1.ServiceType(d.overrideHTTPTriggerServiceType)
	}

	if overridingHTTPServiceType != "" {
		d.rootCommandeer.platformConfiguration.Kube.DefaultServiceType = overridingHTTPServiceType
	}
}

func (d *deployCommandeer) enrichConfigWithStringArgs() {
	if d.description != "" {
		d.functionConfig.Spec.Description = d.description
	}

	if d.image != "" {
		d.functionConfig.Spec.Image = d.image
	}

	if d.runRegistry != "" {
		d.functionConfig.Spec.RunRegistry = d.runRegistry
	}

	if d.runtime != "" {
		d.functionConfig.Spec.Runtime = d.runtime
	}

	if d.handler != "" {
		d.functionConfig.Spec.Handler = d.handler
	}

	// check if logger level is set
	if d.loggerLevel != "" {
		d.functionConfig.Spec.LoggerSinks = []functionconfig.LoggerSink{
			{Level: d.loggerLevel},
		}
	}

	if d.nodeName != "" {
		d.functionConfig.Spec.NodeName = d.nodeName
	}

	if d.preemptionPolicy != "" {
		preemptionPolicy := v1.PreemptionPolicy(d.preemptionPolicy)
		d.functionConfig.Spec.PreemptionPolicy = &preemptionPolicy
	}

	if d.priorityClassName != "" {
		d.functionConfig.Spec.PriorityClassName = d.priorityClassName
	}
}

func (d *deployCommandeer) enrichConfigWithBoolArgs() {
	if d.disable {
		d.functionConfig.Spec.Disable = d.disable
	}

	if d.publish {
		d.functionConfig.Spec.Publish = d.publish
	}
}

func (d *deployCommandeer) enrichConfigWithIntArgs() {

	// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.Replicas as nil)
	if d.replicas >= 0 {
		d.functionConfig.Spec.Replicas = &d.replicas
	}

	// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.MinReplicas as nil)
	if d.minReplicas >= 0 {
		d.functionConfig.Spec.MinReplicas = &d.minReplicas
	}

	// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.MaxReplicas as nil)
	if d.maxReplicas >= 0 {
		d.functionConfig.Spec.MaxReplicas = &d.maxReplicas
	}

	// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.TargetCPU as default)
	if d.targetCPU >= 0 {
		d.functionConfig.Spec.TargetCPU = d.targetCPU
	}

	// any negative value counted as not set (meaning leaving commandeer.functionConfig.Spec.ReadinessTimeoutSeconds as default)
	if d.readinessTimeoutSeconds >= 0 {
		d.functionConfig.Spec.ReadinessTimeoutSeconds = d.readinessTimeoutSeconds
	}

	// fill security context

	// initialize struct if at least one flag is provided
	if common.AnyPositiveInSliceInt64([]int64{d.runAsUser, d.runAsGroup, d.fsGroup}) {
		d.functionConfig.Spec.SecurityContext = &v1.PodSecurityContext{}
	}

	// user id
	if d.runAsUser >= 0 {
		d.functionConfig.Spec.SecurityContext.RunAsUser = &d.runAsUser
	}

	// group id
	if d.runAsGroup >= 0 {
		d.functionConfig.Spec.SecurityContext.RunAsGroup = &d.runAsGroup
	}

	// groups
	if d.fsGroup >= 0 {
		d.functionConfig.Spec.SecurityContext.FSGroup = &d.fsGroup
	}

	// eof security context
}

func (d *deployCommandeer) enrichConfigWithComplexArgs() error {
	// parse volumes
	volumes, err := parseVolumes(d.volumes)
	if err != nil {
		return errors.Wrap(err, "Failed to parse volumes")
	}
	d.functionConfig.Spec.Volumes = append(d.functionConfig.Spec.Volumes, volumes...)

	// parse resource limits
	if err := parseResourceAllocations(d.resourceLimits,
		&d.functionConfig.Spec.Resources.Limits); err != nil {
		return errors.Wrap(err, "Failed to parse resource limits")
	}

	// parse resource requests
	if err := parseResourceAllocations(d.resourceRequests,
		&d.functionConfig.Spec.Resources.Requests); err != nil {
		return errors.Wrap(err, "Failed to parse resource requests")
	}

	// decode the JSON data bindings
	if d.encodedDataBindings != "" {
		if err := json.Unmarshal([]byte(d.encodedDataBindings),
			&d.functionConfig.Spec.DataBindings); err != nil {
			return errors.Wrap(err, "Failed to decode data bindings")
		}
	}

	// decode the JSON triggers
	if d.encodedTriggers != "" {
		if err := json.Unmarshal([]byte(d.encodedTriggers),
			&d.functionConfig.Spec.Triggers); err != nil {
			return errors.Wrap(err, "Failed to decode triggers")
		}
	}

	// decode the JSON function platform configuration
	if d.encodedFunctionPlatformConfig != "" {
		if err := json.Unmarshal([]byte(d.encodedFunctionPlatformConfig),
			&d.functionConfig.Spec.Platform); err != nil {
			return errors.Wrap(err, "Failed to decode function platform configuration")
		}
	}

	// decode the JSON runtime attributes
	if d.encodedRuntimeAttributes != "" {
		if err := json.Unmarshal([]byte(d.encodedRuntimeAttributes),
			&d.functionConfig.Spec.RuntimeAttributes); err != nil {
			return errors.Wrap(err, "Failed to decode runtime attributes")
		}
	}

	// decode the JSON build runtime attributes
	if d.encodedBuildRuntimeAttributes != "" {
		if err := json.Unmarshal([]byte(d.encodedBuildRuntimeAttributes),
			&d.functionConfig.Spec.Build.RuntimeAttributes); err != nil {
			return errors.Wrap(err, "Failed to decode build runtime attributes")
		}
	}

	// decode the JSON build code entry attributes
	if d.encodedBuildCodeEntryAttributes != "" {
		if err := json.Unmarshal([]byte(d.encodedBuildCodeEntryAttributes),
			&d.functionConfig.Spec.Build.CodeEntryAttributes); err != nil {
			return errors.Wrap(err, "Failed to decode code entry attributes")
		}
	}

	// decode annotations
	if d.functionConfig.Meta.Annotations == nil {
		d.functionConfig.Meta.Annotations = map[string]string{}
	}
	for annotation, annotationValue := range common.StringToStringMap(d.encodedAnnotations, "=") {
		d.functionConfig.Meta.Annotations[annotation] = annotationValue
	}

	// decode labels
	if d.functionConfig.Meta.Labels == nil {
		d.functionConfig.Meta.Labels = map[string]string{}
	}
	for label, labelValue := range common.StringToStringMap(d.encodedLabels, "=") {
		d.functionConfig.Meta.Labels[label] = labelValue
	}

	// decode node selector
	if d.encodedNodeSelector != "" {
		if d.functionConfig.Spec.NodeSelector == nil {
			d.functionConfig.Spec.NodeSelector = map[string]string{}
		}
		for key, value := range common.StringToStringMap(d.encodedNodeSelector, "=") {
			d.functionConfig.Spec.NodeSelector[key] = value
		}
	}

	// if the project name was set, add it as a label (not in string enrichment, because it's part of the labels)
	if d.projectName != "" {
		d.functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] = d.projectName
	}

	// decode env
	for _, encodedEnvNameAndValue := range d.encodedEnv {
		envNameAndValue := strings.SplitN(encodedEnvNameAndValue, "=", 2)
		if len(envNameAndValue) != 2 {
			return errors.Errorf("Environment variable must be in the form of name=value: %s",
				encodedEnvNameAndValue)
		}

		d.functionConfig.Spec.Env = append(d.functionConfig.Spec.Env, v1.EnvVar{
			Name:  envNameAndValue[0],
			Value: envNameAndValue[1],
		})
	}

	return nil
}
