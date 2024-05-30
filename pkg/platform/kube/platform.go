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

package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	externalproject "github.com/nuclio/nuclio/pkg/platform/abstract/project/external"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/internalc/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/samber/lo"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/cache"
)

type Platform struct {
	*abstract.Platform
	deployer           *client.Deployer
	getter             *client.Getter
	updater            *client.Updater
	deleter            *client.Deleter
	kubeconfigPath     string
	consumer           *client.Consumer
	projectsClient     project.Client
	projectsCache      *cache.Expiring
	apiGatewayScrubber *platform.APIGatewayScrubber
}

const Mib = 1048576

func NewProjectsClient(platform *Platform, platformConfiguration *platformconfig.Config) (project.Client, error) {

	// create kube projects client
	kubeProjectsClient, err := kube.NewClient(platform.Logger, platform, platform.consumer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create internal projects client (kube)")
	}

	if platformConfiguration.ProjectsLeader != nil {

		// wrap external client around kube projects client as internal client
		return externalproject.NewClient(platform.Logger,
			kubeProjectsClient,
			platformConfiguration)
	}

	return kubeProjectsClient, nil
}

// NewPlatform instantiates a new kubernetes platform
func NewPlatform(ctx context.Context,
	parentLogger logger.Logger,
	platformConfiguration *platformconfig.Config,
	defaultNamespace string) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewPlatform(parentLogger, newPlatform, platformConfiguration, defaultNamespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create an abstract platform")
	}

	// init platform
	newPlatform.Platform = newAbstractPlatform

	// we run GetKubeConfigClientCmdByKubeconfigPath in order to check if we are running in k8s
	// empty error means that the kubeconfig path was found, the configuration at this path exists and can be loaded successfully
	// if error is not nil, we leave kubeconfigPath empty and use the in-cluster k8s configuration when creating the consumer below
	if _, err := common.GetKubeConfigClientCmdByKubeconfigPath(platformConfiguration.Kube.KubeConfigPath); err == nil {
		newPlatform.kubeconfigPath = common.GetKubeconfigPath(platformConfiguration.Kube.KubeConfigPath)
	}

	// create consumer
	newPlatform.consumer, err = client.NewConsumer(ctx, newPlatform.Logger, newPlatform.kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a consumer")
	}

	// create deployer
	newPlatform.deployer, err = client.NewDeployer(newPlatform.Logger, newPlatform.consumer, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a deployer")
	}

	// create getter
	newPlatform.getter, err = client.NewGetter(newPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a getter")
	}

	// create deleter
	newPlatform.deleter, err = client.NewDeleter(newPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a deleter")
	}

	// create updater
	newPlatform.updater, err = client.NewUpdater(newPlatform.Logger, newPlatform.consumer, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create an updater")
	}

	// set kubeClientSet for Function Scrubber
	newPlatform.FunctionScrubber = functionconfig.NewScrubber(parentLogger,
		platformConfiguration.SensitiveFields.CompileSensitiveFieldsRegex(),
		newPlatform.consumer.KubeClientSet,
	)

	// create api gateway scrubber
	newPlatform.apiGatewayScrubber = platform.NewAPIGatewayScrubber(parentLogger, platform.GetAPIGatewaySensitiveField(),
		newPlatform.consumer.KubeClientSet)

	// create projects client
	newPlatform.projectsClient, err = NewProjectsClient(newPlatform, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create projects client")
	}

	newPlatform.projectsCache = cache.NewExpiring()

	return newPlatform, nil
}

func (p *Platform) Initialize(ctx context.Context) error {
	if err := p.projectsClient.Initialize(); err != nil {
		return errors.Wrap(err, "Failed to initialize projects client")
	}

	// ensure default project existence only when projects aren't managed by external leader
	if p.Config.ProjectsLeader == nil {
		if err := p.EnsureDefaultProjectExistence(ctx); err != nil {
			return errors.Wrap(err, "Failed to ensure default project existence")
		}
	}

	return nil
}

// CreateFunction will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) CreateFunction(ctx context.Context, createFunctionOptions *platform.CreateFunctionOptions) (
	*platform.CreateFunctionResult, error) {

	var err error
	var existingFunctionInstance *nuclioio.NuclioFunction
	var existingFunctionConfig *functionconfig.ConfigWithStatus

	// make sure container builder is initialized
	if err := p.InitializeContainerBuilder(); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize container builder")
	}

	if err := p.enrichAndValidateFunctionConfig(ctx, &createFunctionOptions.FunctionConfig, createFunctionOptions.AutofixConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to enrich and validate a function configuration")
	}

	// Check OPA permissions
	permissionOptions := createFunctionOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionPermissions(
		createFunctionOptions.FunctionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		createFunctionOptions.FunctionConfig.Meta.Name,
		opa.ActionCreate,
		&permissionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	// it's possible to pass a function without specifying any meta in the request, in that case skip getting existing function
	// with appropriate namespace and name
	// e.g. ./nuctl deploy --path /path/to/function-with-function.yaml (function.yaml specifying the name and namespace)
	// TODO: We should enrich the createFunctionOptions.FunctionConfig meta & spec before reaching here
	// And delete this check
	if createFunctionOptions.FunctionConfig.Meta.Namespace != "" &&
		createFunctionOptions.FunctionConfig.Meta.Name != "" {
		existingFunctionInstance, existingFunctionConfig, err =
			p.getFunctionInstanceAndConfig(ctx,
				&platform.GetFunctionsOptions{
					Namespace:             createFunctionOptions.FunctionConfig.Meta.Namespace,
					Name:                  createFunctionOptions.FunctionConfig.Meta.Name,
					ResourceVersion:       createFunctionOptions.FunctionConfig.Meta.ResourceVersion,
					EnrichWithAPIGateways: true,
				})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get an existing function configuration")
		}
	}

	// if function exists, perform some validation with new function create options
	if err := p.ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(ctx,
		existingFunctionConfig,
		createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed to validate a function configuration against an existing configuration")
	}

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams.Store(createFunctionOptions.FunctionConfig.Meta.GetUniqueID(), logStream)

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	// called when function creation failed, update function status with failure
	reportCreationError := func(creationError error, briefErrorsMessage string, clearCallStack bool) error {
		errorStack := bytes.Buffer{}
		errors.PrintErrorStack(&errorStack, creationError, 20)

		// cut messages that are too big
		if errorStack.Len() >= 4*Mib {
			errorStack.Truncate(4 * Mib)
		}

		// when no brief error message was passed - infer it from the creation error
		if briefErrorsMessage == "" {
			rootCause := errors.RootCause(creationError)

			// when clearCallStack is requested and there's a root cause - set it to be the specific root cause
			if clearCallStack && rootCause != nil {
				briefErrorsMessage = rootCause.Error()

				// otherwise, set it to be the whole error stack
			} else {
				briefErrorsMessage = errorStack.String()
			}
		}

		if clearCallStack {
			briefErrorsMessage = p.clearCallStack(briefErrorsMessage)
		}

		// low severity to not over log in the warning
		createFunctionOptions.Logger.DebugWithCtx(ctx, "Function creation failed, brief error message extracted",
			"briefErrorsMessage", briefErrorsMessage)

		functionStatus := &functionconfig.Status{
			State:   functionconfig.FunctionStateError,
			Message: briefErrorsMessage,
		}
		if existingFunctionInstance != nil {

			// preserve invocation metadata for when function become healthy again
			functionStatus.HTTPPort = existingFunctionInstance.Status.HTTPPort
			functionStatus.ExternalInvocationURLs = existingFunctionInstance.Status.ExternalInvocationURLs
			functionStatus.InternalInvocationURLs = existingFunctionInstance.Status.InternalInvocationURLs
			functionStatus.Logs = existingFunctionInstance.Status.Logs

			// if function deployment ended up with unhealthy, due to unstable Kubernetes env that lead
			// to failing on waiting for function readiness.
			// it is desired to preserve the function unhealthiness state set by the controller, to allow
			// function recovery later on, when Kubernetes become stable
			// alternatively, set function in error state to indicate deployment has failed
			if existingFunctionInstance.Status.State == functionconfig.FunctionStateUnhealthy {
				functionStatus.State = functionconfig.FunctionStateUnhealthy
			}
		}

		if functionStatus.State == functionconfig.FunctionStateUnhealthy {
			createFunctionOptions.Logger.WarnWithCtx(ctx, "Function deployment failed, setting state to unhealthy. The issue might be transient or require manual redeployment",
				"err", errorStack.String())
		} else {
			createFunctionOptions.Logger.WarnWithCtx(ctx, "Function creation failed, setting state to error",
				"err", errorStack.String())
		}

		// create or update the function. The possible creation needs to happen here, since on cases of
		// early build failures we might get here before the function CR was created. After this point
		// it is guaranteed to be created and updated with the reported error state
		if _, err := p.deployer.CreateOrUpdateFunction(ctx,
			existingFunctionInstance,
			createFunctionOptions,
			functionStatus,
		); err != nil {
			return errors.Wrap(err, "Failed to create or update function")
		}
		return nil
	}

	// the builder may update the configuration, so we have to create the function in the platform only after
	// the builder does that
	onAfterConfigUpdated := func() error {
		var err error

		// enrich and validate again because it may not be valid after config was updated by external code entry type
		if err := p.enrichAndValidateFunctionConfig(ctx, &createFunctionOptions.FunctionConfig, createFunctionOptions.AutofixConfiguration); err != nil {
			return errors.Wrap(err, "Failed to enrich and validate an updated function configuration")
		}

		existingFunctionInstance, err = p.getFunction(ctx,
			&platform.GetFunctionsOptions{
				Namespace:       createFunctionOptions.FunctionConfig.Meta.Namespace,
				Name:            createFunctionOptions.FunctionConfig.Meta.Name,
				ResourceVersion: createFunctionOptions.FunctionConfig.Meta.ResourceVersion,
			})
		if err != nil {
			return errors.Wrap(err, "Failed to get a function")
		}

		// if the function already exists then it either doesn't have the FunctionAnnotationSkipDeploy annotation, or it
		// was imported and has the annotation, but on this recreate it shouldn't. So the annotation should be removed.
		if existingFunctionInstance != nil {
			createFunctionOptions.FunctionConfig.Meta.RemoveSkipDeployAnnotation()
		}

		// create or update the function if it exists. If functionInstance is nil, the function will be created
		// with the configuration and status. if it exists, it will be updated with the configuration and status.
		// the goal here is for the function to exist prior to building so that it is gettable
		existingFunctionInstance, err = p.deployer.CreateOrUpdateFunction(ctx,
			existingFunctionInstance,
			createFunctionOptions,
			&functionconfig.Status{
				State: functionconfig.FunctionStateBuilding,
			})
		if err != nil {
			return errors.Wrap(err, "Failed to create or update a function before build")
		}

		// indicate that the creation state has been updated
		if createFunctionOptions.CreationStateUpdated != nil {
			createFunctionOptions.CreationStateUpdated <- true
		}

		return nil
	}

	// called after function was built
	onAfterBuild := func(buildResult *platform.CreateFunctionBuildResult,
		buildErr error) (*platform.CreateFunctionResult, error) {

		skipDeploy := functionconfig.ShouldSkipDeploy(createFunctionOptions.FunctionConfig.Meta.Annotations)

		// after a function build (or skip-build) if the annotation FunctionAnnotationSkipBuild exists, it should be removed
		// so next time, the build will happen. (skip-deploy will be removed on next update so the controller can use the
		// annotation as well).
		createFunctionOptions.FunctionConfig.Meta.RemoveSkipBuildAnnotation()

		if buildErr != nil {

			// try to report the error
			reportingErr := reportCreationError(buildErr, "", false)
			if reportingErr != nil {
				p.Logger.ErrorWithCtx(ctx, "Failed to report a creation error",
					"reportingErr", reportingErr,
					"buildErr", buildErr)
				return nil, reportingErr
			}
			return nil, buildErr
		}

		if err := p.setScaleToZeroSpec(&createFunctionOptions.FunctionConfig.Spec); err != nil {
			return nil, errors.Wrap(err, "Failed to set the scale-to-zero spec")
		}

		if skipDeploy {
			p.Logger.InfoWithCtx(ctx,
				"Skipping function deployment",
				"functionName", createFunctionOptions.FunctionConfig.Meta.Name,
				"functionNamespace", createFunctionOptions.FunctionConfig.Meta.Namespace)

			if _, err := p.deployer.CreateOrUpdateFunction(ctx,
				existingFunctionInstance,
				createFunctionOptions,
				&functionconfig.Status{
					State: functionconfig.FunctionStateImported,
				}); err != nil {
				return nil, errors.Wrap(err, "Failed to create/update imported function")
			}

			return &platform.CreateFunctionResult{
				CreateFunctionBuildResult: platform.CreateFunctionBuildResult{
					Image:                 createFunctionOptions.FunctionConfig.Spec.Image,
					UpdatedFunctionConfig: createFunctionOptions.FunctionConfig,
				},
			}, nil
		}

		createFunctionResult, updatedFunctionInstance, briefErrorsMessage, deployErr := p.deployer.Deploy(ctx,
			existingFunctionInstance,
			createFunctionOptions)

		// update the function instance (after the deployment)
		if updatedFunctionInstance != nil {
			existingFunctionInstance = updatedFunctionInstance
		}

		if deployErr != nil {

			// try to report the error
			reportingErr := reportCreationError(deployErr, briefErrorsMessage, true)
			if reportingErr != nil {
				p.Logger.ErrorWithCtx(ctx, "Failed to report a deployment error",
					"reportingErr", reportingErr.Error(),
					"buildErr", buildErr)
				return nil, reportingErr
			}

			return nil, deployErr
		}

		return createFunctionResult, nil
	}

	// do the deploying in the abstract base class
	return p.HandleDeployFunction(ctx, existingFunctionConfig, createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
}

