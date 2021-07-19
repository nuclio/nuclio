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

package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

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
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Platform struct {
	*abstract.Platform
	deployer       *client.Deployer
	getter         *client.Getter
	updater        *client.Updater
	deleter        *client.Deleter
	kubeconfigPath string
	consumer       *client.Consumer
	projectsClient project.Client
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
func NewPlatform(parentLogger logger.Logger,
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
	newPlatform.kubeconfigPath = common.GetKubeconfigPath(platformConfiguration.Kube.KubeConfigPath)

	// create consumer
	newPlatform.consumer, err = client.NewConsumer(newPlatform.Logger, newPlatform.kubeconfigPath)
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

	// create projects client
	newPlatform.projectsClient, err = NewProjectsClient(newPlatform, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create projects client")
	}

	// create container builder
	if platformConfiguration.ContainerBuilderConfiguration.Kind == "kaniko" {
		newPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewKaniko(newPlatform.Logger,
			newPlatform.consumer.KubeClientSet, platformConfiguration.ContainerBuilderConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create a kaniko builder")
		}
	} else {

		// Default container image builder
		newPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewDocker(newPlatform.Logger,
			platformConfiguration.ContainerBuilderConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create a Docker builder")
		}
	}

	return newPlatform, nil
}

func (p *Platform) Initialize() error {
	if err := p.projectsClient.Initialize(); err != nil {
		return errors.Wrap(err, "Failed to initialize projects client")
	}

	// ensure default project existence only when projects aren't managed by external leader
	if p.Config.ProjectsLeader == nil {
		if err := p.EnsureDefaultProjectExistence(); err != nil {
			return errors.Wrap(err, "Failed to ensure default project existence")
		}
	}

	return nil
}

// CreateFunction will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (
	*platform.CreateFunctionResult, error) {

	var existingFunctionInstance *nuclioio.NuclioFunction
	var existingFunctionConfig *functionconfig.ConfigWithStatus

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams.Store(createFunctionOptions.FunctionConfig.Meta.GetUniqueID(), logStream)

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	if err := p.enrichAndValidateFunctionConfig(&createFunctionOptions.FunctionConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to enrich and validate a function configuration")
	}

	// Check OPA permissions
	permissionOptions := createFunctionOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := p.QueryOPAFunctionPermissions(createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"],
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
			p.getFunctionInstanceAndConfig(createFunctionOptions.FunctionConfig.Meta.Namespace,
				createFunctionOptions.FunctionConfig.Meta.Name,
				true)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get an existing function configuration")
		}
	}

	// if function exists, perform some validation with new function create options
	if err := p.ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(existingFunctionConfig,
		createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed to validate a function configuration against an existing configuration")
	}

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
		createFunctionOptions.Logger.DebugWith("Function creation failed, brief error message extracted",
			"briefErrorsMessage", briefErrorsMessage)

		createFunctionOptions.Logger.WarnWith("Function creation failed, updating function status",
			"errorStack", errorStack.String())

		functionStatus := &functionconfig.Status{
			State:   functionconfig.FunctionStateError,
			Message: briefErrorsMessage,
		}
		if existingFunctionInstance != nil {

			// preserve invocation metadata for when function become healthy again
			functionStatus.HTTPPort = existingFunctionInstance.Status.HTTPPort
			functionStatus.ExternalInvocationURLs = existingFunctionInstance.Status.ExternalInvocationURLs
			functionStatus.InternalInvocationURLs = existingFunctionInstance.Status.InternalInvocationURLs

			// if function deployment ended up with unhealthy, due to unstable Kubernetes env that lead
			// to failing on waiting for function readiness.
			// it is desired to preserve the function unhealthiness state set by the controller, to allow
			// function recovery later on, when Kubernetes become stable
			// alternatively, set function in error state to indicate deployment has failed
			if existingFunctionInstance.Status.State == functionconfig.FunctionStateUnhealthy {
				functionStatus.State = functionconfig.FunctionStateUnhealthy
			}
		}

		// create or update the function. The possible creation needs to happen here, since on cases of
		// early build failures we might get here before the function CR was created. After this point
		// it is guaranteed to be created and updated with the reported error state
		_, err := p.deployer.CreateOrUpdateFunction(existingFunctionInstance,
			createFunctionOptions,
			functionStatus,
		)
		return err
	}

	// the builder may update the configuration, so we have to create the function in the platform only after
	// the builder does that
	onAfterConfigUpdated := func() error {
		var err error

		// enrich and validate again because it may not be valid after config was updated by external code entry type
		if err := p.enrichAndValidateFunctionConfig(&createFunctionOptions.FunctionConfig); err != nil {
			return errors.Wrap(err, "Failed to enrich and validate an updated function configuration")
		}

		existingFunctionInstance, err = p.getFunction(createFunctionOptions.FunctionConfig.Meta.Namespace,
			createFunctionOptions.FunctionConfig.Meta.Name)
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
		existingFunctionInstance, err = p.deployer.CreateOrUpdateFunction(existingFunctionInstance,
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
				p.Logger.ErrorWith("Failed to report a creation error",
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
			p.Logger.Info("Skipping function deployment",
				"functionName", createFunctionOptions.FunctionConfig.Meta.Name,
				"functionNamespace", createFunctionOptions.FunctionConfig.Meta.Namespace)

			if _, err := p.deployer.CreateOrUpdateFunction(existingFunctionInstance,
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

		createFunctionResult, updatedFunctionInstance, briefErrorsMessage, deployErr := p.deployer.Deploy(existingFunctionInstance,
			createFunctionOptions)

		// update the function instance (after the deployment)
		if updatedFunctionInstance != nil {
			existingFunctionInstance = updatedFunctionInstance
		}

		if deployErr != nil {

			// try to report the error
			reportingErr := reportCreationError(deployErr, briefErrorsMessage, true)
			if reportingErr != nil {
				p.Logger.ErrorWith("Failed to report a deployment error",
					"reportingErr", reportingErr,
					"buildErr", buildErr)
				return nil, reportingErr
			}

			return nil, deployErr
		}

		return createFunctionResult, nil
	}

	// do the deploy in the abstract base class
	return p.HandleDeployFunction(existingFunctionConfig, createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
}

func (p Platform) EnrichFunctionConfig(functionConfig *functionconfig.Config) error {
	if err := p.Platform.EnrichFunctionConfig(functionConfig); err != nil {
		return err
	}

	if err := p.enrichHTTPTriggers(functionConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich http trigger")
	}

	return nil
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	functions, err := p.getter.Get(p.consumer, getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functions, err = p.Platform.FilterFunctionsByPermissions(&getFunctionsOptions.PermissionOptions, functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to filter functions by permissions")
	}

	p.EnrichFunctionsWithDeployLogStream(functions)

	if getFunctionsOptions.EnrichWithAPIGateways {
		if err = p.enrichFunctionsWithAPIGateways(functions, getFunctionsOptions.Namespace); err != nil {

			// relevant when upgrading nuclio from a version that didn't have api-gateways to one that has
			if !strings.Contains(errors.RootCause(err).Error(),
				"the server could not find the requested resource (get nuclioapigateways.nuclio.io)") {

				return nil, errors.Wrap(err, "Failed to enrich functions with API gateways")
			}

			p.Logger.DebugWith("Api-gateway crd isn't installed; skipping function api gateways enrichment",
				"err", err)
		}
	}

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	return p.updater.Update(updateFunctionOptions)
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	p.Logger.DebugWith("Deleting function",
		"functionConfig", deleteFunctionOptions.FunctionConfig)

	// pre delete validation
	if err := p.ValidateDeleteFunctionOptions(deleteFunctionOptions); err != nil {
		return errors.Wrap(err, "Failed to validate function-deletion options")
	}

	// user must clean api gateway before deleting the function
	if err := p.validateFunctionHasNoAPIGateways(deleteFunctionOptions); err != nil {
		return errors.Wrap(err, "Failed to validate that the function has no API gateways")
	}

	return p.deleter.Delete(p.consumer, deleteFunctionOptions)
}

func (p *Platform) GetFunctionReplicaLogsStream(ctx context.Context,
	options *platform.GetFunctionReplicaLogsStreamOptions) (io.ReadCloser, error) {
	return p.consumer.KubeClientSet.
		CoreV1().
		Pods(options.Namespace).
		GetLogs(options.Name, &v1.PodLogOptions{
			Container:    client.FunctionContainerName,
			SinceSeconds: options.SinceSeconds,
			TailLines:    options.TailLines,
			Follow:       options.Follow,
		}).
		Context(ctx).
		Stream()
}

func (p *Platform) GetFunctionReplicaNames(ctx context.Context,
	functionConfig *functionconfig.Config) ([]string, error) {

	pods, err := p.consumer.KubeClientSet.
		CoreV1().
		Pods(functionConfig.Meta.Namespace).
		List(metav1.ListOptions{
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

func IsInCluster() bool {
	return len(os.Getenv("KUBERNETES_SERVICE_HOST")) != 0 && len(os.Getenv("KUBERNETES_SERVICE_PORT")) != 0
}

// GetName returns the platform name
func (p *Platform) GetName() string {
	return "kube"
}

// GetNodes returns a slice of nodes currently in the cluster
func (p *Platform) GetNodes() ([]platform.Node, error) {
	var platformNodes []platform.Node

	kubeNodes, err := p.consumer.KubeClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get nodes")
	}

	// iterate over nodes and convert to platform nodes
	for _, kubeNode := range kubeNodes.Items {
		platformNodes = append(platformNodes, &node{
			Node: kubeNode,
		})
	}

	return platformNodes, nil
}

// CreateProject creates a new project
func (p *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {

	// enrich
	if err := p.EnrichCreateProjectConfig(createProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to enrich a project configuration")
	}

	// validate
	if err := p.ValidateProjectConfig(createProjectOptions.ProjectConfig); err != nil {
		return errors.Wrap(err, "Failed to validate a project configuration")
	}

	// create
	p.Logger.DebugWith("Creating project",
		"projectName", createProjectOptions.ProjectConfig.Meta.Name)
	if _, err := p.projectsClient.Create(createProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to create project")
	}

	return nil
}

// UpdateProject updates an existing project
func (p *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	if err := p.ValidateProjectConfig(&updateProjectOptions.ProjectConfig); err != nil {
		return nuclio.WrapErrBadRequest(err)
	}

	if _, err := p.projectsClient.Update(updateProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to update project")
	}

	return nil
}

// DeleteProject will delete a previously existing project
func (p *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := p.Platform.ValidateDeleteProjectOptions(deleteProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to validate delete project options")
	}

	// check only, do not delete
	if deleteProjectOptions.Strategy == platform.DeleteProjectStrategyCheck {
		p.Logger.DebugWith("Project is ready for deletion", "projectMeta", deleteProjectOptions.Meta)
		return nil
	}

	p.Logger.DebugWith("Deleting project", "projectMeta", deleteProjectOptions.Meta)
	if err := p.projectsClient.Delete(deleteProjectOptions); err != nil {
		return errors.Wrap(err, "Failed to delete project")
	}

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		return p.Platform.WaitForProjectResourcesDeletion(&deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration)
	}

	return nil
}

// GetProjects will list existing projects
func (p *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return p.projectsClient.Get(getProjectsOptions)
}

// CreateAPIGateway creates and deploys a new api gateway
func (p *Platform) CreateAPIGateway(createAPIGatewayOptions *platform.CreateAPIGatewayOptions) error {
	newAPIGateway := nuclioio.NuclioAPIGateway{}

	// enrich
	p.EnrichAPIGatewayConfig(createAPIGatewayOptions.APIGatewayConfig)

	// validate
	if err := p.ValidateAPIGatewayConfig(createAPIGatewayOptions.APIGatewayConfig); err != nil {
		return errors.Wrap(err, "Failed to validate and enrich an API-gateway name")
	}

	p.platformAPIGatewayToAPIGateway(createAPIGatewayOptions.APIGatewayConfig, &newAPIGateway)

	// set api gateway state to "waitingForProvisioning", so the controller will know to create/update this resource
	newAPIGateway.Status.State = platform.APIGatewayStateWaitingForProvisioning

	// create
	_, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(newAPIGateway.Namespace).
		Create(&newAPIGateway)
	if err != nil {
		return errors.Wrap(err, "Failed to create an API gateway")
	}

	return nil
}

// UpdateAPIGateway will update a previously existing api gateway
func (p *Platform) UpdateAPIGateway(updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) error {
	apiGateway, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Get(updateAPIGatewayOptions.APIGatewayConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get api gateway to update")
	}

	// enrich
	p.EnrichAPIGatewayConfig(updateAPIGatewayOptions.APIGatewayConfig)

	// validate
	if err := p.ValidateAPIGatewayConfig(updateAPIGatewayOptions.APIGatewayConfig); err != nil {
		return errors.Wrap(err, "Failed to validate and enrich an API-gateway name")
	}

	apiGateway.Annotations = updateAPIGatewayOptions.APIGatewayConfig.Meta.Annotations
	apiGateway.Labels = updateAPIGatewayOptions.APIGatewayConfig.Meta.Labels
	apiGateway.Spec = updateAPIGatewayOptions.APIGatewayConfig.Spec

	// set api gateway state to "waitingForProvisioning", so the controller will know to create/update this resource
	apiGateway.Status.State = platform.APIGatewayStateWaitingForProvisioning

	// update
	if _, err = p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Update(apiGateway); err != nil {
		return errors.Wrap(err, "Failed to update an API gateway")
	}

	return nil
}

// DeleteAPIGateway will delete a previously existing api gateway
func (p *Platform) DeleteAPIGateway(deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) error {

	// validate
	if err := p.validateAPIGatewayMeta(&deleteAPIGatewayOptions.Meta); err != nil {
		return errors.Wrap(err, "Failed to validate an API gateway's metadata")
	}

	p.Logger.DebugWith("Deleting api gateway", "name", deleteAPIGatewayOptions.Meta.Name)

	// delete
	if err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(deleteAPIGatewayOptions.Meta.Namespace).
		Delete(deleteAPIGatewayOptions.Meta.Name, &metav1.DeleteOptions{}); err != nil {

		return errors.Wrapf(err,
			"Failed to delete API gateway %s from namespace %s",
			deleteAPIGatewayOptions.Meta.Name,
			deleteAPIGatewayOptions.Meta.Namespace)
	}

	return nil
}

// GetAPIGateways will list existing api gateways
func (p *Platform) GetAPIGateways(getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) ([]platform.APIGateway, error) {
	var platformAPIGateways []platform.APIGateway
	var apiGateways []nuclioio.NuclioAPIGateway

	// if identifier specified, we need to get a single NuclioAPIGateway
	if getAPIGatewaysOptions.Name != "" {

		// get specific NuclioAPIGateway CR
		apiGateway, err := p.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioAPIGateways(getAPIGatewaysOptions.Namespace).
			Get(getAPIGatewaysOptions.Name, metav1.GetOptions{})
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
			List(metav1.ListOptions{LabelSelector: getAPIGatewaysOptions.Labels})
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
func (p *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	newFunctionEvent := nuclioio.NuclioFunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&createFunctionEventOptions.FunctionEventConfig, &newFunctionEvent)

	if err := p.Platform.EnrichFunctionEvent(&createFunctionEventOptions.FunctionEventConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich function event")
	}

	functionName := newFunctionEvent.Labels[common.NuclioResourceLabelKeyFunctionName]
	projectName := newFunctionEvent.Labels[common.NuclioResourceLabelKeyProjectName]

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
		Create(&newFunctionEvent); err != nil {
		return errors.Wrap(err, "Failed to create a function event")
	}
	return nil
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	updatedFunctionEvent := nuclioio.NuclioFunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig, &updatedFunctionEvent)

	functionEvent, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Get(updateFunctionEventOptions.FunctionEventConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get a function event")
	}

	if err := p.Platform.EnrichFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig); err != nil {
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
		Update(functionEvent); err != nil {
		return errors.Wrap(err, "Failed to update a function event")
	}

	return nil
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	functionEventToDelete, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(deleteFunctionEventOptions.Meta.Namespace).
		Get(deleteFunctionEventOptions.Meta.Name, metav1.GetOptions{})
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
		Delete(deleteFunctionEventOptions.Meta.Name, &metav1.DeleteOptions{}); err != nil {
		return errors.Wrapf(err,
			"Failed to delete function event %s from namespace %s",
			deleteFunctionEventOptions.Meta.Name,
			deleteFunctionEventOptions.Meta.Namespace)
	}

	return nil
}

// GetFunctionEvents will list existing function events
func (p *Platform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	var platformFunctionEvents []platform.FunctionEvent
	var functionEvents []nuclioio.NuclioFunctionEvent

	// if identifier specified, we need to get a single function event
	if getFunctionEventsOptions.Meta.Name != "" {

		// get specific function event CR
		functionEvent, err := p.consumer.NuclioClientSet.NuclioV1beta1().
			NuclioFunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			Get(getFunctionEventsOptions.Meta.Name, metav1.GetOptions{})

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
			List(metav1.ListOptions{LabelSelector: labelSelector})

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

	return p.Platform.FilterFunctionEventsByPermissions(&getFunctionEventsOptions.PermissionOptions,
		platformFunctionEvents)
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
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

	nodes, err := p.GetNodes()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get nodes")
	}

	// try to get an external IP address from one of the nodes. if that doesn't work,
	// try to get an internal IP
	for _, addressType := range []platform.AddressType{
		platform.AddressTypeExternalIP,
		platform.AddressTypeInternalIP} {

		for _, node := range nodes {
			for _, address := range node.GetAddresses() {
				if address.Type == addressType {
					externalIPAddress = append(externalIPAddress, address.Address)
				}
			}
		}

		// if we found addresses of a given type, return them
		if len(externalIPAddress) != 0 {
			return externalIPAddress, nil
		}
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

	return nil, errors.New("No external addresses found")
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (p *Platform) ResolveDefaultNamespace(defaultNamespace string) string {
	if defaultNamespace == "" {
		defaultNamespace = p.DefaultNamespace
	}

	if defaultNamespace == "@nuclio.selfNamespace" {

		// get namespace from within the pod. if found, return that
		if namespacePod, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			return string(namespacePod)
		}
	}

	if defaultNamespace == "" {

		return "default"
	}

	return defaultNamespace
}

// GetNamespaces returns all the namespaces in the platform
func (p *Platform) GetNamespaces() ([]string, error) {
	if len(p.Config.ManagedNamespaces) > 0 {
		return p.Config.ManagedNamespaces, nil
	}

	namespaces, err := p.consumer.KubeClientSet.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, nuclio.WrapErrForbidden(err)
		}
		return nil, errors.Wrap(err, "Failed to list namespaces")
	}

	var namespaceNames []string

	for _, namespace := range namespaces.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}

	return namespaceNames, nil
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

func (p *Platform) SaveFunctionDeployLogs(functionName, namespace string) error {
	functions, err := p.GetFunctions(&platform.GetFunctionsOptions{
		Name:      functionName,
		Namespace: namespace,
	})
	if err != nil || len(functions) == 0 {
		return errors.Wrap(err, "Failed to get existing functions")
	}

	// enrich with build logs
	p.EnrichFunctionsWithDeployLogStream(functions)

	function := functions[0]

	return p.updater.Update(&platform.UpdateFunctionOptions{
		FunctionMeta:   &function.GetConfig().Meta,
		FunctionStatus: function.GetStatus(),
	})
}

func (p *Platform) generateFunctionToAPIGatewaysMapping(namespace string) (map[string][]string, error) {
	functionToAPIGateways := map[string][]string{}

	// get all api gateways in the namespace
	apiGateways, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(namespace).
		List(metav1.ListOptions{})
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

func (p *Platform) enrichFunctionsWithAPIGateways(functions []platform.Function, namespace string) error {
	var err error
	var functionToAPIGateways map[string][]string

	// no functions to enrich
	if len(functions) == 0 {
		return nil
	}

	// generate function to api gateways mapping
	if functionToAPIGateways, err = p.generateFunctionToAPIGatewaysMapping(namespace); err != nil {
		return errors.Wrap(err, "Failed to get a function to API-gateways mapping")
	}

	// set the api gateways list for every function according to the mapping above
	for _, function := range functions {
		function.GetStatus().APIGateways = functionToAPIGateways[function.GetConfig().Meta.Name]
	}

	return nil
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

func (p *Platform) getFunction(namespace, name string) (*nuclioio.NuclioFunction, error) {
	p.Logger.DebugWith("Getting function",
		"namespace", namespace,
		"name", name)

	// get specific function CR
	function, err := p.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctions(namespace).
		Get(name, metav1.GetOptions{})
	if err != nil {

		// if we didn't find the function, return nothing
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "Failed to get a function")
	}

	p.Logger.DebugWith("Completed getting function",
		"name", name,
		"namespace", namespace,
		"function", function)

	return function, nil
}

func (p *Platform) getFunctionInstanceAndConfig(namespace string,
	name string, enrichWithAPIGateway bool) (*nuclioio.NuclioFunction, *functionconfig.ConfigWithStatus, error) {
	functionInstance, err := p.getFunction(namespace, name)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get a function")
	}

	// found function instance, return as function config
	if functionInstance != nil {
		initializedFunctionInstance, err := client.NewFunction(p.Logger, p, functionInstance, p.consumer)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to create a new function instance")
		}
		if enrichWithAPIGateway {
			if err := p.enrichFunctionsWithAPIGateways([]platform.Function{initializedFunctionInstance},
				namespace); err != nil {
				return nil, nil, errors.Wrap(err, "Failed to enrich function with api gateway")
			}
			functionInstance.Status.APIGateways = initializedFunctionInstance.GetConfigWithStatus().Status.APIGateways
		}
		return functionInstance, initializedFunctionInstance.GetConfigWithStatus(), nil
	}
	return nil, nil, nil
}

func (p *Platform) platformProjectToProject(platformProject *platform.ProjectConfig, project *nuclioio.NuclioProject) {
	project.Name = platformProject.Meta.Name
	project.Namespace = platformProject.Meta.Namespace
	project.Labels = platformProject.Meta.Labels
	project.Annotations = platformProject.Meta.Annotations
	project.Spec = platformProject.Spec
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

func (p *Platform) EnrichAPIGatewayConfig(apiGatewayConfig *platform.APIGatewayConfig) {

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

	p.EnrichLabelsWithProjectName(apiGatewayConfig.Meta.Labels)
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

func (p *Platform) ValidateAPIGatewayConfig(platformAPIGateway *platform.APIGatewayConfig) error {

	// general validations
	if platformAPIGateway.Spec.Name != platformAPIGateway.Meta.Name {
		return nuclio.NewErrBadRequest("Api gateway metadata.name must match api gateway spec.name")
	}
	if common.StringInSlice(platformAPIGateway.Spec.Name, p.ResolveReservedResourceNames()) {
		return nuclio.NewErrPreconditionFailed(fmt.Sprintf("Api gateway name '%s' is reserved and cannot be used",
			platformAPIGateway.Spec.Name))
	}

	// meta
	if err := p.validateAPIGatewayMeta(&platformAPIGateway.Meta); err != nil {
		return errors.Wrap(err, "Failed to validate API-gateway metadata")
	}

	// spec
	if err := ValidateAPIGatewaySpec(&platformAPIGateway.Spec); err != nil {
		return errors.Wrap(err, "Failed to validate the API-gateway spec")
	}

	return p.validateAPIGatewayIngresses(platformAPIGateway)
}

func (p *Platform) ValidateFunctionConfig(functionConfig *functionconfig.Config) error {
	if err := p.Platform.ValidateFunctionConfig(functionConfig); err != nil {
		return err
	}

	return p.validateFunctionIngresses(functionConfig)
}

func (p *Platform) enrichAndValidateFunctionConfig(functionConfig *functionconfig.Config) error {
	if err := p.EnrichFunctionConfig(functionConfig); err != nil {
		return errors.Wrap(err, "Failed to enrich a function configuration")
	}

	if err := p.ValidateFunctionConfig(functionConfig); err != nil {
		return errors.Wrap(err, "Failed to validate a function configuration")
	}

	return nil
}

func (p *Platform) enrichHTTPTriggers(functionConfig *functionconfig.Config) error {

	// for backwards compatibility
	serviceType := functionConfig.Spec.ServiceType
	if serviceType == "" {
		serviceType = p.Config.Kube.DefaultServiceType
	}

	for triggerName, trigger := range functionconfig.GetTriggersByKind(functionConfig.Spec.Triggers, "http") {
		p.enrichTriggerWithServiceType(functionConfig, &trigger, serviceType)
		if err := p.enrichHTTPTriggerIngresses(&trigger, functionConfig); err != nil {
			return errors.Wrap(err, "Failed to enrich HTTP trigger ingresses")
		}
		functionConfig.Spec.Triggers[triggerName] = trigger
	}

	return nil
}

func (p *Platform) validateFunctionHasNoAPIGateways(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	var functionToAPIGateways map[string][]string
	var err error

	// generate function to api gateways mapping
	if functionToAPIGateways, err = p.generateFunctionToAPIGatewaysMapping(deleteFunctionOptions.FunctionConfig.Meta.Namespace); err != nil {
		return errors.Wrap(err, "Failed to get a function to API-gateways mapping")
	}

	if len(functionToAPIGateways[deleteFunctionOptions.FunctionConfig.Meta.Name]) > 0 {
		return platform.ErrFunctionIsUsedByAPIGateways
	}

	return nil
}

func (p *Platform) enrichTriggerWithServiceType(functionConfig *functionconfig.Config,
	trigger *functionconfig.Trigger,
	serviceType v1.ServiceType) {

	if trigger.Attributes == nil {
		trigger.Attributes = map[string]interface{}{}
	}

	if triggerServiceType, serviceTypeExists := trigger.Attributes["serviceType"]; !serviceTypeExists || triggerServiceType == "" {

		p.Logger.DebugWith("Enriching function HTTP trigger with service type",
			"functionName", functionConfig.Meta.Name,
			"triggerName", trigger.Name,
			"serviceType", serviceType)
		trigger.Attributes["serviceType"] = serviceType
	}
}

func (p *Platform) validateAPIGatewayIngresses(apiGatewayConfig *platform.APIGatewayConfig) error {
	if err := p.validateAPIGatewayFunctionsHaveNoIngresses(apiGatewayConfig); err != nil {
		return errors.Wrap(err, "Failed to validate: API-gateway functions have no ingresses")
	}

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

	if err := p.validateIngressHostAndPathAvailability(listIngressesOptions,
		apiGatewayConfig.Meta.Namespace,
		apiGatewayIngresses); err != nil {
		return errors.Wrap(err, "Failed to validate the API-gateway host and path availability")
	}

	return nil
}

func (p *Platform) validateFunctionIngresses(functionConfig *functionconfig.Config) error {
	if err := p.validateFunctionNoIngressAndAPIGateway(functionConfig); err != nil {
		return errors.Wrap(err, "Failed to validate: the function isn't exposed by an internal ingresses or an API gateway")
	}

	listIngressesOptions := metav1.ListOptions{

		// validate ingresses not created by this function
		FieldSelector: fmt.Sprintf("metadata.name!=%s", IngressNameFromFunctionName(functionConfig.Meta.Name)),
	}

	ingresses := functionconfig.GetFunctionIngresses(functionConfig)
	if err := p.validateIngressHostAndPathAvailability(listIngressesOptions,
		functionConfig.Meta.Namespace,
		ingresses); err != nil {
		return errors.Wrapf(err, "Failed to validate the function-ingress host and path availability")
	}

	return nil
}

func (p *Platform) validateIngressHostAndPathAvailability(listIngressesOptions metav1.ListOptions,
	namespace string,
	ingresses map[string]functionconfig.Ingress) error {

	// get all ingresses on the namespace
	existingIngresses, err := p.consumer.KubeClientSet.
		ExtensionsV1beta1().
		Ingresses(namespace).
		List(listIngressesOptions)
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

func (p *Platform) validateAPIGatewayFunctionsHaveNoIngresses(apiGatewayConfig *platform.APIGatewayConfig) error {

	// check ingresses on every upstream function
	errGroup, _ := errgroup.WithContext(context.TODO(), p.Logger)
	for _, upstream := range apiGatewayConfig.Spec.Upstreams {
		upstream := upstream
		errGroup.Go("GetFunctionIngresses", func() error {
			function, err := p.getFunction(apiGatewayConfig.Meta.Namespace, upstream.NuclioFunction.Name)
			if err != nil {
				return errors.New("Failed to get upstream function")
			}
			if function == nil {

				// if such function doesn't exist, just skip - because it doesn't have ingresses for sure
				return nil
			}

			ingresses := functionconfig.GetFunctionIngresses(client.NuclioioToFunctionConfig(function))
			if len(ingresses) > 0 {
				return nuclio.NewErrPreconditionFailed(
					fmt.Sprintf("Api gateway upstream function: %s must not have an ingress",
						upstream.NuclioFunction.Name))
			}
			return nil
		})
	}

	return errGroup.Wait()
}

// validate that a function is not exposed inside http triggers, while it is also exposed by an api gateway
// this is done to prevent the nginx bug, where it is not working properly when the same service is exposed more than once
// (e.g. when a service is exposed by an ingress with host-1.com without canary ingress, and on another api gateway with host-2.com
// with canary ingress, when sending requests to host-1.com we may get directed to the canary ingress defined by the api gateway)
func (p *Platform) validateFunctionNoIngressAndAPIGateway(functionConfig *functionconfig.Config) error {
	ingresses := functionconfig.GetFunctionIngresses(functionConfig)
	if len(ingresses) > 0 {

		// TODO: when we'll add upstream labels to api gateway, use get api gateways by label to replace this line
		functionToAPIGateways, err := p.generateFunctionToAPIGatewaysMapping(functionConfig.Meta.Namespace)
		if err != nil {
			return errors.Wrap(err, "Failed to get a function to API-gateways mapping")
		}
		if _, found := functionToAPIGateways[functionConfig.Meta.Name]; found {
			return nuclio.NewErrBadRequest("Function can't expose ingresses while it is being exposed by an API gateway")
		}
	}

	return nil
}

func (p *Platform) enrichHTTPTriggerIngresses(httpTrigger *functionconfig.Trigger,
	functionConfig *functionconfig.Config) error {

	ingresses, hasIngresses := httpTrigger.Attributes["ingresses"]
	if !hasIngresses {
		return nil
	}

	templateData := map[string]interface{}{
		"Name":         functionConfig.Meta.Name,
		"ResourceName": functionConfig.Meta.Name,
		"Namespace":    functionConfig.Meta.Namespace,
		"ProjectName":  functionConfig.Meta.Labels["nuclio.io/project-name"],
	}

	// iterate over the encoded ingresses map and created ingress structures
	encodedIngresses := ingresses.(map[string]interface{})
	for _, encodedIngress := range encodedIngresses {
		encodedIngressMap := encodedIngress.(map[string]interface{})

		if ingressHostTemplate, hostTemplateFound := encodedIngressMap["hostTemplate"].(string); hostTemplateFound {

			// one way to say "just render me the default"
			if ingressHostTemplate == "@nuclio.fromDefault" {
				ingressHostTemplate = p.Config.Kube.DefaultHTTPIngressHostTemplate
			} else {
				p.Logger.DebugWith("Received custom ingress host template to enrich host with",
					"ingressHostTemplate", ingressHostTemplate,
					"functionName", functionConfig.Meta.Name)
			}

			// render host with pre-defined data
			renderedIngressHost, err := common.RenderTemplate(ingressHostTemplate, templateData)
			if err != nil {
				return errors.Wrap(err, "Failed to render ingress host template")
			}

			// try infer from attributes, if not use default 8
			hostTemplateRandomCharsLength := 8
			if hostTemplateRandomCharsLengthValue, ok := encodedIngressMap["hostTemplateRandomCharsLength"].(int); ok {
				hostTemplateRandomCharsLength = hostTemplateRandomCharsLengthValue
			}
			renderedIngressHost = p.alignIngressHostSubdomainLevel(renderedIngressHost, hostTemplateRandomCharsLength)
			if ingressHost, ingressHostFound := encodedIngressMap["host"].(string); !ingressHostFound || ingressHost == "" {
				p.Logger.DebugWith("Enriching function ingress host from template",
					"renderedIngressHost", renderedIngressHost,
					"functionName", functionConfig.Meta.Name)
				encodedIngressMap["host"] = renderedIngressHost
			}
		}
	}
	return nil
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
