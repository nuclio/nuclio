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

package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
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

	// beta - api client
	betaCommandeer         *betaCommandeer
	noBuild                bool
	redeployWithReportFile bool
	deployAll              bool
	waitForFunction        bool
	skipSpecCleanup        bool
	verifyExternalRegistry bool
	outputManifest         *nuctlcommon.PatchManifest
	inputManifest          *nuctlcommon.PatchManifest
	noReport               bool
	reportFilePath         string
	excludedProjects       []string
	excludedFunctions      []string
	excludeFunctionWithGPU bool
	importedOnly           bool
	waitTimeout            time.Duration
}

func newDeployCommandeer(ctx context.Context, rootCommandeer *RootCommandeer, betaCommandeer *betaCommandeer) *deployCommandeer {
	commandeer := &deployCommandeer{
		rootCommandeer: rootCommandeer,
		functionConfig: *functionconfig.NewConfig(),
		outputManifest: nuctlcommon.NewPatchManifest(),
	}

	if betaCommandeer != nil {
		commandeer.betaCommandeer = betaCommandeer
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

			if commandeer.betaCommandeer != nil {
				if err := commandeer.betaCommandeer.initialize(); err != nil {
					return errors.Wrap(err, "Failed to initialize beta commandeer")
				}
				commandeer.rootCommandeer.loggerInstance.Debug("In BETA mode")
				if commandeer.noBuild {
					if err := commandeer.betaDeploy(ctx, args); err != nil {
						return errors.Wrap(err, "Failed to deploy function")
					}
					return nil
				}
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
					commandeer.functionConfig = commandeer.prepareFunctionConfigForRedeploy(importedFunction, commandeer.skipSpecCleanup)
				}
			}

			// If config file is provided
			if importedFunction == nil && commandeer.functionConfigPath != "" {
				commandeer.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Loading function config from file", "file", commandeer.functionConfigPath)
				functionConfigFile, err := nuctlcommon.OpenFile(commandeer.functionConfigPath)
				if err != nil {
					return errors.Wrap(err, "Failed opening function config file")
				}

				functionBody, err := io.ReadAll(functionConfigFile)
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

			// Enrich function config with args
			if err := commandeer.enrichConfigMetadata(rootCommandeer); err != nil {
				return errors.Wrap(err, "Failed overriding basic config fields")
			}
			commandeer.enrichConfigWithStringArgs()
			commandeer.enrichConfigWithIntArgs()
			commandeer.enrichConfigWithBoolArgs()
			commandeer.enrichBuildConfigWithArgs()
			if err = commandeer.enrichConfigWithComplexArgs(); err != nil {
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
	cmd.Flags().BoolVarP(&commandeer.skipSpecCleanup, "skip-spec-cleanup", "", false, "Do not clean up spec in function configs")
	cmd.Flags().BoolVarP(&commandeer.redeployWithReportFile, "redeploy-with-report-file", "", false, "Redeploy any functions that failed in the report and can be redeployed")
	cmd.Flags().BoolVarP(&commandeer.noReport, "no-report", "", false, "Do not save report to a file")
	cmd.Flags().BoolVarP(&commandeer.verifyExternalRegistry, "verify-external-registry", "", false, "verify registry is external")
	cmd.Flags().StringVarP(&commandeer.inputImageFile, "input-image-file", "", "", "Path to an input function-image Docker archive file")
	cmd.Flags().StringVarP(&commandeer.reportFilePath, "output-file", "", "nuctl-deployment-report.json", "Path to deployment report")

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

	addBetaDeployFlags(cmd, commandeer)
}

func addBetaDeployFlags(cmd *cobra.Command,
	commandeer *deployCommandeer) {

	cmd.Flags().BoolVar(&commandeer.noBuild, "no-build", false, "Don't build the function, only deploy it")
	cmd.Flags().BoolVar(&commandeer.deployAll, "deploy-all", false, "Deploy all functions in the namespace")
	cmd.PersistentFlags().BoolVarP(&commandeer.waitForFunction, "wait", "w", false, "Wait for function deployment to complete")
	cmd.PersistentFlags().StringSliceVar(&commandeer.excludedProjects, "exclude-projects", []string{}, "Exclude projects to patch")
	cmd.PersistentFlags().StringSliceVar(&commandeer.excludedFunctions, "exclude-functions", []string{}, "Exclude functions to patch")
	cmd.PersistentFlags().BoolVar(&commandeer.excludeFunctionWithGPU, "exclude-functions-with-gpu", false, "Skip functions with GPU")
	cmd.PersistentFlags().BoolVar(&commandeer.importedOnly, "imported-only", false, "Deploy only imported functions")
	cmd.PersistentFlags().DurationVar(&commandeer.waitTimeout, "wait-timeout", 15*time.Minute, "Wait timeout duration for the function deployment (default 15m)")
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

func (d *deployCommandeer) prepareFunctionConfigForRedeploy(importedFunction platform.Function, skipSpecCleanup bool) functionconfig.Config {
	functionConfig := importedFunction.GetConfig()

	// Ensure RunRegistry is taken from the commandeer config
	if !skipSpecCleanup {
		functionConfig.CleanFunctionSpec()
	}
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

	d.functionConfig.Spec.ReadinessTimeoutSeconds =
		d.rootCommandeer.platformConfiguration.GetFunctionReadinessTimeoutOrDefault(d.functionConfig.Spec.ReadinessTimeoutSeconds)

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

// enrichConfigMetadata overrides metadata fields in the function config with the values from the commandeer,
// if they are given explicitly
func (d *deployCommandeer) enrichConfigMetadata(rootCommandeer *RootCommandeer) error {

	functionName, err := d.resolveFunctionName()
	if err != nil {
		return errors.Wrap(err, "Failed to resolve function name")
	}

	// set the resolved function name
	d.functionConfig.Meta.Name = functionName
	d.functionName = functionName

	// override the namespace in the config with the namespace from the command line (must be set)
	d.functionConfig.Meta.Namespace = rootCommandeer.namespace

	return nil
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

func (d *deployCommandeer) enrichBuildConfigWithArgs() {

	// enrich string fields in function config with flags
	common.PopulateFieldsFromValues(map[*string]string{
		&d.functionConfig.Spec.Build.FunctionConfigPath: d.functionConfigPath,
		&d.functionConfig.Spec.Build.Path:               d.functionBuild.Path,
		&d.functionConfig.Spec.Build.FunctionSourceCode: d.functionBuild.FunctionSourceCode,
		&d.functionConfig.Spec.Build.Image:              d.functionBuild.Image,
		&d.functionConfig.Spec.Build.Registry:           d.functionBuild.Registry,
		&d.functionConfig.Spec.Build.BaseImage:          d.functionBuild.BaseImage,
		&d.functionConfig.Spec.Build.OnbuildImage:       d.functionBuild.OnbuildImage,
		&d.functionConfig.Spec.Build.CodeEntryType:      d.functionBuild.CodeEntryType,
	})

	// enrich bool fields in function config with flags
	common.PopulateFieldsFromValues(map[*bool]bool{
		&d.functionConfig.Spec.Build.NoBaseImagesPull: d.functionBuild.NoBaseImagesPull,
		&d.functionConfig.Spec.Build.NoCleanup:        d.functionBuild.NoCleanup,
		&d.functionConfig.Spec.Build.Offline:          d.functionBuild.Offline,
	})

	// enrich build commands
	if len(d.commands) > 0 {
		d.functionConfig.Spec.Build.Commands = d.commands
	}
}

func (d *deployCommandeer) betaDeploy(ctx context.Context, args []string) error {

	if d.redeployWithReportFile {
		d.inputManifest = nuctlcommon.NewPatchManifestFromFile(d.reportFilePath)
		retryableFunctions := d.inputManifest.GetRetryable()
		d.rootCommandeer.loggerInstance.InfoWith("Redeploying failed functions from report file", "report file", d.reportFilePath,
			"functions", retryableFunctions)
		args = append(args, retryableFunctions...)
	}

	if len(args) == 0 && !d.redeployWithReportFile {
		// redeploy all functions in the namespace
		if err := d.redeployAllFunctions(ctx); err != nil {
			return errors.Wrap(err, "Failed to redeploy all functions")
		}
	} else {

		// redeploy the given functions
		if err := d.redeployFunctions(ctx, args); err != nil {
			return errors.Wrap(err, "Failed to redeploy functions")
		}
	}

	return nil
}

func (d *deployCommandeer) redeployAllFunctions(ctx context.Context) error {
	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Redeploying all functions")

	// get function names to redeploy
	functionNames, err := d.getFunctionNames(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get functions")
	}

	return d.redeployFunctions(ctx, functionNames)
}

func (d *deployCommandeer) getFunctionNames(ctx context.Context) ([]string, error) {
	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Getting function names")

	functionConfigs, err := d.betaCommandeer.apiClient.GetFunctions(ctx, d.rootCommandeer.namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functionNames := make([]string, 0)
	for functionName, functionConfigWithStatus := range functionConfigs {

		// filter excluded functions
		if d.shouldSkipFunction(functionConfigWithStatus.Config) {
			d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Excluding function", "function", functionName)
			d.outputManifest.AddSkipped(functionName)
			continue
		}
		functionNames = append(functionNames, functionName)
	}

	return functionNames, nil
}

func (d *deployCommandeer) redeployFunctions(ctx context.Context, functionNames []string) error {
	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Redeploying functions", "functionNames", functionNames)

	patchErrGroup, _ := errgroup.WithContextSemaphore(ctx, d.rootCommandeer.loggerInstance, uint(d.betaCommandeer.concurrency))
	for _, function := range functionNames {
		function := function
		patchErrGroup.Go("patch function", func() error {
			if err := d.patchFunction(ctx, function); err != nil {
				d.rootCommandeer.loggerInstance.ErrorWith("Problem in redeployment", "err", err)
				d.outputManifest.AddFailure(function, err, d.resolveIfRedeployRetryable(err))
				return errors.Wrap(err, "Failed to patch function")
			}
			d.outputManifest.AddSuccess(function)
			return nil
		})
	}

	if err := patchErrGroup.Wait(); err != nil {

		// Functions that failed to patch are included in the output manifest,
		// so we don't need to fail the entire operation here
		d.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Failed to patch functions", "err", err)
	}

	d.outputManifest.LogOutput(ctx, d.rootCommandeer.loggerInstance)
	d.outputManifest.SaveToFile(ctx, d.rootCommandeer.loggerInstance, d.reportFilePath)

	return nil
}

// patchFunction patches a single function
func (d *deployCommandeer) patchFunction(ctx context.Context, functionName string) error {

	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Redeploying function", "function", functionName)

	// patch function
	patchOptions := map[string]string{
		"desiredState": "ready",
	}

	payload, err := json.Marshal(patchOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal payload")
	}

	requestHeaders := d.resolveRequestHeaders()

	if err := d.betaCommandeer.apiClient.PatchFunction(ctx,
		functionName,
		d.rootCommandeer.namespace,
		payload,
		requestHeaders); err != nil {
		switch typedError := err.(type) {
		case *nuclio.ErrorWithStatusCode:
			return nuclio.GetWrapByStatusCode(typedError.StatusCode())(errors.Wrap(err, "Failed to patch function"))
		default:
			return errors.Wrap(typedError, "Failed to patch function")
		}
	}

	d.rootCommandeer.loggerInstance.InfoWithCtx(ctx,
		"Function redeploy request sent successfully",
		"function", functionName)

	if d.waitForFunction {
		return d.waitForFunctionDeployment(ctx, functionName)
	}

	return nil
}

func (d *deployCommandeer) waitForFunctionDeployment(ctx context.Context, functionName string) error {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-time.After(d.waitTimeout):
			return errors.New(fmt.Sprintf("Timed out waiting for function '%s' to be ready", functionName))
		case <-ticker.C:
			isTerminal, err := d.functionIsInTerminalState(ctx, functionName)
			if !isTerminal {

				// function isn't in terminal state yet, retry
				continue
			}
			if err != nil {

				// function is terminal, but not ready, return error
				return err
			}

			// function is ready
			return nil
		}
	}
}

// functionIsInTerminalState checks if the function is in terminal state
// if the function is ready, it returns true and no error
// if the function is in another terminal state, it returns true and an error
// else it returns false
func (d *deployCommandeer) functionIsInTerminalState(ctx context.Context, functionName string) (bool, error) {

	// get function and poll its status
	function, err := d.betaCommandeer.apiClient.GetFunction(ctx, functionName, d.rootCommandeer.namespace)
	if err != nil {
		d.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Failed to get function", "functionName", functionName)
		return false, err
	}
	if function.Status.State == functionconfig.FunctionStateReady {
		d.rootCommandeer.loggerInstance.InfoWithCtx(ctx,
			"Function redeployed successfully",
			"functionName", functionName)
		return true, nil
	}

	// we use this function to check if the function is in terminal state, as we already checked if it's ready
	if functionconfig.FunctionStateInSlice(function.Status.State,
		[]functionconfig.FunctionState{
			functionconfig.FunctionStateError,
			functionconfig.FunctionStateUnhealthy,
			functionconfig.FunctionStateScaledToZero,
		}) {
		return true, errors.New(fmt.Sprintf("Function '%s' is in terminal state '%s' but not ready",
			functionName, function.Status.State))
	}

	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
		"Function not ready yet",
		"functionName", functionName,
		"functionState", function.Status.State)

	return false, nil
}

// shouldSkipFunction returns true if the function patch should be skipped
func (d *deployCommandeer) shouldSkipFunction(functionConfig functionconfig.Config) bool {
	functionName := functionConfig.Meta.Name
	projectName := functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	// skip if function is excluded or if it has a positive GPU resource limit
	if common.StringSliceContainsString(d.excludedFunctions, functionName) ||
		common.StringSliceContainsString(d.excludedProjects, projectName) ||
		(d.excludeFunctionWithGPU && functionConfig.Spec.PositiveGPUResourceLimit()) {
		return true
	}

	return false
}

func (d *deployCommandeer) resolveRequestHeaders() map[string]string {
	requestHeaders := map[string]string{}
	if d.importedOnly {

		// add a header that will tell the API to only deploy imported functions
		requestHeaders[headers.ImportedFunctionOnly] = "true"
	}
	if d.verifyExternalRegistry {
		requestHeaders[headers.VerifyExternalRegistry] = "true"
	}
	return requestHeaders
}

func (d *deployCommandeer) resolveFunctionName() (string, error) {
	if d.functionName != "" {
		return d.functionName, nil
	}

	// if function name is not provided, use the name from the config
	if d.functionConfig.Meta.Name != "" {
		return d.functionConfig.Meta.Name, nil
	}

	// if a path is provided, read the function config from the path and use the name from the config
	if d.functionBuild.Path != "" {
		functionName, err := d.resolveFunctionNameFromPath()
		if err != nil {
			return "", errors.Wrap(err, "Failed to resolve function name from path")
		}
		return functionName, nil
	}

	return "", errors.New("Function name is not provided")
}

func (d *deployCommandeer) resolveIfRedeployRetryable(err error) bool {
	switch typedError := err.(type) {
	case *nuclio.ErrorWithStatusCode:
		// if the status code is 412, then another redeployment will not help because there is something wrong with the configuration
		return typedError.StatusCode() != http.StatusPreconditionFailed
	case error:
		return true
	}
	return true
}

func (d *deployCommandeer) resolveFunctionNameFromPath() (string, error) {

	functionConfigPath := d.resolveFunctionConfigPath()
	if functionConfigPath == "" {
		return "", errors.New("Failed to resolve function config path")
	}

	config := &functionconfig.Config{}

	functionconfigReader, err := functionconfig.NewReader(d.rootCommandeer.loggerInstance)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create functionconfig reader")
	}
	if err := functionconfigReader.ReadFunctionConfigFile(functionConfigPath, config); err != nil {
		return "", errors.Wrap(err, "Failed to read function configuration")
	}

	return config.Meta.Name, nil
}

func (d *deployCommandeer) resolveFunctionConfigPath() string {

	// if the user provided a configuration path, use that
	if d.functionBuild.FunctionConfigPath != "" {
		return d.functionBuild.FunctionConfigPath
	}

	// if the path is a file, check if it is a yaml file
	if common.IsFile(d.functionBuild.Path) {
		cleanPath := filepath.Clean(d.functionBuild.Path)
		if filepath.Ext(cleanPath) == ".yaml" || filepath.Ext(cleanPath) == ".yml" {
			return cleanPath
		}

		// it's a file, but not a config file
		return ""
	}

	functionConfigPath := filepath.Join(d.functionBuild.Path, common.FunctionConfigFileName)

	if !common.FileExists(functionConfigPath) {
		return ""
	}

	return functionConfigPath
}