func (p *Platform) EnrichFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {
	if err := p.Platform.EnrichFunctionConfig(ctx, functionConfig); err != nil {
		return err
	}

	if err := p.enrichHTTPTriggers(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich http trigger")
	}

	if err := p.enrichFunctionNodeSelector(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich node selector")
	}

	// enrich function tolerations
	if functionConfig.Spec.Tolerations == nil && p.Config.Kube.DefaultFunctionTolerations != nil {
		p.Logger.DebugWithCtx(ctx,
			"Enriching function tolerations",
			"functionName", functionConfig.Meta.Name,
			"tolerations", p.Config.Kube.DefaultFunctionTolerations)
		functionConfig.Spec.Tolerations = p.Config.Kube.DefaultFunctionTolerations
	}

	// enrich function pod priority class name
	if functionConfig.Spec.PriorityClassName == "" && p.Config.Kube.DefaultFunctionPriorityClassName != "" {
		p.Logger.DebugWithCtx(ctx,
			"Enriching pod priority class name",
			"functionName", functionConfig.Meta.Name,
			"priorityClassName", p.Config.Kube.DefaultFunctionPriorityClassName)
		functionConfig.Spec.PriorityClassName = p.Config.Kube.DefaultFunctionPriorityClassName
	}

	// enrich function service account
	if functionConfig.Spec.ServiceAccount == "" && p.Config.Kube.DefaultFunctionServiceAccount != "" {
		p.Logger.DebugWithCtx(ctx,
			"Enriching service account",
			"functionName", functionConfig.Meta.Name,
			"serviceAccount", p.Config.Kube.DefaultFunctionServiceAccount)
		functionConfig.Spec.ServiceAccount = p.Config.Kube.DefaultFunctionServiceAccount
	}

	p.enrichFunctionPreemptionSpec(ctx, p.Config.Kube.PreemptibleNodes, functionConfig)
	p.enrichInitContainersSpec(functionConfig)
	p.enrichSidecarsSpec(functionConfig)
	return nil
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(ctx context.Context, getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	projectName, err := p.Platform.ResolveProjectNameFromLabelsStr(getFunctionsOptions.Labels)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	if err := p.Platform.EnsureProjectRead(projectName, &getFunctionsOptions.PermissionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed to ensure project read permission")
	}

	functions, err := p.getter.Get(ctx, p.consumer, getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functions, err = p.Platform.FilterFunctionsByPermissions(ctx, &getFunctionsOptions.PermissionOptions, functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to filter functions by permissions")
	}

	p.EnrichFunctionsWithDeployLogStream(functions)

	if getFunctionsOptions.EnrichWithAPIGateways {
		if err = p.enrichFunctionsWithAPIGateways(ctx, functions, getFunctionsOptions.Namespace); err != nil {

			// relevant when upgrading nuclio from a version that didn't have api-gateways to one that has
			if !strings.Contains(errors.RootCause(err).Error(),
				"the server could not find the requested resource (get nuclioapigateways.nuclio.io)") {

				return nil, errors.Wrap(err, "Failed to enrich functions with API gateways")
			}

			p.Logger.DebugWithCtx(ctx, "Api-gateway crd isn't installed; skipping function api gateways enrichment",
				"err", err)
		}
	}

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(ctx context.Context, updateFunctionOptions *platform.UpdateFunctionOptions) error {
	return p.updater.Update(ctx, updateFunctionOptions)
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(ctx context.Context, deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	p.Logger.DebugWithCtx(ctx, "Deleting function",
		"functionConfig", deleteFunctionOptions.FunctionConfig)

	// pre delete validation
	functionToDelete, err := p.ValidateDeleteFunctionOptions(ctx, deleteFunctionOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to validate function-deletion options")
	}

	// nothing to delete
	if functionToDelete == nil {
		return nil
	}

	if !deleteFunctionOptions.DeleteApiGateways {
		// user must clean api gateway before deleting the function
		if err := p.validateFunctionHasNoAPIGateways(ctx, deleteFunctionOptions); err != nil {
			return errors.Wrap(err, "Failed to validate that the function has no API gateways")
		}
	} else {

		apiGateways, err := p.getApiGatewaysForFunction(ctx, deleteFunctionOptions.FunctionConfig.Meta.Namespace, deleteFunctionOptions.FunctionConfig.Meta.Name)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to get API gateways for function %s",
				deleteFunctionOptions.FunctionConfig.Meta.Name))
		}
		deleteAPIGatewayOptions := &platform.DeleteAPIGatewayOptions{
			AuthSession: deleteFunctionOptions.AuthSession,
		}
		for _, apiGatewayInstance := range apiGateways {
			// check if there is any canary function in which this function is used
			// if there is one, we not allow deleting such functions
			if len(apiGatewayInstance.Spec.Upstreams) == 2 {
				return errors.New("Failed to delete function - it is used in canary api gateway")
			}
		}
		for _, apiGatewayInstance := range apiGateways {
			deleteAPIGatewayOptions.Meta = platform.APIGatewayMeta{
				Name:      apiGatewayInstance.Name,
				Namespace: apiGatewayInstance.Namespace,
			}
			if err := p.DeleteAPIGateway(ctx, deleteAPIGatewayOptions); err != nil {
				return errors.Wrap(err, fmt.Sprintf("Failed to delete api gateway %s associated with a function %s",
					apiGatewayInstance.Name, deleteFunctionOptions.FunctionConfig.Meta.Name))
			}
		}
	}

	return p.deleter.Delete(ctx, p.consumer, deleteFunctionOptions)
}

// RedeployFunction will redeploy a previously deployed function
func (p *Platform) RedeployFunction(ctx context.Context, redeployFunctionOptions *platform.RedeployFunctionOptions) error {

	// Check OPA permissions
	permissionOptions := redeployFunctionOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionRedeployPermissions(
		redeployFunctionOptions.FunctionMeta.Labels[common.NuclioResourceLabelKeyProjectName],
		redeployFunctionOptions.FunctionMeta.Name,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}
	var state functionconfig.FunctionState

	switch redeployFunctionOptions.DesiredState {
	case functionconfig.FunctionStateScaledToZero:
		state = functionconfig.FunctionStateWaitingForScaleResourcesToZero
	default:
		// setting a different function state is not supported so we fallback to ready
		state = functionconfig.FunctionStateWaitingForResourceConfiguration
	}

	// update the function CRD state to waiting for resource configuration, so the controller will redeploy its resources
	if err := p.updater.UpdateState(ctx,
		redeployFunctionOptions.FunctionMeta.Name,
		redeployFunctionOptions.FunctionMeta.Namespace,
		redeployFunctionOptions.AuthConfig,
		state); err != nil {
		return errors.Wrap(err, "Failed to update function state")
	}
	return nil
}

func (p *Platform) GetFunctionReplicaLogsStream(ctx context.Context,
	options *platform.GetFunctionReplicaLogsStreamOptions) (io.ReadCloser, error) {
	return p.consumer.KubeClientSet.
		CoreV1().
		Pods(options.Namespace).
		GetLogs(options.Name, &v1.PodLogOptions{
			Container:    options.ContainerName,
			SinceSeconds: options.SinceSeconds,
			TailLines:    options.TailLines,
			Follow:       options.Follow,
		}).
		Stream(ctx)
}

func (p *Platform) GetFunctionReplicaNames(ctx context.Context,
	functionConfig *functionconfig.Config) ([]string, error) {

	pods, err := p.consumer.KubeClientSet.
		CoreV1().
		Pods(functionConfig.Meta.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: common.CompileListFunctionPodsLabelSelector(functionConfig.Meta.Name),
		})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function pods")
	}
	var names []string
	for _, pod := range pods.Items {
		names = append(names, pod.GetName())
	}
	return names, nil
}

