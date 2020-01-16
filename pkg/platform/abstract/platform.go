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

package abstract

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/logger"
)

const (
	triggerMaxWorkersLimit = 1000
)

//
// Base for all platforms
//

type Platform struct {
	Logger                         logger.Logger
	platform                       platform.Platform
	invoker                        *invoker
	Config                         interface{}
	ExternalIPAddresses            []string
	DeployLogStreams               map[string]*LogStream
	ContainerBuilder               containerimagebuilderpusher.BuilderPusher
	DefaultHTTPIngressHostTemplate string
}

func NewPlatform(parentLogger logger.Logger, platform platform.Platform, platformConfiguration interface{}) (*Platform, error) {
	var err error

	newPlatform := &Platform{
		Logger:           parentLogger.GetChild("platform"),
		platform:         platform,
		Config:           platformConfiguration,
		DeployLogStreams: map[string]*LogStream{},
	}

	// create invoker
	newPlatform.invoker, err = newInvoker(newPlatform.Logger, platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create invoker")
	}

	return newPlatform, nil
}

func (ap *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {

	// execute a build
	builder, err := build.NewBuilder(createFunctionBuildOptions.Logger, ap.platform, &common.AbstractS3Client{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	return builder.Build(createFunctionBuildOptions)
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *Platform) HandleDeployFunction(existingFunctionConfig *functionconfig.ConfigWithStatus,
	createFunctionOptions *platform.CreateFunctionOptions,
	onAfterConfigUpdated func(*functionconfig.Config) error,
	onAfterBuild func(*platform.CreateFunctionBuildResult, error) (*platform.CreateFunctionResult, error)) (*platform.CreateFunctionResult, error) {

	createFunctionOptions.Logger.InfoWith("Deploying function",
		"name", createFunctionOptions.FunctionConfig.Meta.Name)

	var buildResult *platform.CreateFunctionBuildResult
	var buildErr error

	// when the config is updated, save to deploy options and call underlying hook
	onAfterConfigUpdatedWrapper := func(updatedFunctionConfig *functionconfig.Config) error {
		createFunctionOptions.FunctionConfig = *updatedFunctionConfig

		return onAfterConfigUpdated(updatedFunctionConfig)
	}

	functionBuildRequired, err := ap.functionBuildRequired(createFunctionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed determining whether function should build")
	}

	// special case when we are asked to build the function and it wasn't been created yet
	if existingFunctionConfig == nil &&
		createFunctionOptions.FunctionConfig.Spec.Build.Mode == functionconfig.NeverBuild {
		return nil, errors.New("Non existing function cannot be created with neverBuild mode")
	}

	// clear build mode
	createFunctionOptions.FunctionConfig.Spec.Build.Mode = ""

	// check if we need to build the image
	if functionBuildRequired {
		buildResult, buildErr = ap.platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
			Logger:                     createFunctionOptions.Logger,
			FunctionConfig:             createFunctionOptions.FunctionConfig,
			PlatformName:               ap.platform.GetName(),
			OnAfterConfigUpdate:        onAfterConfigUpdatedWrapper,
			DependantImagesRegistryURL: createFunctionOptions.DependantImagesRegistryURL,
		})

		if buildErr == nil {

			// use the function configuration augmented by the builder
			createFunctionOptions.FunctionConfig.Spec.Image = buildResult.Image

			// if run registry isn't set, set it to that of the build
			if createFunctionOptions.FunctionConfig.Spec.RunRegistry == "" {
				createFunctionOptions.FunctionConfig.Spec.RunRegistry = createFunctionOptions.FunctionConfig.Spec.Build.Registry
			}

			// on successful build set the timestamp of build
			createFunctionOptions.FunctionConfig.Spec.Build.Timestamp = time.Now().Unix()
		}
	} else {
		createFunctionOptions.Logger.InfoWith("Skipping build",
			"name", createFunctionOptions.FunctionConfig.Meta.Name)

		// verify user passed runtime
		if createFunctionOptions.FunctionConfig.Spec.Runtime == "" {
			return nil, errors.New("If image is passed, runtime must be specified")
		}

		// populate image if possible
		if existingFunctionConfig != nil && createFunctionOptions.FunctionConfig.Spec.Image == "" {
			createFunctionOptions.FunctionConfig.Spec.Image = existingFunctionConfig.Spec.Image
		}

		// trigger the on after config update ourselves
		if err = onAfterConfigUpdatedWrapper(&createFunctionOptions.FunctionConfig); err != nil {
			return nil, errors.Wrap(err, "Failed to trigger on after config update")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction
	deployResult, err := onAfterBuild(buildResult, buildErr)
	if buildErr != nil || err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// sanity
	if deployResult == nil {
		return nil, errors.New("Deployer returned no error, but nil deploy result")
	}

	// if we got a deploy result and build result, set them
	if buildResult != nil {
		deployResult.CreateFunctionBuildResult = *buildResult
	}

	// indicate that we're done
	createFunctionOptions.Logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, nil
}

// Validation and enforcement of required function creation logic
func (ap *Platform) ValidateCreateFunctionOptions(createFunctionOptions *platform.CreateFunctionOptions) error {

	// if labels is nil assign an empty map to it
	if createFunctionOptions.FunctionConfig.Meta.Labels == nil {
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{}
	}

	// if no project name was given, set it to the default project
	if createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"] == "" {
		createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"] = "default"
		ap.Logger.Debug("No project name specified. Setting to default")
	}

	// validate the project exists
	getProjectsOptions := &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"],
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		},
	}
	projects, err := ap.platform.GetProjects(getProjectsOptions)
	if err != nil {
		return errors.Wrap(err, "Failed getting projects")
	}

	if len(projects) == 0 {
		return errors.New("Project doesn't exist")
	}

	// Verify trigger's MaxWorkers value is making sense
	for triggerName, trigger := range createFunctionOptions.FunctionConfig.Spec.Triggers {
		if trigger.MaxWorkers > triggerMaxWorkersLimit {
			return errors.Errorf("MaxWorkers value for %s trigger (%d) exceeds the limit of %d",
				triggerName,
				trigger.MaxWorkers,
				triggerMaxWorkersLimit)
		}
	}

	return nil
}

