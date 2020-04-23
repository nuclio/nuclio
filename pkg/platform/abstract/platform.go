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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

//
// Base for all platforms
//

const (
	DefaultReadinessTimeoutSeconds = 60
	DefaultTargetCPU               = 75
)

type Platform struct {
	Logger                         logger.Logger
	platform                       platform.Platform
	invoker                        *invoker
	Config                         interface{}
	ExternalIPAddresses            []string
	DeployLogStreams               *sync.Map
	ContainerBuilder               containerimagebuilderpusher.BuilderPusher
	DefaultHTTPIngressHostTemplate string
	ImageNamePrefixTemplate        string
}

func NewPlatform(parentLogger logger.Logger, platform platform.Platform, platformConfiguration interface{}) (*Platform, error) {
	var err error

	newPlatform := &Platform{
		Logger:           parentLogger.GetChild("platform"),
		platform:         platform,
		Config:           platformConfiguration,
		DeployLogStreams: &sync.Map{},
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

	if createFunctionOptions.FunctionConfig.Spec.ImagePullSecrets == "" {
		createFunctionOptions.FunctionConfig.Spec.ImagePullSecrets = ap.platform.GetDefaultRegistryCredentialsSecretName()
	}

	// clear build mode
	createFunctionOptions.FunctionConfig.Spec.Build.Mode = ""

	// check if we need to build the image
	if functionBuildRequired && !functionconfig.ShouldSkipBuild(createFunctionOptions.FunctionConfig.Meta.Annotations) {
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

// Enrichment of create function options
func (ap *Platform) EnrichCreateFunctionOptions(createFunctionOptions *platform.CreateFunctionOptions) error {

	// if labels is nil assign an empty map to it
	if createFunctionOptions.FunctionConfig.Meta.Labels == nil {
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{}
	}

	if err := ap.enrichProjectName(createFunctionOptions); err != nil {
		return errors.Wrap(err, "Failed enriching project name")
	}

	if err := ap.enrichImageName(createFunctionOptions); err != nil {
		return errors.Wrap(err, "Failed enriching image name")
	}

	ap.enrichMinMaxReplicas(createFunctionOptions)

	return nil
}

// Enrich functions status with logs
func (ap *Platform) EnrichFunctionsWithDeployLogStream(functions []platform.Function) {

	// iterate over functions and enrich with deploy logs
	for _, function := range functions {
		if deployLogStream, exists := ap.DeployLogStreams.Load(function.GetConfig().Meta.GetUniqueID()); exists {
			deployLogStream.(*LogStream).ReadLogs(nil, &function.GetStatus().Logs)
		}
	}
}

// Validation and enforcement of required function creation logic
func (ap *Platform) ValidateCreateFunctionOptions(createFunctionOptions *platform.CreateFunctionOptions) error {

	if err := ap.validateTriggers(createFunctionOptions); err != nil {
		return errors.Wrap(err, "Triggers validation failed")
	}

	if err := ap.validateMinMaxReplicas(createFunctionOptions); err != nil {
		return errors.Wrap(err, "Min max replicas validation failed")
	}

	if err := ap.validateProjectExists(createFunctionOptions); err != nil {
		return errors.Wrap(err, "Project existence validation failed")
	}

	return nil
}

// Validation and enforcement of required function deletion logic
func (ap *Platform) ValidateDeleteProjectOptions(deleteProjectOptions *platform.DeleteProjectOptions) error {
	projectName := deleteProjectOptions.Meta.Name

	if projectName == platform.DefaultProjectName {
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

func (ap *Platform) SetImageNamePrefixTemplate(imageNamePrefixTemplate string) {
	ap.ImageNamePrefixTemplate = imageNamePrefixTemplate
}

func (ap *Platform) GetImageNamePrefixTemplate() string {
	return ap.ImageNamePrefixTemplate
}

func (ap *Platform) RenderImageNamePrefixTemplate(projectName string, functionName string) (string, error) {
	return common.RenderTemplate(ap.ImageNamePrefixTemplate, map[string]interface{}{
		"ProjectName":  projectName,
		"FunctionName": functionName,
	})
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

// GetBaseImageRegistry returns base image registry
func (ap *Platform) GetBaseImageRegistry(registry string) string {
	return ap.ContainerBuilder.GetBaseImageRegistry(registry)
}

// GetOnbuildImageRegistry returns onbuild image registry
func (ap *Platform) GetOnbuildImageRegistry(registry string) string {
	return ap.ContainerBuilder.GetOnbuildImageRegistry(registry)
}

// // GetDefaultRegistryCredentialsSecretName returns secret with credentials to push/pull from docker registry
func (ap *Platform) GetDefaultRegistryCredentialsSecretName() string {
	return ap.ContainerBuilder.GetDefaultRegistryCredentialsSecretName()
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
	var formattedProcessorLogs, briefErrorsMessage string
	var stopWritingRawLinesToBriefErrorsMessage bool

	briefErrorsArray := &[]string{}

	for scanner.Scan() {
		currentLogLine, briefLogLine, err := ap.prettifyProcessorLogLine(scanner.Bytes())
		if err != nil {
			rawLogLine := scanner.Text()

			// when it is unstructured just add the log as a text
			formattedProcessorLogs += rawLogLine + "\n"

			// if there's a panic or call stack is printed,
			// stop appending raw log lines to the briefErrorsMessage (unnecessary information from now on)
			// this information can still be found in the full log
			if strings.HasPrefix(rawLogLine, "panic: Wrapper") ||
				strings.HasPrefix(rawLogLine, "Call stack:") {
				stopWritingRawLinesToBriefErrorsMessage = true
			}

			if !stopWritingRawLinesToBriefErrorsMessage {
				*briefErrorsArray = append(*briefErrorsArray, rawLogLine)
			}

			continue
		}

		if briefLogLine != "" {
			*briefErrorsArray = append(*briefErrorsArray, briefLogLine)
		}

		// when it is a log line generated by the processor
		formattedProcessorLogs += currentLogLine + "\n"
	}

	*briefErrorsArray = ap.aggregateConsecutiveDuplicateMessages(*briefErrorsArray)

	// create brief errors log as string, and remove double newlines
	briefErrorsMessage = strings.Join(*briefErrorsArray, "\n")

	return common.FixEscapeChars(formattedProcessorLogs), common.FixEscapeChars(briefErrorsMessage)
}

func (ap *Platform) aggregateConsecutiveDuplicateMessages(errorMessagesArray []string) []string {
	var aggregatedErrorsArray []string

	for i := 0; i < len(errorMessagesArray); i++ {
		currentErrorMessage := errorMessagesArray[i]
		consecutiveErrorMessageCount := 1

		// count how many consecutive times current error message reoccurs
		for i+1 < len(errorMessagesArray) && errorMessagesArray[i+1] == currentErrorMessage {
			consecutiveErrorMessageCount++
			i++
		}

		if consecutiveErrorMessageCount > 1 {
			aggregatedErrorsArray = append(aggregatedErrorsArray,
				fmt.Sprintf("[repeated %d times] %s", consecutiveErrorMessageCount, currentErrorMessage))
			continue
		}

		aggregatedErrorsArray = append(aggregatedErrorsArray, currentErrorMessage)
	}

	return aggregatedErrorsArray
}

// Prettifies log line, and returns - (formattedLogLine, briefLogLine, error)
// when line shouldn't be added to brief error message - briefLogLine will be an empty string ("")
func (ap *Platform) prettifyProcessorLogLine(log []byte) (string, string, error) {
	var workerID, logStructArgs string

	logStruct := struct {
		Time    *string `json:"time"`
		Level   *string `json:"level"`
		Message *string `json:"message"`
		Name    *string `json:"name,omitempty"`
		More    *string `json:"more,omitempty"`
	}{}

	if ap.isSDKLogLine(log) {

		// when it is a wrapper log line
		wrapperLogStruct := struct {
			Datetime *string           `json:"datetime"`
			Level    *string           `json:"level"`
			Message  *string           `json:"message"`
			Name     *string           `json:"name,omitempty"`
			With     map[string]string `json:"with,omitempty"`
		}{}

		if err := json.Unmarshal(log[1:], &wrapperLogStruct); err != nil {
			return "", "", err
		}

		// manipulate the time format so it can be parsed later
		unparsedTime := *wrapperLogStruct.Datetime + "Z"
		unparsedTime = strings.Replace(unparsedTime, " ", "T", 1)
		unparsedTime = strings.Replace(unparsedTime, ",", ".", 1)

		logStruct.Time = &unparsedTime
		logStruct.Level = wrapperLogStruct.Level
		logStruct.Message = wrapperLogStruct.Message
		logStruct.Name = wrapperLogStruct.Name

		if wrapperLogStruct.With != nil {
			workerID = wrapperLogStruct.With["worker_id"]

			more := common.CreateKeyValuePairs(wrapperLogStruct.With)
			logStruct.More = &more
		}

	} else {

		// when it is a log line generated by the processor
		if err := json.Unmarshal(log, &logStruct); err != nil {
			return "", "", err
		}
	}

	// check required fields existence
	if logStruct.Time == nil || logStruct.Level == nil || logStruct.Message == nil {
		return "", "", errors.New("Missing required fields in pod log line")
	}

	parsedTime, err := time.Parse(time.RFC3339, *logStruct.Time)
	if err != nil {
		return "", "", err
	}

	logLevel := strings.ToUpper(*logStruct.Level)[0]

	// if worker ID wasn't explicitly given as an arg, try to infer worker ID from logger name
	if workerID == "" && logStruct.Name != nil {
		workerID = ap.tryInferWorkerID(*logStruct.Name)
	}

	if logStruct.More != nil {
		logStructArgs = *logStruct.More
	}
	messageAndArgs := ap.getMessageAndArgs(*logStruct.Message, logStructArgs, log, workerID)

	res := fmt.Sprintf("[%s] (%c) %s",
		parsedTime.Format("15:04:05.000"),
		logLevel,
		messageAndArgs)

	briefLogLine := ""
	if ap.shouldAddToBriefErrorsMessage(logLevel, *logStruct.Message, workerID) {
		briefLogLine = messageAndArgs
	}

	return res, briefLogLine, nil
}

// get the worker ID from the logger name, for example:
// "processor.http.w5.python.logger" -> 5
func (ap *Platform) tryInferWorkerID(loggerName string) string {
	processorRe := regexp.MustCompile(`^processor\..*\.w[0-9]+\..*`)
	if processorRe.MatchString(loggerName) {
		splitName := strings.Split(loggerName, ".")
		return splitName[2][1:]
	}

	return ""
}

func (ap *Platform) getMessageAndArgs(message string, args string, log []byte, workerID string) string {
	var additionalKwargsAsString string

	additionalKwargs, err := ap.getLogLineAdditionalKwargs(log)
	if err != nil {
		ap.Logger.WarnWith("Failed to get log line's additional kwargs",
			"logLineMessage", message)
	}
	additionalKwargsAsString = common.CreateKeyValuePairs(additionalKwargs)

	// format result depending on args/additional kwargs existence
	var messageArgsList []string
	if args != "" {
		messageArgsList = append(messageArgsList, args)
	}
	if additionalKwargsAsString != "" {
		messageArgsList = append(messageArgsList, additionalKwargsAsString)
	}
	if len(messageArgsList) > 0 {
		return fmt.Sprintf("%s [%s]", message, strings.Join(messageArgsList, " || "))
	}

	return message
}

func (ap *Platform) getLogLineAdditionalKwargs(log []byte) (map[string]string, error) {
	logAsMap := map[string]interface{}{}

	if ap.isSDKLogLine(log) {
		if err := json.Unmarshal(log[1:], &logAsMap); err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshal log line")
		}
	} else if err := json.Unmarshal(log, &logAsMap); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal log line")
	}

	additionalKwargs := map[string]string{}

	defaultArgs := []string{"time", "datetime", "level", "message", "with", "more", "name"}

	// validate it is a suitable special arg
	for argKey, argValue := range logAsMap {

		// validate it is indeed an additional arg - it isn't a default arg
		if common.StringSliceContainsString(defaultArgs, argKey) {
			continue
		}

		// ensure argument is a string
		if _, ok := argValue.(string); !ok {
			continue
		}

		additionalKwargs[argKey] = argValue.(string)
	}

	return additionalKwargs, nil
}

func (ap *Platform) isSDKLogLine(logLine []byte) bool {
	return len(logLine) > 0 && logLine[0] == 'l'
}

func (ap *Platform) shouldAddToBriefErrorsMessage(logLevel uint8, logMessage, workerID string) bool {
	knownFailureSubstrings := [...]string{"Failed to connect to broker"}
	ignoreFailureSubstrings := [...]string{
		string(common.UnexpectedTerminationChildProcess),
		string(common.FailedReadFromConnection),
	}

	// when the log message contains a failure that should be ignored
	for _, ignoreFailureSubstring := range ignoreFailureSubstrings {
		if strings.Contains(logMessage, ignoreFailureSubstring) {
			return false
		}
	}

	// show errors only of the first worker
	// done to prevent error duplication from several workers
	if workerID != "" && workerID != "0" {
		return false
	}
	// when log level is warning or above
	if logLevel != 'D' && logLevel != 'I' {
		return true
	}

	// when the log message contains a known failure substring
	for _, knownFailureSubstring := range knownFailureSubstrings {
		if strings.Contains(logMessage, knownFailureSubstring) {
			return true
		}
	}

	return false
}

// Function must have project name - if it was not given - set to default project
func (ap *Platform) enrichProjectName(createFunctionOptions *platform.CreateFunctionOptions) error {

	// if no project name was given, set it to the default project
	if createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"] == "" {
		createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"] = platform.DefaultProjectName
		ap.Logger.Debug("No project name specified. Setting to default")
	}

	return nil
}

// If a user specify the image name to be built - add "projectName-functionName-" prefix to it
func (ap *Platform) enrichImageName(createFunctionOptions *platform.CreateFunctionOptions) error {
	if ap.ImageNamePrefixTemplate == "" {
		return nil
	}
	functionName := createFunctionOptions.FunctionConfig.Meta.Name
	projectName := createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"]

	functionBuildRequired, err := ap.functionBuildRequired(createFunctionOptions)
	if err != nil {
		return errors.Wrap(err, "Failed determining whether function build is required for image name enrichment")
	}

	// if build is not required or custom image name was not asked enrichment is irrelevant
	// note that leaving Spec.Build.Image will cause further enrichment deeper in build/builder.go.
	// TODO: Revisit the need for this logic being stretched on so many places
	if !functionBuildRequired || createFunctionOptions.FunctionConfig.Spec.Build.Image == "" {
		return nil
	}

	imagePrefix, err := ap.RenderImageNamePrefixTemplate(projectName, functionName)

	if err != nil {
		return errors.Wrap(err, "Failed to render image name prefix template")
	}

	// avoid re-enrichment
	if !strings.HasPrefix(createFunctionOptions.FunctionConfig.Spec.Build.Image, imagePrefix) {

		createFunctionOptions.FunctionConfig.Spec.Build.Image = fmt.Sprintf("%s%s",
			imagePrefix, createFunctionOptions.FunctionConfig.Spec.Build.Image)
	}

	return nil
}

func (ap *Platform) validateMinMaxReplicas(createFunctionOptions *platform.CreateFunctionOptions) error {
	minReplicas := createFunctionOptions.FunctionConfig.Spec.MinReplicas
	maxReplicas := createFunctionOptions.FunctionConfig.Spec.MaxReplicas

	if minReplicas != nil {
		if maxReplicas == nil && *minReplicas == 0 {
			return errors.New("Max replicas must be set when min replicas is zero")
		}
		if maxReplicas != nil && *minReplicas > *maxReplicas {
			return errors.New("Min replicas must be less than or equal to max replicas")
		}
	}
	if maxReplicas != nil && *maxReplicas == 0 {
		return errors.New("Max replicas must be greater than zero")
	}

	return nil
}

func (ap *Platform) validateProjectExists(createFunctionOptions *platform.CreateFunctionOptions) error {

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
		return errors.New("Project does not exist")
	}
	return nil
}

func (ap *Platform) validateTriggers(createFunctionOptions *platform.CreateFunctionOptions) error {

	var httpTriggerExists bool
	for triggerName, _trigger := range createFunctionOptions.FunctionConfig.Spec.Triggers {

		// no more workers than limitation allows
		if _trigger.MaxWorkers > trigger.MaxWorkersLimit {
			return errors.Errorf("MaxWorkers value for %s trigger (%d) exceeds the limit of %d",
				triggerName,
				_trigger.MaxWorkers,
				trigger.MaxWorkersLimit)
		}

		// no more than one http trigger is allowed
		if _trigger.Kind == "http" {
			if !httpTriggerExists {
				httpTriggerExists = true
				continue
			}
			return errors.New("There's more than one http trigger (unsupported)")
		}
	}
	return nil
}

func (ap *Platform) enrichMinMaxReplicas(createFunctionOptions *platform.CreateFunctionOptions) {

	// if min replicas was not set, and max replicas is set, assign max replicas to min replicas
	if createFunctionOptions.FunctionConfig.Spec.MinReplicas == nil &&
		createFunctionOptions.FunctionConfig.Spec.MaxReplicas != nil {
		createFunctionOptions.FunctionConfig.Spec.MinReplicas = createFunctionOptions.FunctionConfig.Spec.MaxReplicas
	}

	// if max replicas was not set, and min replicas is set, assign min replicas to max replicas
	if createFunctionOptions.FunctionConfig.Spec.MaxReplicas == nil &&
		createFunctionOptions.FunctionConfig.Spec.MinReplicas != nil {
		createFunctionOptions.FunctionConfig.Spec.MaxReplicas = createFunctionOptions.FunctionConfig.Spec.MinReplicas
	}
}