func (p *Platform) GetFunctionReplicaContainers(ctx context.Context, functionConfig *functionconfig.Config, replicaName string) ([]string, error) {
	pod, err := p.consumer.KubeClientSet.
		CoreV1().
		Pods(functionConfig.Meta.Namespace).
		Get(ctx, replicaName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function pod")
	}
	var containerNames []string
	for _, container := range pod.Spec.Containers {
		containerNames = append(containerNames, container.Name)
	}
	return containerNames, nil
}

// GetName returns the platform name
func (p *Platform) GetName() string {
	return common.KubePlatformName
}

// CreateProject creates a new project
func (p *Platform) CreateProject(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) error {

	// enrich
	if err := p.EnrichCreateProjectConfig(createProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to enrich a project configuration")
	}

	// validate
	if err := p.ValidateProjectConfig(createProjectOptions.ProjectConfig); err != nil {
		return errors.Wrap(err, "Failed to validate a project configuration")
	}

	// create
	p.Logger.DebugWithCtx(ctx,
		"Creating project",
		"projectName", createProjectOptions.ProjectConfig.Meta.Name)
	createdProject, err := p.projectsClient.Create(ctx, createProjectOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to create project")
	}

	// adding to cache for 30 seconds, allowing
	p.projectsCache.Set(
		p.getProjectCacheKey(
			createProjectOptions.ProjectConfig.Meta,
			createProjectOptions.ProjectConfig.Spec.Owner,
		),
		createdProject,
		time.Second*30)

	return nil
}

// UpdateProject updates an existing project
func (p *Platform) UpdateProject(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) error {
	if err := p.ValidateProjectConfig(&updateProjectOptions.ProjectConfig); err != nil {
		return nuclio.WrapErrBadRequest(err)
	}

	if _, err := p.projectsClient.Update(ctx, updateProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to update project")
	}

	return nil
}

// DeleteProject will delete a previously existing project
func (p *Platform) DeleteProject(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {

	// enrich to protect test flows where auth session is nil
	if deleteProjectOptions.AuthSession == nil {
		deleteProjectOptions.AuthSession = &auth.NopSession{}
	}

	if err := p.Platform.ValidateDeleteProjectOptions(ctx, deleteProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to validate delete project options")
	}

	// check only, do not delete
	if deleteProjectOptions.Strategy == platform.DeleteProjectStrategyCheck {
		p.Logger.DebugWithCtx(ctx,
			"Project is ready for deletion",
			"projectMeta", deleteProjectOptions.Meta)
		return nil
	}

	p.Logger.DebugWithCtx(ctx,
		"Deleting project",
		"projectMeta", deleteProjectOptions.Meta)
	if err := p.projectsClient.Delete(ctx, deleteProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to delete project")
	}

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		if err := p.Platform.WaitForProjectResourcesDeletion(ctx,
			&deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration); err != nil {
			return errors.Wrap(err, "Failed waiting for project resources deletion")
		}
	}

	// cache revocation
	p.projectsCache.Delete(p.getProjectCacheKey(deleteProjectOptions.Meta,
		deleteProjectOptions.AuthSession.GetUsername()))
	return nil
}

// GetProjects will list existing projects
func (p *Platform) GetProjects(ctx context.Context,
	getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {

	// enrich to protect test flows where auth session is nil
	if getProjectsOptions.AuthSession == nil {
		getProjectsOptions.AuthSession = &auth.NopSession{}
	}

	projects, err := p.projectsClient.Get(ctx, getProjectsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting projects")
	}

	filteredProjectList, err := p.Platform.FilterProjectsByPermissions(
		ctx,
		&getProjectsOptions.PermissionOptions,
		projects)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to filter projects by permission")
	}

	// tl;dr simply bypass opa manifest authorization
	// iterate over the original retrieved project list
	// if has been recently cached, re-add to filtered project list
	// this is done to avoid cases where
	// 1. project is being created by leader
	// 2. opa manifest distribution did not occur yet
	// 3. user GET on his recently added project
	for _, projectInstance := range projects {
		if _, exist := p.projectsCache.Get(
			p.getProjectCacheKey(
				projectInstance.GetConfig().Meta,
				getProjectsOptions.AuthSession.GetUsername(),
			),
		); exist {
			var found bool
			for _, filteredProject := range filteredProjectList {
				if filteredProject.GetConfig().Meta.IsEqual(projectInstance.GetConfig().Meta) {
					found = true
					break
				}
			}

			// re-add project instance to filtered out as it has been cached
			if !found {
				p.Logger.DebugWithCtx(ctx,
					"Project is read from cache",
					"projectName", projectInstance.GetConfig().Meta.Name)
				filteredProjectList = append(filteredProjectList, projectInstance)
			}
		}
	}

	return filteredProjectList, nil
}

// CreateAPIGateway creates and deploys a new api gateway
func (p *Platform) CreateAPIGateway(ctx context.Context,
	createAPIGatewayOptions *platform.CreateAPIGatewayOptions) error {
	newAPIGateway := &nuclioio.NuclioAPIGateway{}

	// enrich
	p.enrichAPIGatewayConfig(ctx, createAPIGatewayOptions.APIGatewayConfig, nil)

	// validate
	if err := p.validateAPIGatewayConfig(ctx,
		createAPIGatewayOptions.APIGatewayConfig,
		createAPIGatewayOptions.ValidateFunctionsExistence,
		nil); err != nil {
		return errors.Wrap(err, "Failed to validate and enrich an API-gateway name")
	}

	// scrub api gateway config
	if p.GetConfig().SensitiveFields.MaskSensitiveFields {
		scrubbedConfig, err := p.apiGatewayScrubber.ScrubAPIGatewayConfig(ctx, createAPIGatewayOptions.APIGatewayConfig)
		if err != nil {
			return errors.Wrap(err, "Failed to scrub api gateway config")
		}
		createAPIGatewayOptions.APIGatewayConfig = scrubbedConfig
	}

	p.platformAPIGatewayToAPIGateway(createAPIGatewayOptions.APIGatewayConfig, newAPIGateway)

	// set api gateway state to "waitingForProvisioning", so the controller will know to create/update this resource
	newAPIGateway.Status.State = platform.APIGatewayStateWaitingForProvisioning

	// create
	if _, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(newAPIGateway.Namespace).
		Create(ctx, newAPIGateway, metav1.CreateOptions{}); err != nil {
		return errors.Wrap(err, "Failed to create an API gateway")
	}

	return nil
}

// UpdateAPIGateway will update a previously existing api gateway
func (p *Platform) UpdateAPIGateway(ctx context.Context, updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) error {
	// get existing api gateway
	apiGateway, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Get(ctx, updateAPIGatewayOptions.APIGatewayConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get api gateway to update")
	}

	// restore existing config
	apiGatewayConfig := &platform.APIGatewayConfig{
		Meta: platform.APIGatewayMeta{
			Namespace:   apiGateway.Namespace,
			Name:        apiGateway.Name,
			Labels:      apiGateway.Labels,
			Annotations: apiGateway.Annotations,
		},
		Spec: apiGateway.Spec,
	}
	var restoredAPIGatewayConfig *platform.APIGatewayConfig
	if scrubbed, err := p.apiGatewayScrubber.HasScrubbedConfig(apiGatewayConfig, platform.GetAPIGatewaySensitiveField()); err == nil && scrubbed {
		if restoredAPIGatewayConfig, err = p.apiGatewayScrubber.RestoreAPIGatewayConfig(ctx, apiGatewayConfig); err != nil {
			return errors.Wrap(err, "Failed to scrub api gateway config")
		} else if err != nil {
			return errors.Wrap(err, "Failed to check if api gateway config is scrubbed")
		}
		apiGateway.Spec = restoredAPIGatewayConfig.Spec
		apiGateway.Labels = restoredAPIGatewayConfig.Meta.Labels
		apiGateway.Annotations = restoredAPIGatewayConfig.Meta.Annotations
	}

	// enrich
	p.enrichAPIGatewayConfig(ctx, updateAPIGatewayOptions.APIGatewayConfig, apiGateway)

	// validate
	if err := p.validateAPIGatewayConfig(ctx,
		updateAPIGatewayOptions.APIGatewayConfig,
		updateAPIGatewayOptions.ValidateFunctionsExistence,
		apiGateway); err != nil {
		return errors.Wrap(err, "Failed to validate api gateway")
	}
	// scrub api gateway config
	if p.GetConfig().SensitiveFields.MaskSensitiveFields {
		scrubbedConfig, err := p.apiGatewayScrubber.ScrubAPIGatewayConfig(ctx, updateAPIGatewayOptions.APIGatewayConfig)
		if err != nil {
			return errors.Wrap(err, "Failed to scrub api gateway config")
		}
		updateAPIGatewayOptions.APIGatewayConfig = scrubbedConfig
	}

	apiGateway.Annotations = updateAPIGatewayOptions.APIGatewayConfig.Meta.Annotations
	apiGateway.Labels = updateAPIGatewayOptions.APIGatewayConfig.Meta.Labels
	apiGateway.Spec = updateAPIGatewayOptions.APIGatewayConfig.Spec

	// set api gateway state to "waitingForProvisioning", so the controller will know to create/update this resource
	apiGateway.Status.State = platform.APIGatewayStateWaitingForProvisioning

	// update
	if _, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Update(ctx, apiGateway, metav1.UpdateOptions{}); err != nil {
		return errors.Wrap(err, "Failed to update an api gateway")
	}

	return nil
}