// Validation and enforcement of required function deletion logic
func (ap *Platform) ValidateDeleteProjectOptions(deleteProjectOptions *platform.DeleteProjectOptions) error {
	projectName := deleteProjectOptions.Meta.Name

	if projectName == "default" {
		return errors.New("Cannot delete the default project")
	}

	getFunctionsOptions := &platform.GetFunctionsOptions{
		Namespace: deleteProjectOptions.Meta.Namespace,
		Labels:    fmt.Sprintf("nuclio.io/project-name=%s", projectName),
	}

	functions, err := ap.platform.GetFunctions(getFunctionsOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) != 0 {
		return platform.ErrProjectContainsFunctions
	}

	return nil
}

// CreateFunctionInvocation will invoke a previously deployed function
func (ap *Platform) CreateFunctionInvocation(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	return ap.invoker.invoke(createFunctionInvocationOptions)
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (ap *Platform) GetHealthCheckMode() platform.HealthCheckMode {

	// by default return that some external entity does health checks for us
	return platform.HealthCheckModeExternal
}

// CreateProject will probably create a new project
func (ap *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	return errors.New("Unsupported")
}

// UpdateProject will update a previously existing project
func (ap *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	return errors.New("Unsupported")
}

// DeleteProject will delete a previously existing project
func (ap *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	return errors.New("Unsupported")
}

// GetProjects will list existing projects
func (ap *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return nil, errors.New("Unsupported")
}

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (ap *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	return errors.New("Unsupported")
}

// UpdateFunctionEvent will update a previously existing function event
func (ap *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	return errors.New("Unsupported")
}

// DeleteFunctionEvent will delete a previously existing function event
func (ap *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	return errors.New("Unsupported")
}

// GetFunctionEvents will list existing function events
func (ap *Platform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	return nil, errors.New("Unsupported")
}

// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
// If this is not invoked, each platform will try to discover these addresses automatically
func (ap *Platform) SetExternalIPAddresses(externalIPAddresses []string) error {
	ap.ExternalIPAddresses = externalIPAddresses

	return nil
}

func (ap *Platform) SetDefaultHTTPIngressHostTemplate(defaultHTTPIngressHostTemplate string) {
	ap.DefaultHTTPIngressHostTemplate = defaultHTTPIngressHostTemplate
}

func (ap *Platform) GetDefaultHTTPIngressHostTemplate() string {
	return ap.DefaultHTTPIngressHostTemplate
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (ap *Platform) GetExternalIPAddresses() ([]string, error) {
	return ap.ExternalIPAddresses, nil
}

func (ap *Platform) GetScaleToZeroConfiguration() (*platformconfig.ScaleToZero, error) {
	return nil, nil
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (ap *Platform) ResolveDefaultNamespace(defaultNamespace string) string {
	return ""
}

// BuildAndPushContainerImage builds container image and pushes it into docker registry
func (ap *Platform) BuildAndPushContainerImage(buildOptions *containerimagebuilderpusher.BuildOptions) error {
	return ap.ContainerBuilder.BuildAndPushContainerImage(buildOptions, ap.platform.ResolveDefaultNamespace("@nuclio.selfNamespace"))
}

// Get Onbuild stage for multistage builds
func (ap *Platform) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	return ap.ContainerBuilder.GetOnbuildStages(onbuildArtifacts)
}

// Change Onbuild artifact paths depending on the type of the builder used
func (ap *Platform) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	return ap.ContainerBuilder.TransformOnbuildArtifactPaths(onbuildArtifacts)
}

// GetBaseImageRegistry returns onbuild base registry
func (ap *Platform) GetBaseImageRegistry(registry string) string {
	return ap.ContainerBuilder.GetBaseImageRegistry(registry)
}

func (ap *Platform) functionBuildRequired(createFunctionOptions *platform.CreateFunctionOptions) (bool, error) {

	// if neverBuild was passed explicitly don't build
	if createFunctionOptions.FunctionConfig.Spec.Build.Mode == functionconfig.NeverBuild {
		return false, nil
	}

	// if the function contains source code, an image name or a path somewhere - we need to rebuild. the shell
	// runtime supports a case where user just tells image name and we build around the handler without a need
	// for a path
	if createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode != "" ||
		createFunctionOptions.FunctionConfig.Spec.Build.Path != "" ||
		createFunctionOptions.FunctionConfig.Spec.Build.Image != "" {
		return true, nil
	}

	if createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryType == build.S3EntryType {
		return true, nil
	}

	// if user didn't give any of the above but _did_ specify an image to run from, just dont build
	if createFunctionOptions.FunctionConfig.Spec.Image != "" {
		return false, nil
	}

	// should not get here - we should either be able to build an image or have one specified for us
	return false, errors.New("Function must have either spec.build.path," +
		"spec.build.functionSourceCode, spec.build.image or spec.image set in order to create")
}

func (ap *Platform) GetProcessorLogsAndBriefError(scanner *bufio.Scanner) (string, string) {
	var formattedProcessorLogs, briefErrorMessage string

	for scanner.Scan() {
		currentLogLine, logLevelAboveInfo, err := ap.prettifyProcessorLogLine(scanner.Bytes())
		if err != nil {

			// when it is unstructured just add the log as a text
			formattedProcessorLogs += scanner.Text() + "\n"
			briefErrorMessage += scanner.Text() + "\n"
			continue
		}

		if logLevelAboveInfo {
			briefErrorMessage += currentLogLine + "\n"
		}

		// when it is a log line generated by the processor
		formattedProcessorLogs += currentLogLine + "\n"
	}

	return common.FixEscapeChars(formattedProcessorLogs), common.FixEscapeChars(briefErrorMessage)
}

// returns:
// string: prettified pod log line
// bool:   log level is above info
// error:  explaining what failed during parsing the log line
func (ap *Platform) prettifyProcessorLogLine(log []byte) (string, bool, error) {
	logStruct := struct {
		Time    *string `json:"time"`
		Level   *string `json:"level"`
		Message *string `json:"message"`
		More    *string `json:"more,omitempty"`
	}{}

	if len(log) > 0 && log[0] == 'l' {

		// when it is a wrapper log line
		wrapperLogStruct := struct {
			Datetime *string           `json:"datetime"`
			Level    *string           `json:"level"`
			Message  *string           `json:"message"`
			With     map[string]string `json:"with,omitempty"`
		}{}

		if err := json.Unmarshal(log[1:], &wrapperLogStruct); err != nil {
			return "", false, err
		}

		// manipulate the time format so it can be parsed later
		unparsedTime := *wrapperLogStruct.Datetime + "Z"
		unparsedTime = strings.Replace(unparsedTime, " ", "T", 1)
		unparsedTime = strings.Replace(unparsedTime, ",", ".", 1)

		logStruct.Time = &unparsedTime
		logStruct.Level = wrapperLogStruct.Level
		logStruct.Message = wrapperLogStruct.Message

		more := common.CreateKeyValuePairs(wrapperLogStruct.With)
		logStruct.More = &more

	} else {

		// when it is a log line generated by the processor
		if err := json.Unmarshal(log, &logStruct); err != nil {
			return "", false, err
		}
	}

	// check required fields existence
	if logStruct.Time == nil || logStruct.Level == nil || logStruct.Message == nil {
		return "", false, errors.New("Missing required fields in pod log line")
	}

	parsedTime, err := time.Parse(time.RFC3339, *logStruct.Time)
	if err != nil {
		return "", false, err
	}

	logLevel := strings.ToUpper(*logStruct.Level)[0]

	res := fmt.Sprintf("[%s] (%c) %s",
		parsedTime.Format("15:04:05.000"),
		logLevel,
		*logStruct.Message)

	if logStruct.More != nil {
		res = fmt.Sprintf("%s [%s]", res, *logStruct.More)
	}

	if logLevel != 'D' && logLevel != 'I' {
		return res, true, nil
	}

	return res, false, nil
}