// DeleteAPIGateway will delete a previously existing api gateway
func (p *Platform) DeleteAPIGateway(ctx context.Context, deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) error {

	// validate
	if err := p.validateAPIGatewayMeta(&deleteAPIGatewayOptions.Meta); err != nil {
		return errors.Wrap(err, "Failed to validate an API gateway's metadata")
	}

	p.Logger.DebugWithCtx(ctx, "Deleting api gateway", "name", deleteAPIGatewayOptions.Meta.Name)

	// delete
	if err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(deleteAPIGatewayOptions.Meta.Namespace).
		Delete(ctx, deleteAPIGatewayOptions.Meta.Name, metav1.DeleteOptions{}); err != nil {

		return errors.Wrapf(err,
			"Failed to delete API gateway %s from namespace %s",
			deleteAPIGatewayOptions.Meta.Name,
			deleteAPIGatewayOptions.Meta.Namespace)
	}

	return nil
}

// GetAPIGateways will list existing api gateways
func (p *Platform) GetAPIGateways(ctx context.Context, getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) ([]platform.APIGateway, error) {
	var platformAPIGateways []platform.APIGateway
	var apiGateways []nuclioio.NuclioAPIGateway

	// if identifier specified, we need to get a single NuclioAPIGateway
	if getAPIGatewaysOptions.Name != "" {

		// get specific NuclioAPIGateway CR
		apiGateway, err := p.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioAPIGateways(getAPIGatewaysOptions.Namespace).
			Get(ctx, getAPIGatewaysOptions.Name, metav1.GetOptions{})
		if err != nil {

			// if we didn't find the NuclioAPIGateway, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformAPIGateways, nil
			}

			return nil, errors.Wrap(err, "Failed to get an API gateway")
		}

		apiGateways = append(apiGateways, *apiGateway)

	} else {

		apiGatewayInstanceList, err := p.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioAPIGateways(getAPIGatewaysOptions.Namespace).
			List(ctx, metav1.ListOptions{LabelSelector: getAPIGatewaysOptions.Labels})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to list API gateways")
		}

		apiGateways = apiGatewayInstanceList.Items
	}

	// convert []nuclioio.NuclioAPIGateway -> NuclioAPIGateway
	for apiGatewayInstanceIndex := 0; apiGatewayInstanceIndex < len(apiGateways); apiGatewayInstanceIndex++ {
		apiGatewayInstance := apiGateways[apiGatewayInstanceIndex]

		newAPIGateway, err := platform.NewAbstractAPIGateway(p.Logger,
			p,
			platform.APIGatewayConfig{
				Meta: platform.APIGatewayMeta{
					Name:              apiGatewayInstance.Name,
					Namespace:         apiGatewayInstance.Namespace,
					Labels:            apiGatewayInstance.Labels,
					Annotations:       apiGatewayInstance.Annotations,
					CreationTimestamp: &apiGatewayInstance.CreationTimestamp,
				},
				Spec:   apiGatewayInstance.Spec,
				Status: apiGatewayInstance.Status,
			})
		if err != nil {
			return nil, err
		}

		platformAPIGateways = append(platformAPIGateways, newAPIGateway)
	}

	// render it
	return platformAPIGateways, nil
}

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (p *Platform) CreateFunctionEvent(ctx context.Context, createFunctionEventOptions *platform.CreateFunctionEventOptions) error {

	if err := p.Platform.EnrichFunctionEvent(ctx, &createFunctionEventOptions.FunctionEventConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich function event")
	}

	newFunctionEvent := nuclioio.NuclioFunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&createFunctionEventOptions.FunctionEventConfig, &newFunctionEvent)

	projectName := newFunctionEvent.Labels[common.NuclioResourceLabelKeyProjectName]
	functionName := newFunctionEvent.Labels[common.NuclioResourceLabelKeyFunctionName]

	// Check OPA permissions
	permissionOptions := createFunctionEventOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionEventPermissions(projectName,
		functionName,
		newFunctionEvent.Name,
		opa.ActionCreate,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	if _, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(createFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Create(ctx, &newFunctionEvent, metav1.CreateOptions{}); err != nil {
		return errors.Wrap(err, "Failed to create a function event")
	}
	return nil
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(ctx context.Context, updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	updatedFunctionEvent := nuclioio.NuclioFunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig, &updatedFunctionEvent)

	functionEvent, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Get(ctx, updateFunctionEventOptions.FunctionEventConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get a function event")
	}

	if err := p.Platform.EnrichFunctionEvent(ctx, &updateFunctionEventOptions.FunctionEventConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich function event")
	}

	functionName := functionEvent.Labels[common.NuclioResourceLabelKeyFunctionName]
	projectName := functionEvent.Labels[common.NuclioResourceLabelKeyProjectName]

	// Check OPA permissions
	permissionOptions := updateFunctionEventOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionEventPermissions(projectName,
		functionName,
		functionEvent.Name,
		opa.ActionUpdate,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	functionEvent.Spec = updatedFunctionEvent.Spec
	functionEvent.Annotations = updatedFunctionEvent.Annotations
	functionEvent.Labels = updatedFunctionEvent.Labels

	if _, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Update(ctx, functionEvent, metav1.UpdateOptions{}); err != nil {
		return errors.Wrap(err, "Failed to update a function event")
	}

	return nil
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(ctx context.Context, deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	functionEventToDelete, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(deleteFunctionEventOptions.Meta.Namespace).
		Get(ctx, deleteFunctionEventOptions.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get a function event")
	}

	functionName := functionEventToDelete.Labels[common.NuclioResourceLabelKeyFunctionName]
	projectName := functionEventToDelete.Labels[common.NuclioResourceLabelKeyProjectName]

	// Check OPA permissions
	permissionOptions := deleteFunctionEventOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionEventPermissions(projectName,
		functionName,
		functionEventToDelete.Name,
		opa.ActionDelete,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	if err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(deleteFunctionEventOptions.Meta.Namespace).
		Delete(ctx, deleteFunctionEventOptions.Meta.Name, metav1.DeleteOptions{}); err != nil {
		return errors.Wrapf(err,
			"Failed to delete function event %s from namespace %s",
			deleteFunctionEventOptions.Meta.Name,
			deleteFunctionEventOptions.Meta.Namespace)
	}

	return nil
}

// GetFunctionEvents will list existing function events
func (p *Platform) GetFunctionEvents(ctx context.Context, getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	var platformFunctionEvents []platform.FunctionEvent
	var functionEvents []nuclioio.NuclioFunctionEvent

	// if identifier specified, we need to get a single function event
	if getFunctionEventsOptions.Meta.Name != "" {

		// get specific function event CR
		functionEvent, err := p.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioFunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			Get(ctx, getFunctionEventsOptions.Meta.Name, metav1.GetOptions{})

		if err != nil {

			// if we didn't find the function event, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformFunctionEvents, nil
			}

			return nil, errors.Wrap(err, "Failed to get a function event")
		}

		functionEvents = append(functionEvents, *functionEvent)

	} else {
		var labelSelector string
		functionName := getFunctionEventsOptions.Meta.Labels[common.NuclioResourceLabelKeyFunctionName]

		// if function name specified, supply it
		if functionName != "" {
			labelSelector = fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyFunctionName, functionName)
		} else if len(getFunctionEventsOptions.FunctionNames) > 0 {
			encodedFunctionNames := strings.Join(getFunctionEventsOptions.FunctionNames, ",")
			labelSelector = fmt.Sprintf("%s in (%s)",
				common.NuclioResourceLabelKeyFunctionName,
				encodedFunctionNames)
		}

		functionEventInstanceList, err := p.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioFunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			List(ctx, metav1.ListOptions{LabelSelector: labelSelector})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list function events")
		}

		// convert []NuclioFunctionEvent to []*NuclioFunctionEvent
		functionEvents = functionEventInstanceList.Items
	}

	// convert []nuclioio.NuclioFunctionEvent -> []platform.FunctionEvent
	for functionEventInstanceIndex := 0; functionEventInstanceIndex < len(functionEvents); functionEventInstanceIndex++ {
		functionEventInstance := functionEvents[functionEventInstanceIndex]
		newFunctionEvent, err := platform.NewAbstractFunctionEvent(p.Logger,
			p,
			platform.FunctionEventConfig{
				Meta: platform.FunctionEventMeta{
					Name:        functionEventInstance.Name,
					Namespace:   functionEventInstance.Namespace,
					Labels:      functionEventInstance.Labels,
					Annotations: functionEventInstance.Annotations,
				},
				Spec: functionEventInstance.Spec,
			})
		if err != nil {
			return nil, err
		}
		platformFunctionEvents = append(platformFunctionEvents, newFunctionEvent)
	}

	return p.Platform.FilterFunctionEventsByPermissions(ctx,
		&getFunctionEventsOptions.PermissionOptions,
		platformFunctionEvents)
}

// GetExternalIPAddresses returns the external IP addresses invocations will use.
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (p *Platform) GetExternalIPAddresses() ([]string, error) {

	// check if parent has addresses
	externalIPAddress, err := p.Platform.GetExternalIPAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get external IP addresses from parent")
	}

	// if the parent has something, use that
	if len(externalIPAddress) != 0 {
		return externalIPAddress, nil
	}

	// try to take from kube host as configured
	kubeURL, err := url.Parse(p.consumer.KubeHost)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse kube host")
	}

	if kubeURL.Host != "" {
		return []string{
			strings.Split(kubeURL.Host, ":")[0],
		}, nil
	}

	// return an empty string to maintain backwards compatibility
	return []string{""}, nil
}

// GetNamespaces returns all the namespaces in the platform
func (p *Platform) GetNamespaces(ctx context.Context) ([]string, error) {
	if len(p.Config.ManagedNamespaces) > 0 {
		return p.Config.ManagedNamespaces, nil
	}

	namespaces, err := p.consumer.KubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {

			// if we're not allowed to list namespaces (e.g.: when nuclio is namespaced), return our
			// default namespace (aka the namespace we're running in)
			return []string{common.ResolveDefaultNamespace(p.DefaultNamespace)}, nil
		}
		return nil, errors.Wrap(err, "Failed to list namespaces")
	}

	var namespaceNames []string

	// put default namespace first in namespace list
	namespaceNames = append(namespaceNames, p.DefaultNamespace)

	for _, namespace := range namespaces.Items {
		if namespace.Name != p.DefaultNamespace {
			namespaceNames = append(namespaceNames, namespace.Name)
		}
	}

	return namespaceNames, nil
}

func (p *Platform) GetFunctionScrubber() *functionconfig.Scrubber {
	if p.FunctionScrubber == nil {
		p.FunctionScrubber = functionconfig.NewScrubber(p.Logger,
			p.GetConfig().SensitiveFields.CompileSensitiveFieldsRegex(),
			p.consumer.KubeClientSet,
		)
		return p.FunctionScrubber
	}
	if p.FunctionScrubber.KubeClientSet == nil {
		p.FunctionScrubber.KubeClientSet = p.consumer.KubeClientSet
	}
	return p.FunctionScrubber
}

func (p *Platform) GetAPIGatewayScrubber() *platform.APIGatewayScrubber {
	if p.apiGatewayScrubber == nil {
		p.apiGatewayScrubber = platform.NewAPIGatewayScrubber(p.Logger, platform.GetAPIGatewaySensitiveField(),
			p.consumer.KubeClientSet)
		return p.apiGatewayScrubber
	}
	if p.apiGatewayScrubber.KubeClientSet == nil {
		p.apiGatewayScrubber.KubeClientSet = p.consumer.KubeClientSet
	}
	return p.apiGatewayScrubber
}

func (p *Platform) GetDefaultInvokeIPAddresses() ([]string, error) {
	return []string{}, nil
}

func (p *Platform) GetScaleToZeroConfiguration() *platformconfig.ScaleToZero {
	return &p.Config.ScaleToZero
}

func (p *Platform) GetAllowedAuthenticationModes() []string {
	allowedAuthenticationModes := []string{string(ingress.AuthenticationModeNone), string(ingress.AuthenticationModeBasicAuth)}
	if len(p.Config.IngressConfig.AllowedAuthenticationModes) > 0 {
		allowedAuthenticationModes = p.Config.IngressConfig.AllowedAuthenticationModes
	}
	return allowedAuthenticationModes
}

func (p *Platform) SaveFunctionDeployLogs(ctx context.Context, functionName, namespace string) error {
	functions, err := p.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Name:      functionName,
		Namespace: namespace,
	})
	if err != nil || len(functions) == 0 {
		return errors.Wrap(err, "Failed to get existing functions")
	}

	// enrich with build logs
	p.EnrichFunctionsWithDeployLogStream(functions)

	function := functions[0]

	return p.updater.Update(ctx, &platform.UpdateFunctionOptions{
		FunctionMeta:   &function.GetConfig().Meta,
		FunctionStatus: function.GetStatus(),
	})
}

func (p *Platform) ValidateFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config) error {
	if err := p.Platform.ValidateFunctionConfig(ctx, functionConfig); err != nil {
		return err
	}

	if err := p.validateCronTriggers(functionConfig); err != nil {
		return errors.Wrap(err, "Cron triggers validation failed")
	}

	if err := p.validateServiceType(functionConfig); err != nil {
		return errors.Wrap(err, "Service type validation failed")
	}

	if err := p.validateInitContainersSpec(functionConfig); err != nil {
		return errors.Wrap(err, "Init containers validation failed")
	}
	if err := p.validateSidecarSpec(functionConfig); err != nil {
		return errors.Wrap(err, "Sidecar validation failed")
	}

	return p.validateFunctionIngresses(ctx, functionConfig)
}

// InitializeContainerBuilder initializes the container builder, if not already initialized
func (p *Platform) InitializeContainerBuilder() error {

	// if container builder is already initialized, return
	if p.ContainerBuilder != nil {
		return nil
	}

	var err error

	containerBuilderConfiguration := p.GetConfig().ContainerBuilderConfiguration

	// create container builder
	if containerBuilderConfiguration.Kind == "kaniko" {
		p.ContainerBuilder, err = containerimagebuilderpusher.NewKaniko(p.Logger,
			p.consumer.KubeClientSet, containerBuilderConfiguration)
		if err != nil {
			return errors.Wrap(err, "Failed to create a kaniko builder")
		}
	} else {

		// Default container image builder
		p.ContainerBuilder, err = containerimagebuilderpusher.NewDocker(p.Logger,
			containerBuilderConfiguration)
		if err != nil {
			return errors.Wrap(err, "Failed to create a Docker builder")
		}
	}

	return nil
}

func (p *Platform) generateFunctionToAPIGatewaysMapping(ctx context.Context, namespace string) (map[string][]string, error) {
	functionToAPIGateways := map[string][]string{}

	// get all api gateways in the namespace
	apiGateways, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list API gateways")
	}

	// iterate over all api gateways
	for _, apiGateway := range apiGateways.Items {

		// iterate over all upstreams(functions) of the api gateway
		for _, upstream := range apiGateway.Spec.Upstreams {

			// append the current api gateway to the function's api gateways list
			functionToAPIGateways[upstream.NuclioFunction.Name] =
				append(functionToAPIGateways[upstream.NuclioFunction.Name], apiGateway.Name)
		}
	}

	return functionToAPIGateways, nil
}

func (p *Platform) getApiGatewaysForFunction(ctx context.Context,
	namespace string,
	functionName string) ([]nuclioio.NuclioAPIGateway, error) {
	var functionsApiGateways []nuclioio.NuclioAPIGateway

	apiGateways, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list API gateways")
	}
	for _, apiGateway := range apiGateways.Items {
		for _, upstream := range apiGateway.Spec.Upstreams {

			if upstream.NuclioFunction.Name == functionName {
				functionsApiGateways = append(functionsApiGateways, apiGateway)
				break
			}
		}
	}
	return functionsApiGateways, nil
}

func (p *Platform) enrichFunctionsWithAPIGateways(ctx context.Context, functions []platform.Function, namespace string) error {
	var err error
	var functionToAPIGateways map[string][]string

	// no functions to enrich
	if len(functions) == 0 {
		return nil
	}

	// generate function to api gateways mapping
	if functionToAPIGateways, err = p.generateFunctionToAPIGatewaysMapping(ctx, namespace); err != nil {
		return errors.Wrap(err, "Failed to get a function to API-gateways mapping")
	}

	// set the api gateways list for every function according to the mapping above
	for _, function := range functions {
		function.GetStatus().APIGateways = functionToAPIGateways[function.GetConfig().Meta.Name]
	}

	return nil
}

// enrichFunctionPreemptionSpec - Enriches function pod with the below described spec. if no platformConfiguration related
// configuration is given, do nothing.
//
//		`Allow` 	- Adds Tolerations / GPU Tolerations if taints were given. otherwise, assume pods can be scheduled on preemptible nodes.
//	               > Purges any `affinity` / `anti-affinity` preemption related configuration
//		`Constrain` - Uses node-affinity to make sure pods are assigned using OR on the given node label selectors.
//	               > Uses `Allow` configuration as well.
//	               > Purges any `anti-affinity` preemption related configuration
//		`Prevent`	- Prevention is done either using taints (if Tolerations were given) or anti-affinity.
//	               > Purges any `tolerations` / `gpuTolerations` preemption related configuration
//	               > Purges any `affinity` preemption related configuration
//	               > Adds anti-affinity IF no tolerations were given
func (p *Platform) enrichFunctionPreemptionSpec(ctx context.Context,
	preemptibleNodes *platformconfig.PreemptibleNodes,
	functionConfig *functionconfig.Config) {

	// nothing to do here, configuration is not populated
	if p.Config.Kube.PreemptibleNodes == nil {
		return
	}

	// we do such stuff to allow exposing features before they are exposed on UI
	if preemptionMode, exists := functionConfig.Meta.Annotations["nuclio.io/preemptible-mode"]; exists {
		p.Logger.DebugWithCtx(ctx,
			"Enriching function preemption mode from function annotations",
			"preemptionMode", preemptionMode)
		functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodeMode(preemptionMode)
	}

	// no preemption mode was selected, default to prevent
	if functionConfig.Spec.PreemptionMode == "" {
		functionConfig.Spec.PreemptionMode = p.Config.Kube.PreemptibleNodes.DefaultMode
		p.Logger.DebugWithCtx(ctx,
			"No preemption mode was given, using the default",
			"newPreemptionMode", functionConfig.Spec.PreemptionMode)
	}

	p.Logger.DebugWithCtx(ctx,
		"Enriching function spec for given preemption mode",
		"functionName", functionConfig.Meta.Name,
		"preemptionMode", functionConfig.Spec.PreemptionMode)

	switch functionConfig.Spec.PreemptionMode {
	case functionconfig.RunOnPreemptibleNodesNone:

		// do nothing
		break

	case functionconfig.RunOnPreemptibleNodesPrevent:

		// ensure no preemptible node tolerations
		functionConfig.PruneTolerations(preemptibleNodes.Tolerations)
		functionConfig.PruneTolerations(preemptibleNodes.GPUTolerations)

		// ensure no preemptible node selector
		functionConfig.PruneNodeSelector(preemptibleNodes.NodeSelector)

		// if tolerations were given, purge `affinity` preemption related configuration
		if preemptibleNodes.Tolerations != nil {
			functionConfig.
				PruneAffinityNodeSelectorRequirement(
					preemptibleNodes.CompileAffinityByLabelSelector(v1.NodeSelectorOpIn), "oneOf")

			// prevention is done by tolerations (as spec was explicitly given)
			// and thus, stop here
			break
		}

		// initial affinity
		if functionConfig.Spec.Affinity == nil {
			functionConfig.Spec.Affinity = &v1.Affinity{}
		}

		// initial node affinity
		if functionConfig.Spec.Affinity.NodeAffinity == nil {
			functionConfig.Spec.Affinity.NodeAffinity = &v1.NodeAffinity{}
		}

		// using a single term with potentially multiple expressions to ensure affinity.
		// when having multiple terms, pod scheduling is succeeded if at least one
		// term is satisfied.
		functionConfig.
			Spec.
			Affinity.
			NodeAffinity.
			RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{
			NodeSelectorTerms: preemptibleNodes.CompileAntiAffinityByLabelSelectorNoScheduleOnMatchingNodes(),
		}

	case functionconfig.RunOnPreemptibleNodesConstrain:

		// remove anti affinity even thought we assign affinity below
		// which will override the same fields.
		// doing it here for future proofing
		functionConfig.
			PruneAffinityNodeSelectorRequirement(
				preemptibleNodes.CompileAffinityByLabelSelector(v1.NodeSelectorOpNotIn), "matchAll")

		// enrich with tolerations
		functionConfig.EnrichWithTolerations(preemptibleNodes.Tolerations)

		// in case function pod requires gpu resource(s), enrich with gpu tolerations
		if functionConfig.Spec.PositiveGPUResourceLimit() {
			functionConfig.EnrichWithTolerations(preemptibleNodes.GPUTolerations)
		}

		// initial affinity
		if functionConfig.Spec.Affinity == nil {
			functionConfig.Spec.Affinity = &v1.Affinity{}
		}

		// initial node affinity
		if functionConfig.Spec.Affinity.NodeAffinity == nil {
			functionConfig.Spec.Affinity.NodeAffinity = &v1.NodeAffinity{}
		}

		// using a single term with potentially multiple expressions to ensure affinity.
		// when having multiple terms, pod scheduling is succeeded if at least one
		// term is satisfied.
		functionConfig.
			Spec.
			Affinity.
			NodeAffinity.
			RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{
			NodeSelectorTerms: preemptibleNodes.CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes(),
		}

	case functionconfig.RunOnPreemptibleNodesAllow:

		// purges any `affinity` / `anti-affinity` preemption related configuration
		// remove anti-affinity
		functionConfig.
			PruneAffinityNodeSelectorRequirement(
				preemptibleNodes.CompileAffinityByLabelSelector(v1.NodeSelectorOpNotIn), "matchAll")

		// remove affinity
		functionConfig.
			PruneAffinityNodeSelectorRequirement(
				preemptibleNodes.CompileAffinityByLabelSelector(v1.NodeSelectorOpIn), "oneOf")

		// remove preemptible nodes constrain
		functionConfig.PruneNodeSelector(preemptibleNodes.NodeSelector)

		// enrich with tolerations
		functionConfig.EnrichWithTolerations(preemptibleNodes.Tolerations)

		// in case function pod requires gpu resource(s), enrich with gpu tolerations
		if functionConfig.Spec.PositiveGPUResourceLimit() {
			functionConfig.EnrichWithTolerations(preemptibleNodes.GPUTolerations)
		}

	default:

		// nothing to do here
		break
	}
}

func (p *Platform) enrichInitContainersSpec(functionConfig *functionconfig.Config) {
	for _, initContainer := range functionConfig.Spec.InitContainers {
		p.enrichContainerSpec(initContainer, functionConfig)
	}
}

func (p *Platform) enrichSidecarsSpec(functionConfig *functionconfig.Config) {
	for _, sidecar := range functionConfig.Spec.Sidecars {
		p.enrichContainerSpec(sidecar, functionConfig)
	}
}

func (p *Platform) enrichContainerSpec(container *v1.Container, functionConfig *functionconfig.Config) {
	// enrich env vars
	if container.Env == nil {
		container.Env = make([]v1.EnvVar, 0)
	}
	container.Env = common.MergeEnvSlices(container.Env, functionConfig.Spec.Env)

	// image pull policy
	if container.ImagePullPolicy == "" {
		container.ImagePullPolicy = functionConfig.Spec.ImagePullPolicy
	}
}

func (p *Platform) clearCallStack(message string) string {
	if message == "" {
		return ""
	}

	splitMessage := strings.Split(message, "\nCall stack:\n")
	return splitMessage[0]
}

func (p *Platform) setScaleToZeroSpec(functionSpec *functionconfig.Spec) error {

	// If function already has scale to zero spec, don't override it
	if functionSpec.ScaleToZero != nil {
		return nil
	}

	if p.Config.ScaleToZero.Mode == platformconfig.EnabledScaleToZeroMode {
		functionSpec.ScaleToZero = &functionconfig.ScaleToZeroSpec{
			ScaleResources: p.Config.ScaleToZero.ScaleResources,
		}
	}

	return nil
}

func (p *Platform) getFunction(ctx context.Context,
	getFunctionOptions *platform.GetFunctionsOptions) (*nuclioio.NuclioFunction, error) {
	p.Logger.DebugWithCtx(ctx, "Getting function",
		"namespace", getFunctionOptions.Namespace,
		"name", getFunctionOptions.Name)

	// get specific function CR
	function, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctions(getFunctionOptions.Namespace).
		Get(ctx,
			getFunctionOptions.Name,
			metav1.GetOptions{
				ResourceVersion: getFunctionOptions.ResourceVersion,
			})
	if err != nil {

		// if we didn't find the function, return nothing
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "Failed to get a function")
	}

	p.Logger.DebugWithCtx(ctx, "Completed getting function",
		"name", getFunctionOptions.Name,
		"namespace", getFunctionOptions.Namespace,
		"function", function)

	return function, nil
}

func (p *Platform) getFunctionInstanceAndConfig(ctx context.Context,
	getFunctionOptions *platform.GetFunctionsOptions) (*nuclioio.NuclioFunction, *functionconfig.ConfigWithStatus, error) {
	functionInstance, err := p.getFunction(ctx, getFunctionOptions)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get a function")
	}

	// found function instance, return as function config
	if functionInstance != nil {
		initializedFunctionInstance, err := client.NewFunction(p.Logger, p, functionInstance, p.consumer)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to create a new function instance")
		}
		if getFunctionOptions.EnrichWithAPIGateways {
			if err := p.enrichFunctionsWithAPIGateways(ctx,
				[]platform.Function{initializedFunctionInstance},
				getFunctionOptions.Namespace); err != nil {
				return nil, nil, errors.Wrap(err, "Failed to enrich function with api gateway")
			}
			functionInstance.Status.APIGateways = initializedFunctionInstance.GetConfigWithStatus().Status.APIGateways
		}
		return functionInstance, initializedFunctionInstance.GetConfigWithStatus(), nil
	}
	return nil, nil, nil
}

func (p *Platform) platformAPIGatewayToAPIGateway(platformAPIGateway *platform.APIGatewayConfig, apiGateway *nuclioio.NuclioAPIGateway) {
	apiGateway.Name = platformAPIGateway.Meta.Name
	apiGateway.Namespace = platformAPIGateway.Meta.Namespace
	apiGateway.Labels = platformAPIGateway.Meta.Labels
	apiGateway.Annotations = platformAPIGateway.Meta.Annotations
	apiGateway.Spec = platformAPIGateway.Spec
	apiGateway.Status = platformAPIGateway.Status
}

func (p *Platform) platformFunctionEventToFunctionEvent(platformFunctionEvent *platform.FunctionEventConfig, functionEvent *nuclioio.NuclioFunctionEvent) {
	functionEvent.Name = platformFunctionEvent.Meta.Name
	functionEvent.Namespace = platformFunctionEvent.Meta.Namespace
	functionEvent.Labels = platformFunctionEvent.Meta.Labels
	functionEvent.Annotations = platformFunctionEvent.Meta.Annotations
	functionEvent.Spec = platformFunctionEvent.Spec // deep copy instead?
}

func (p *Platform) enrichAPIGatewayConfig(ctx context.Context,
	apiGatewayConfig *platform.APIGatewayConfig,
	existingApiGatewayConfig *nuclioio.NuclioAPIGateway) {

	// meta
	if apiGatewayConfig.Meta.Name == "" {
		apiGatewayConfig.Meta.Name = apiGatewayConfig.Spec.Name
	}

	// spec
	if apiGatewayConfig.Spec.Name == "" {
		apiGatewayConfig.Spec.Name = apiGatewayConfig.Meta.Name
	}

	if apiGatewayConfig.Meta.Labels == nil {
		apiGatewayConfig.Meta.Labels = map[string]string{}
	}

	if apiGatewayConfig.Spec.Host == "" {
		templateData := map[string]interface{}{
			"Name":         apiGatewayConfig.Meta.Name,
			"ResourceName": apiGatewayConfig.Meta.Name,
			"Namespace":    apiGatewayConfig.Meta.Namespace,
			"ProjectName":  apiGatewayConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		}
		if apiGatewayHost, err := p.renderIngressHost(ctx, common.DefaultIngressHostTemplate, templateData, 8); err == nil {
			apiGatewayConfig.Spec.Host = apiGatewayHost
		}
	}

	// enrich project name if not exists or value is empty
	if existingApiGatewayConfig != nil {
		if value, exist := apiGatewayConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]; value == "" || !exist {
			apiGatewayConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName] =
				existingApiGatewayConfig.Labels[common.NuclioResourceLabelKeyProjectName]
		}
	}

	p.EnrichLabels(ctx, apiGatewayConfig.Meta.Labels)
}

func (p *Platform) validateAPIGatewayMeta(platformAPIGatewayMeta *platform.APIGatewayMeta) error {
	if platformAPIGatewayMeta.Name == "" {
		return nuclio.NewErrBadRequest("Api gateway name must be provided in metadata")
	}

	if platformAPIGatewayMeta.Namespace == "" {
		return nuclio.NewErrBadRequest("Api gateway namespace must be provided in metadata")
	}

	return nil
}

func (p *Platform) validateAPIGatewayConfig(ctx context.Context,
	apiGateway *platform.APIGatewayConfig,
	validateFunctionsExistence bool,
	existingAPIGateway *nuclioio.NuclioAPIGateway) error {

	// general validations
	if apiGateway.Spec.Name != apiGateway.Meta.Name {
		return nuclio.NewErrBadRequest("Api gateway metadata.name must match api gateway spec.name")
	}

	// not a reserved name
	if common.StringInSlice(apiGateway.Spec.Name, p.ResolveReservedResourceNames()) {
		return nuclio.NewErrPreconditionFailed(fmt.Sprintf("Api gateway name '%s' is reserved and cannot be used",
			apiGateway.Spec.Name))
	}

	// meta
	if err := p.validateAPIGatewayMeta(&apiGateway.Meta); err != nil {
		return errors.Wrap(err, "Failed to validate API-gateway metadata")
	}

	// spec
	if err := ValidateAPIGatewaySpec(&apiGateway.Spec); err != nil {
		return errors.Wrap(err, "Failed to validate the API-gateway spec")
	}

	if existingAPIGateway != nil {
		if existingAPIGateway.Labels[common.NuclioResourceLabelKeyProjectName] !=
			apiGateway.Meta.Labels[common.NuclioResourceLabelKeyProjectName] {
			return nuclio.NewErrBadRequest("Changing project name to an existing api gateway is not allowed")
		}
	}

	// get upstream functions for validating functions existence
	if _, err := p.getAPIGatewayUpstreamFunctions(ctx, apiGateway, validateFunctionsExistence); err != nil {
		return errors.Wrap(err, "Failed to get api gateway upstream functions")
	}

	// ingresses
	if err := p.validateAPIGatewayIngresses(ctx, apiGateway); err != nil {
		return errors.Wrap(err, "Failed to validate ingresses")
	}

	return nil
}

func (p *Platform) enrichAndValidateFunctionConfig(ctx context.Context, functionConfig *functionconfig.Config, autofix bool) error {
	if err := p.EnrichFunctionConfig(ctx, functionConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich a function configuration")
	}
	return p.Platform.ValidateFunctionConfigWithRetry(ctx, functionConfig, autofix)
}

// enrichFunctionNodeSelector enriches function node selector
// if node selector is not specified in function config, we firstly try to get it from project CRD
// if it is missing in project CRD, then we try to get it from platform config
func (p *Platform) enrichFunctionNodeSelector(ctx context.Context, functionConfig *functionconfig.Config) error {
	functionProject, err := p.Platform.GetFunctionProject(ctx, functionConfig)

	if functionConfig.Spec.NodeSelector == nil {
		if functionProject.GetConfig().Spec.DefaultFunctionNodeSelector == nil &&
			p.Config.Kube.DefaultFunctionNodeSelector == nil {
			return nil
		}
		functionConfig.Spec.NodeSelector = make(map[string]string)
	}
	if err != nil {
		return errors.Wrap(err, "Failed to get function project")
	}

	var defaultNodeSelector map[string]string

	if p.Config.Kube.IgnorePlatformIfProjectNodeSelectors {
		if functionProject.GetConfig().Spec.DefaultFunctionNodeSelector != nil {
			p.Logger.DebugWithCtx(ctx,
				"Enriching function node selector from project",
				"functionName", functionConfig.Meta.Name,
				"nodeSelector", p.Config.Kube.DefaultFunctionNodeSelector)
			defaultNodeSelector = functionProject.GetConfig().Spec.DefaultFunctionNodeSelector
		} else {
			p.Logger.DebugWithCtx(ctx,
				"Enriching function node selector from platform config",
				"functionName", functionConfig.Meta.Name,
				"nodeSelector", p.Config.Kube.DefaultFunctionNodeSelector)
			defaultNodeSelector = p.Config.Kube.DefaultFunctionNodeSelector
		}
	} else {
		p.Logger.DebugWithCtx(ctx,
			"Enriching function node selector from platform config and project",
			"functionName", functionConfig.Meta.Name,
			"platformNodeSelector", p.Config.Kube.DefaultFunctionNodeSelector,
			"projectNodeSelector", functionProject.GetConfig().Spec.DefaultFunctionNodeSelector)
		defaultNodeSelector = labels.Merge(p.Config.Kube.DefaultFunctionNodeSelector, functionProject.GetConfig().Spec.DefaultFunctionNodeSelector)
	}

	functionConfig.Spec.NodeSelector = labels.Merge(defaultNodeSelector, functionConfig.Spec.NodeSelector)
	return nil
}

func (p *Platform) validateServiceType(functionConfig *functionconfig.Config) error {
	serviceType := functionconfig.ResolveFunctionServiceType(&functionConfig.Spec, p.Config.Kube.DefaultServiceType)
	switch serviceType {
	case "":

		// empty means - let it be enriched by default
		return nil
	case v1.ServiceTypeNodePort, v1.ServiceTypeClusterIP:
		return nil
	default:
		return nuclio.NewErrBadRequest(fmt.Sprintf("Unsupported service type %s", serviceType))
	}
}

func (p *Platform) validateCronTriggers(functionConfig *functionconfig.Config) error {
	if functionConfig.Spec.DisableDefaultHTTPTrigger != nil && *functionConfig.Spec.DisableDefaultHTTPTrigger &&
		len(functionconfig.GetTriggersByKind(functionConfig.Spec.Triggers, "cron")) > 0 &&
		len(functionconfig.GetTriggersByKind(functionConfig.Spec.Triggers, "http")) == 0 &&
		p.Config.CronTriggerCreationMode == platformconfig.KubeCronTriggerCreationMode {
		return errors.New("Cron trigger in `kube` mode cannot be created when default http trigger " +
			"creation is disabled and there is no other http trigger. " +
			"Either enable default http trigger creation or create custom http trigger")
	}
	return nil
}

func (p *Platform) enrichHTTPTriggers(ctx context.Context, functionConfig *functionconfig.Config) error {

	serviceType := functionconfig.ResolveFunctionServiceType(&functionConfig.Spec, p.Config.Kube.DefaultServiceType)

	for triggerName, trigger := range functionconfig.GetTriggersByKind(functionConfig.Spec.Triggers, "http") {
		p.enrichTriggerWithServiceType(ctx, functionConfig, &trigger, serviceType)
		if err := p.enrichHTTPTriggerIngresses(ctx, &trigger, functionConfig); err != nil {
			return errors.Wrap(err, "Failed to enrich HTTP trigger ingresses")
		}
		functionConfig.Spec.Triggers[triggerName] = trigger
	}

	return nil
}

func (p *Platform) validateFunctionHasNoAPIGateways(ctx context.Context, deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	var functionToAPIGateways map[string][]string
	var err error

	// generate function to api gateways mapping
	if functionToAPIGateways, err = p.generateFunctionToAPIGatewaysMapping(ctx, deleteFunctionOptions.FunctionConfig.Meta.Namespace); err != nil {
		return errors.Wrap(err, "Failed to get a function to API-gateways mapping")
	}

	if len(functionToAPIGateways[deleteFunctionOptions.FunctionConfig.Meta.Name]) > 0 {
		return platform.ErrFunctionIsUsedByAPIGateways
	}

	return nil
}

func (p *Platform) enrichTriggerWithServiceType(ctx context.Context,
	functionConfig *functionconfig.Config,
	trigger *functionconfig.Trigger,
	serviceType v1.ServiceType) {

	if trigger.Attributes == nil {
		trigger.Attributes = map[string]interface{}{}
	}

	if triggerServiceType, serviceTypeExists := trigger.Attributes["serviceType"]; !serviceTypeExists ||
		triggerServiceType == "" ||
		triggerServiceType == nil {

		p.Logger.DebugWithCtx(ctx, "Enriching function HTTP trigger with service type",
			"functionName", functionConfig.Meta.Name,
			"triggerName", trigger.Name,
			"serviceType", serviceType)
		trigger.Attributes["serviceType"] = serviceType
	}
}

func (p *Platform) validateAPIGatewayIngresses(ctx context.Context, apiGatewayConfig *platform.APIGatewayConfig) error {

	// create a map to be used for ingress host and path-availability validation
	apiGatewayIngresses := map[string]functionconfig.Ingress{
		"agw-ingress": {
			Host:  apiGatewayConfig.Spec.Host,
			Paths: []string{apiGatewayConfig.Spec.Path},
		},
	}

	ingressName := IngressNameFromAPIGatewayName(apiGatewayConfig.Meta.Name, false)
	ingressNameWithCanary := IngressNameFromAPIGatewayName(apiGatewayConfig.Meta.Name, true)
	listIngressesOptions := metav1.ListOptions{

		// validate ingresses not created by this api gateway (whether it has canary deployment or not)
		FieldSelector: fmt.Sprintf("metadata.name!=%s,metadata.name!=%s", ingressName, ingressNameWithCanary),
	}

	if err := p.validateIngressHostAndPathAvailability(ctx,
		listIngressesOptions,
		apiGatewayConfig.Meta.Namespace,
		apiGatewayIngresses); err != nil {
		return errors.Wrap(err, "Failed to validate the API-gateway host and path availability")
	}

	return nil
}

func (p *Platform) validateSidecarSpec(functionConfig *functionconfig.Config) error {
	for _, sidecar := range functionConfig.Spec.Sidecars {
		if err := p.validateContainerSpec(sidecar); err != nil {
			return nuclio.WrapErrBadRequest(err)
		}

		if err := p.validateContainerPorts(sidecar); err != nil {
			return nuclio.WrapErrBadRequest(err)
		}
	}

	return nil
}

func (p *Platform) validateInitContainersSpec(functionConfig *functionconfig.Config) error {
	for _, initContainer := range functionConfig.Spec.InitContainers {
		if err := p.validateContainerSpec(initContainer); err != nil {
			return nuclio.WrapErrBadRequest(err)
		}
	}

	return nil
}

func (p *Platform) validateContainerSpec(container *v1.Container) error {
	if container.Name == "" {
		return nuclio.NewErrBadRequest("Container name must be provided")
	}

	if container.Image == "" {
		return nuclio.NewErrBadRequest(fmt.Sprintf("Container image must be provided for container %s", container.Name))
	}

	return nil
}

func (p *Platform) validateContainerPorts(container *v1.Container) error {
	if container.Ports != nil {
		portNames := make(map[string]bool)
		portNumbers := make(map[int32]bool)

		for _, port := range container.Ports {
			// validate container port exists
			if port.ContainerPort == 0 {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Container port must be provided for container %s", container.Name))
			}

			// validate container port is not reserved
			if lo.Contains[int]([]int{
				abstract.FunctionContainerHTTPPort,
				abstract.FunctionContainerWebAdminHTTPPort,
				abstract.FunctionContainerHealthCheckHTTPPort,
				abstract.FunctionContainerMetricPort,
			}, int(port.ContainerPort)) {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Container port %d is reserved for Nuclio internal use", port.ContainerPort))
			}

			// validate port name exists
			if port.Name == "" {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Port name must be provided for container %s", container.Name))
			}

			// validate port name is not reserved
			if lo.Contains[string]([]string{
				abstract.FunctionContainerHTTPPortName,
				abstract.FunctionContainerMetricPortName,
			}, port.Name) {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Port name %s is reserved for Nuclio internal use", port.Name))
			}

			// validate port name is unique
			if _, exists := portNames[port.Name]; exists {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Port name %s is duplicated in container %s", port.Name, container.Name))
			}

			// validate port number is unique
			if _, exists := portNumbers[port.ContainerPort]; exists {
				return nuclio.NewErrBadRequest(fmt.Sprintf("Port number %d is duplicated in container %s", port.ContainerPort, container.Name))
			}

			portNames[port.Name] = true
			portNumbers[port.ContainerPort] = true
		}
	}

	return nil
}

func (p *Platform) validateFunctionIngresses(ctx context.Context, functionConfig *functionconfig.Config) error {

	listIngressesOptions := metav1.ListOptions{

		// validate ingresses not created by this function
		FieldSelector: fmt.Sprintf("metadata.name!=%s", IngressNameFromFunctionName(functionConfig.Meta.Name)),
	}

	ingresses := functionconfig.GetFunctionIngresses(functionConfig)
	if err := p.validateIngressHostAndPathAvailability(ctx,
		listIngressesOptions,
		functionConfig.Meta.Namespace,
		ingresses); err != nil {
		return errors.Wrapf(err, "Failed to validate the function-ingress host and path availability")
	}

	return nil
}

func (p *Platform) validateIngressHostAndPathAvailability(ctx context.Context,
	listIngressesOptions metav1.ListOptions,
	namespace string,
	ingresses map[string]functionconfig.Ingress) error {

	// get all ingresses on the namespace
	existingIngresses, err := p.consumer.KubeClientSet.
		NetworkingV1().
		Ingresses(namespace).
		List(ctx, listIngressesOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to list ingresses")
	}

	if len(existingIngresses.Items) == 0 {
		return nil
	}

	// iterate over all ingress instances to validate
	for _, ingressInstance := range ingresses {
		var ingressNormalizedPaths []string

		// normalize ingress instance paths
		for _, path := range ingressInstance.Paths {
			ingressNormalizedPaths = append(ingressNormalizedPaths, common.NormalizeURLPath(path))
		}

		// iterate over all existing ingresses to see if any of them matches host+path of the args ingresses
		for _, existingIngressInstance := range existingIngresses.Items {
			for _, existingIngressRule := range existingIngressInstance.Spec.Rules {
				if ingressInstance.Host == existingIngressRule.Host {

					// if rule HTTP struct is nil - return conflict error only if some path is empty
					if existingIngressRule.HTTP == nil {
						if common.StringInSlice("/", ingressNormalizedPaths) {
							return platform.ErrIngressHostPathInUse
						}

						// rule host HTTP struct is nil - continue to the next rule
						continue
					}

					// check if one of the paths matches on of our paths
					for _, existingIngressPath := range existingIngressRule.HTTP.Paths {
						normalizedPathInstance := common.NormalizeURLPath(existingIngressPath.Path)
						if common.StringInSlice(normalizedPathInstance, ingressNormalizedPaths) {
							return platform.ErrIngressHostPathInUse
						}
					}
				}
			}
		}
	}

	return nil
}

func (p *Platform) enrichHTTPTriggerIngresses(ctx context.Context,
	httpTrigger *functionconfig.Trigger,
	functionConfig *functionconfig.Config) error {

	ingresses, hasIngresses := httpTrigger.Attributes["ingresses"]
	if !hasIngresses {
		return nil
	}

	templateData := map[string]interface{}{
		"Name":         functionConfig.Meta.Name,
		"ResourceName": functionConfig.Meta.Name,
		"Namespace":    functionConfig.Meta.Namespace,
		"ProjectName":  functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
	}

	// iterate over the encoded ingresses map and created ingress structures
	encodedIngresses := ingresses.(map[string]interface{})
	for _, encodedIngress := range encodedIngresses {
		encodedIngressMap := encodedIngress.(map[string]interface{})

		if ingressHostTemplate, hostTemplateFound := encodedIngressMap["hostTemplate"].(string); hostTemplateFound {

			// try infer from attributes, if not use default 8
			hostTemplateRandomCharsLength := 8
			if hostTemplateRandomCharsLengthValue, ok := encodedIngressMap["hostTemplateRandomCharsLength"].(int); ok {
				hostTemplateRandomCharsLength = hostTemplateRandomCharsLengthValue
			}
			renderedIngressHost, err := p.renderIngressHost(ctx, ingressHostTemplate, templateData, hostTemplateRandomCharsLength)
			if err != nil {
				return errors.Wrap(err, "Failed to render ingress host template")
			}
			if ingressHost, ingressHostFound := encodedIngressMap["host"].(string); !ingressHostFound || ingressHost == "" {
				p.Logger.DebugWithCtx(ctx, "Enriching function ingress host from template",
					"renderedIngressHost", renderedIngressHost,
					"functionName", functionConfig.Meta.Name)
				encodedIngressMap["host"] = renderedIngressHost
			}
		}

		if _, ingressPathTypeFound := encodedIngressMap["pathType"].(networkingv1.PathType); !ingressPathTypeFound {
			encodedIngressMap["pathType"] = networkingv1.PathTypeImplementationSpecific
		}
	}
	return nil
}

func (p *Platform) renderIngressHost(ctx context.Context, ingressHostTemplate string, templateData map[string]interface{}, hostTemplateRandomCharsLength int) (string, error) {
	// one way to say "just render me the default"
	if ingressHostTemplate == common.DefaultIngressHostTemplate {
		ingressHostTemplate = p.Config.Kube.DefaultHTTPIngressHostTemplate
	} else {
		p.Logger.DebugWithCtx(ctx,
			"Received custom ingress host template to enrich host with",
			"ingressHostTemplate", ingressHostTemplate)
	}

	// render host with pre-defined data
	renderedIngressHost, err := common.RenderTemplate(ingressHostTemplate, templateData)
	if err != nil {
		return "", err
	}

	return p.alignIngressHostSubdomainLevel(renderedIngressHost, hostTemplateRandomCharsLength), nil
}

// will take a host, split to "."
// for each component, will ensure its max length is not >63
// if it does, it will truncate by randomCharsLength+1 and add "-<random-chars>" to it
func (p *Platform) alignIngressHostSubdomainLevel(host string, randomCharsLength int) string {

	// backdoor to make it stop
	if randomCharsLength == -1 {
		return host
	}
	var reconstructedHost []string
	hostLevels := strings.Split(host, ".")
	for _, hostLevel := range hostLevels {

		// DNS domain level limitation is 63 chars
		if len(hostLevel) <= common.KubernetesDomainLevelMaxLength {
			reconstructedHost = append(reconstructedHost, hostLevel)
			continue
		}
		randomSuffix := common.GenerateRandomString(randomCharsLength, common.SmallLettersAndNumbers)
		truncatedHostLevel := hostLevel[:common.KubernetesDomainLevelMaxLength-randomCharsLength-1]
		truncatedHostLevel = strings.TrimSuffix(truncatedHostLevel, "-")
		reconstructedHost = append(reconstructedHost, fmt.Sprintf("%s-%s", truncatedHostLevel, randomSuffix))
	}
	return strings.Join(reconstructedHost, ".")
}

func (p *Platform) getAPIGatewayUpstreamFunctions(ctx context.Context,
	apiGateway *platform.APIGatewayConfig,
	validateFunctionExistence bool) ([]platform.Function, error) {

	var upstreamFunctionLock sync.Mutex
	var upstreamFunctions []platform.Function

	// get upstream functions
	errGroup, _ := errgroup.WithContext(ctx, p.Logger)
	for _, upstream := range apiGateway.Spec.Upstreams {
		upstream := upstream
		errGroup.Go("GetUpstreamFunction", func() error {
			function, err := p.getFunction(ctx,
				&platform.GetFunctionsOptions{
					Namespace: apiGateway.Meta.Namespace,
					Name:      upstream.NuclioFunction.Name,
				})
			if err != nil {
				return errors.New("Failed to get upstream function")
			}
			if function == nil {
				if validateFunctionExistence {
					return nuclio.NewErrPreconditionFailed(fmt.Sprintf("Function %s does not exist",
						upstream.NuclioFunction.Name))
				}
				return nil
			}

			functionInstance, err := client.NewFunction(p.Logger, p, function, p.consumer)
			if err != nil {
				return errors.Wrap(err, "Failed to initialize function")
			}

			upstreamFunctionLock.Lock()
			upstreamFunctions = append(upstreamFunctions, functionInstance)
			upstreamFunctionLock.Unlock()
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, errors.Wrap(err, "Failed to get upstream functions")
	}
	return upstreamFunctions, nil
}

func (p *Platform) getProjectCacheKey(projectMeta platform.ProjectMeta, owner string) string {
	return fmt.Sprintf("%s/%s", projectMeta.Name, owner)
}
