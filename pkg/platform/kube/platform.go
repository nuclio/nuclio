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
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/config"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
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
	deployer       *deployer
	getter         *getter
	updater        *updater
	deleter        *deleter
	kubeconfigPath string
	consumer       *consumer
}

const Mib = 1048576

// NewPlatform instantiates a new kubernetes platform
func NewPlatform(parentLogger logger.Logger,
	kubeconfigPath string,
	containerBuilderConfiguration *containerimagebuilderpusher.ContainerBuilderConfiguration,
	platformConfiguration interface{}) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewPlatform(parentLogger, newPlatform, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// init platform
	newPlatform.Platform = newAbstractPlatform
	newPlatform.kubeconfigPath = kubeconfigPath

	// create consumer
	newPlatform.consumer, err = newConsumer(newPlatform.Logger, kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer")
	}

	// create deployer
	newPlatform.deployer, err = newDeployer(newPlatform.Logger, newPlatform.consumer, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create deployer")
	}

	// create getter
	newPlatform.getter, err = newGetter(newPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create getter")
	}

	// create deleter
	newPlatform.deleter, err = newDeleter(newPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create deleter")
	}

	// create updater
	newPlatform.updater, err = newUpdater(newPlatform.Logger, newPlatform.consumer, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create updater")
	}

	// create container builder
	if containerBuilderConfiguration != nil && containerBuilderConfiguration.Kind == "kaniko" {
		newPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewKaniko(newPlatform.Logger,
			newPlatform.consumer.kubeClientSet, containerBuilderConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create kaniko builder")
		}
	} else {

		// Default container image builder
		newPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewDocker(newPlatform.Logger, containerBuilderConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create docker builder")
		}
	}

	return newPlatform, nil
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (
	*platform.CreateFunctionResult, error) {

	var existingFunctionInstance *nuclioio.NuclioFunction
	var existingFunctionConfig *functionconfig.ConfigWithStatus

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams.Store(createFunctionOptions.FunctionConfig.Meta.GetUniqueID(), logStream)

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	if err := p.EnrichCreateFunctionOptions(createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Create function options enrichment failed")
	}

	if err := p.enrichHTTPTriggersWithServiceType(createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Failed to enrich HTTP triggers with service type")
	}

	if err := p.ValidateCreateFunctionOptions(createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Create function options validation failed")
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
				createFunctionOptions.FunctionConfig.Meta.Name)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get existing function config")
		}
	}

	// if function exists, perform some validation with new function create options
	if err := p.ValidateCreateFunctionOptionsAgainstExistingFunctionConfig(existingFunctionConfig,
		createFunctionOptions); err != nil {
		return nil, errors.Wrap(err, "Validation against existing function config failed")
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

		defaultHTTPPort := 0
		if existingFunctionInstance != nil {
			defaultHTTPPort = existingFunctionInstance.Status.HTTPPort
		}

		// create or update the function. The possible creation needs to happen here, since on cases of
		// early build failures we might get here before the function CR was created. After this point
		// it is guaranteed to be created and updated with the reported error state
		_, err = p.deployer.createOrUpdateFunction(existingFunctionInstance,
			createFunctionOptions,
			&functionconfig.Status{
				HTTPPort: defaultHTTPPort,
				State:    functionconfig.FunctionStateError,
				Message:  briefErrorsMessage,
			})
		return err
	}

	// the builder may update the configuration, so we have to create the function in the platform only after
	// the builder does that
	onAfterConfigUpdated := func(updatedFunctionConfig *functionconfig.Config) error {
		var err error

		existingFunctionInstance, err = p.getFunction(updatedFunctionConfig.Meta.Namespace,
			updatedFunctionConfig.Meta.Name)
		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}

		// if the function already exists then it either doesn't have the FunctionAnnotationSkipDeploy annotation, or it
		// was imported and has the annotation, but on this recreate it shouldn't. So the annotation should be removed.
		if existingFunctionInstance != nil {
			createFunctionOptions.FunctionConfig.Meta.RemoveSkipDeployAnnotation()
		}

		// create or update the function if it exists. If functionInstance is nil, the function will be created
		// with the configuration and status. if it exists, it will be updated with the configuration and status.
		// the goal here is for the function to exist prior to building so that it is gettable
		existingFunctionInstance, err = p.deployer.createOrUpdateFunction(existingFunctionInstance,
			createFunctionOptions,
			&functionconfig.Status{
				State: functionconfig.FunctionStateBuilding,
			})

		if err != nil {
			return errors.Wrap(err, "Failed to create/update function before build")
		}

		// indicate that the creation state has been updated
		if createFunctionOptions.CreationStateUpdated != nil {
			createFunctionOptions.CreationStateUpdated <- true
		}

		return nil
	}

	// called after function was built
	onAfterBuild := func(buildResult *platform.CreateFunctionBuildResult, buildErr error) (*platform.CreateFunctionResult, error) {

		skipDeploy := functionconfig.ShouldSkipDeploy(createFunctionOptions.FunctionConfig.Meta.Annotations)

		// after a function build (or skip-build) if the annotation FunctionAnnotationSkipBuild exists, it should be removed
		// so next time, the build will happen. (skip-deploy will be removed on next update so the controller can use the
		// annotation as well).
		createFunctionOptions.FunctionConfig.Meta.RemoveSkipBuildAnnotation()

		if buildErr != nil {

			// try to report the error
			reportingErr := reportCreationError(buildErr, "", false)
			if reportingErr != nil {
				p.Logger.ErrorWith("Creation error reporting failed",
					"reportingErr", reportingErr,
					"buildErr", buildErr)
				return nil, reportingErr
			}
			return nil, buildErr
		}

		if err := p.setScaleToZeroSpec(&createFunctionOptions.FunctionConfig.Spec); err != nil {
			return nil, errors.Wrap(err, "Failed setting scale to zero spec")
		}

		if skipDeploy {
			p.Logger.Info("Skipping function deployment")

			_, err = p.deployer.createOrUpdateFunction(existingFunctionInstance,
				createFunctionOptions,
				&functionconfig.Status{
					State: functionconfig.FunctionStateImported,
				})

			return &platform.CreateFunctionResult{
				CreateFunctionBuildResult: platform.CreateFunctionBuildResult{
					Image:                 createFunctionOptions.FunctionConfig.Spec.Image,
					UpdatedFunctionConfig: createFunctionOptions.FunctionConfig,
				},
			}, nil
		}

		createFunctionResult, updatedFunctionInstance, briefErrorsMessage, deployErr := p.deployer.deploy(existingFunctionInstance,
			createFunctionOptions)

		// update the function instance (after the deployment)
		if updatedFunctionInstance != nil {
			existingFunctionInstance = updatedFunctionInstance
		}

		if deployErr != nil {

			// try to report the error
			reportingErr := reportCreationError(deployErr, briefErrorsMessage, true)
			if reportingErr != nil {
				p.Logger.ErrorWith("Deployment error reporting failed",
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

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	functions, err := p.getter.get(p.consumer, getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	p.EnrichFunctionsWithDeployLogStream(functions)

	if err = p.enrichFunctionsWithAPIGateways(functions, getFunctionsOptions.Namespace); err != nil {

		// relevant when upgrading nuclio from a version that didn't have api-gateways to one that has
		if !strings.Contains(errors.RootCause(err).Error(),
			"the server could not find the requested resource (get nuclioapigateways.nuclio.io)") {

			return nil, errors.Wrap(err, "Failed to enrich functions with api gateways")
		}

		p.Logger.DebugWith("API Gateway CRD is not installed, skipping function api gateways enrichment",
			"err", err)
	}

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	return p.updater.update(updateFunctionOptions)
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	if err := p.validateFunctionHasNoAPIGateways(deleteFunctionOptions); err != nil {
		return errors.Wrap(err, "Failed while validating function has no api gateways")
	}

	return p.deleter.delete(p.consumer, deleteFunctionOptions)
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

	kubeNodes, err := p.consumer.kubeClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
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

// CreateProject will probably create a new project
func (p *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	newProject := nuclioio.NuclioProject{}
	p.platformProjectToProject(&createProjectOptions.ProjectConfig, &newProject)

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioProjects(createProjectOptions.ProjectConfig.Meta.Namespace).
		Create(&newProject)

	if err != nil {
		return errors.Wrap(err, "Failed to create project")
	}

	return nil
}

// UpdateProject will update a previously existing project
func (p *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	project, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioProjects(updateProjectOptions.ProjectConfig.Meta.Namespace).
		Get(updateProjectOptions.ProjectConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get projects")
	}

	updatedProject := nuclioio.NuclioProject{}
	p.platformProjectToProject(&updateProjectOptions.ProjectConfig, &updatedProject)
	project.Spec = updatedProject.Spec

	_, err = p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioProjects(updateProjectOptions.ProjectConfig.Meta.Namespace).
		Update(project)

	if err != nil {
		return errors.Wrap(err, "Failed to update project")
	}

	return nil
}

// DeleteProject will delete a previously existing project
func (p *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := p.Platform.ValidateDeleteProjectOptions(deleteProjectOptions); err != nil {
		return err
	}

	if err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioProjects(deleteProjectOptions.Meta.Namespace).
		Delete(deleteProjectOptions.Meta.Name, &metav1.DeleteOptions{}); err != nil {
		return errors.Wrapf(err,
			"Failed to delete project %s from namespace %s",
			deleteProjectOptions.Meta.Name,
			deleteProjectOptions.Meta.Namespace)
	}

	return nil
}

// GetProjects will list existing projects
func (p *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	var platformProjects []platform.Project
	var projects []nuclioio.NuclioProject

	// if identifier specified, we need to get a single NuclioProject
	if getProjectsOptions.Meta.Name != "" {

		// get specific NuclioProject CR
		Project, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			Get(getProjectsOptions.Meta.Name, metav1.GetOptions{})

		if err != nil {

			// if we didn't find the NuclioProject, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformProjects, nil
			}

			return nil, errors.Wrap(err, "Failed to get project")
		}

		projects = append(projects, *Project)

	} else {

		projectInstanceList, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			NuclioProjects(getProjectsOptions.Meta.Namespace).
			List(metav1.ListOptions{LabelSelector: ""})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list projects")
		}

		// convert []NuclioProject to []*NuclioProject
		projects = projectInstanceList.Items
	}

	// convert []nuclioio.NuclioProject -> NuclioProject
	for projectInstanceIndex := 0; projectInstanceIndex < len(projects); projectInstanceIndex++ {
		projectInstance := projects[projectInstanceIndex]

		newProject, err := platform.NewAbstractProject(p.Logger,
			p,
			platform.ProjectConfig{
				Meta: platform.ProjectMeta{
					Name:        projectInstance.Name,
					Namespace:   projectInstance.Namespace,
					Labels:      projectInstance.Labels,
					Annotations: projectInstance.Annotations,
				},
				Spec: projectInstance.Spec,
			})

		if err != nil {
			return nil, err
		}

		platformProjects = append(platformProjects, newProject)
	}

	// render it
	return platformProjects, nil
}

// CreateAPIGateway creates and deploys a new api gateway
func (p *Platform) CreateAPIGateway(createAPIGatewayOptions *platform.CreateAPIGatewayOptions) error {
	newAPIGateway := nuclioio.NuclioAPIGateway{}
	p.platformAPIGatewayToAPIGateway(&createAPIGatewayOptions.APIGatewayConfig, &newAPIGateway)

	if err := p.enrichAndValidateAPIGatewayName(&newAPIGateway); err != nil {
		return errors.Wrap(err, "Failed to validate and enrich api gateway name")
	}

	// set state to waiting for provisioning
	createAPIGatewayOptions.APIGatewayConfig.Status = platform.APIGatewayStatus{
		State: platform.APIGatewayStateWaitingForProvisioning,
	}

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Create(&newAPIGateway)
	if err != nil {
		return errors.Wrap(err, "Failed to create api gateway")
	}

	return nil
}

// UpdateAPIGateway will update a previously existing api gateway
func (p *Platform) UpdateAPIGateway(updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) error {
	apiGateway, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Get(updateAPIGatewayOptions.APIGatewayConfig.Meta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get api gateway to update")
	}

	updatedAPIGateway := nuclioio.NuclioAPIGateway{}
	p.platformAPIGatewayToAPIGateway(&updateAPIGatewayOptions.APIGatewayConfig, &updatedAPIGateway)
	apiGateway.Spec = updatedAPIGateway.Spec

	if err := p.enrichAndValidateAPIGatewayName(&updatedAPIGateway); err != nil {
		return errors.Wrap(err, "Failed to validate and enrich api gateway name")
	}

	// set api gateway state to "waitingForProvisioning", so the controller will know to update this resource
	apiGateway.Status.State = platform.APIGatewayStateWaitingForProvisioning

	_, err = p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace).
		Update(apiGateway)
	if err != nil {
		return errors.Wrap(err, "Failed to update api gateway")
	}

	return nil
}

// DeleteAPIGateway will delete a previously existing api gateway
func (p *Platform) DeleteAPIGateway(deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) error {

	if err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(deleteAPIGatewayOptions.Meta.Namespace).
		Delete(deleteAPIGatewayOptions.Meta.Name, &metav1.DeleteOptions{}); err != nil {

		return errors.Wrapf(err,
			"Failed to delete api gateway %s from namespace %s",
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
		apiGateway, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			NuclioAPIGateways(getAPIGatewaysOptions.Namespace).
			Get(getAPIGatewaysOptions.Name, metav1.GetOptions{})
		if err != nil {

			// if we didn't find the NuclioAPIGateway, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformAPIGateways, nil
			}

			return nil, errors.Wrap(err, "Failed to get api gateway")
		}

		apiGateways = append(apiGateways, *apiGateway)

	} else {

		apiGatewayInstanceList, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			NuclioAPIGateways(getAPIGatewaysOptions.Namespace).
			List(metav1.ListOptions{LabelSelector: getAPIGatewaysOptions.Labels})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to list api gateways")
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

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(createFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Create(&newFunctionEvent)

	if err != nil {
		return errors.Wrap(err, "Failed to create function event")
	}

	return nil
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	updatedFunctionEvent := nuclioio.NuclioFunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig, &updatedFunctionEvent)

	functionEvent, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Get(updateFunctionEventOptions.FunctionEventConfig.Meta.Name, metav1.GetOptions{})

	if err != nil {
		return errors.Wrap(err, "Failed to get function event")
	}

	functionEvent.Spec = updatedFunctionEvent.Spec

	_, err = p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Update(functionEvent)

	if err != nil {
		return errors.Wrap(err, "Failed to update function event")
	}

	return nil
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioFunctionEvents(deleteFunctionEventOptions.Meta.Namespace).
		Delete(deleteFunctionEventOptions.Meta.Name, &metav1.DeleteOptions{})

	if err != nil {
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
		functionEvent, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			NuclioFunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			Get(getFunctionEventsOptions.Meta.Name, metav1.GetOptions{})

		if err != nil {

			// if we didn't find the function event, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformFunctionEvents, nil
			}

			return nil, errors.Wrap(err, "Failed to get function event")
		}

		functionEvents = append(functionEvents, *functionEvent)

	} else {
		var labelSelector string
		functionName := getFunctionEventsOptions.Meta.Labels["nuclio.io/function-name"]

		// if function name specified, supply it
		if functionName != "" {
			labelSelector = fmt.Sprintf("nuclio.io/function-name=%s", functionName)
		}

		functionEventInstanceList, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			NuclioFunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			List(metav1.ListOptions{LabelSelector: labelSelector})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list function events")
		}

		// convert []NuclioFunctionEvent to []*NuclioFunctionEvent
		functionEvents = functionEventInstanceList.Items
	}

	// convert []nuclioio.NuclioFunctionEvent -> NuclioFunctionEvent
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

	// render it
	return platformFunctionEvents, nil
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
	kubeURL, err := url.Parse(p.consumer.kubeHost)
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
	namespaces, err := p.consumer.kubeClientSet.CoreV1().Namespaces().List(metav1.ListOptions{})
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

func (p *Platform) GetScaleToZeroConfiguration() (*platformconfig.ScaleToZero, error) {
	switch configType := p.Config.(type) {
	case *platformconfig.Config:
		return &configType.ScaleToZero, nil

	// FIXME: When deploying using nuctl in a kubernetes environment, it will be a kube platform, but the configuration
	// will be of type *config.Configuration which has no scale to zero configuration
	// we need to fix the platform config (p.Config) to always be of the same type (*platformconfig.Config) and not
	// passing interface{} everywhere
	case *config.Configuration:
		return nil, nil
	default:
		return nil, errors.New("Not a valid configuration instance")
	}
}

func (p *Platform) GetAllowedAuthenticationModes() ([]string, error) {
	switch configType := p.Config.(type) {
	case *platformconfig.Config:
		allowedAuthenticationModes := []string{string(ingress.AuthenticationModeNone), string(ingress.AuthenticationModeBasicAuth)}
		if len(configType.IngressConfig.AllowedAuthenticationModes) > 0 {
			allowedAuthenticationModes = configType.IngressConfig.AllowedAuthenticationModes
		}
		return allowedAuthenticationModes, nil

	// FIXME: see comment in GetScaleToZeroConfiguration
	case *config.Configuration:
		return nil, nil
	default:
		return nil, errors.New("Not a valid configuration instance")
	}
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

	return p.updater.update(&platform.UpdateFunctionOptions{
		FunctionMeta:   &function.GetConfig().Meta,
		FunctionStatus: function.GetStatus(),
	})
}

func (p *Platform) generateFunctionToAPIGatewaysMapping(namespace string) (map[string][]string, error) {
	functionToAPIGateways := map[string][]string{}

	// get all api gateways in the namespace
	apiGateways, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(namespace).
		List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list api gateways")
	}

	// iterate over all api gateways
	for _, apiGateway := range apiGateways.Items {

		// iterate over all upstreams(functions) of the api gateway
		for _, upstream := range apiGateway.Spec.Upstreams {

			// append the current api gateway to the function's api gateways list
			functionToAPIGateways[upstream.Nucliofunction.Name] =
				append(functionToAPIGateways[upstream.Nucliofunction.Name], apiGateway.Name)
		}
	}

	return functionToAPIGateways, nil
}

func (p *Platform) enrichFunctionsWithAPIGateways(functions []platform.Function, namespace string) error {
	var err error
	var functionToAPIGateways map[string][]string

	// generate function to api gateways mapping
	if functionToAPIGateways, err = p.generateFunctionToAPIGatewaysMapping(namespace); err != nil {
		return errors.Wrap(err, "Failed to get function to api gateways mapping")
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

	scaleToZeroConfiguration, err := p.GetScaleToZeroConfiguration()
	if err != nil {
		return errors.Wrap(err, "Failed getting scale to zero configuration")
	}

	if scaleToZeroConfiguration == nil {
		return nil
	}

	if scaleToZeroConfiguration.Mode == platformconfig.EnabledScaleToZeroMode {
		functionSpec.ScaleToZero = &functionconfig.ScaleToZeroSpec{
			ScaleResources: scaleToZeroConfiguration.ScaleResources,
		}
	}

	return nil
}

func (p *Platform) getFunction(namespace, name string) (*nuclioio.NuclioFunction, error) {
	p.Logger.DebugWith("Getting function",
		"namespace", namespace,
		"name", name)

	// get specific function CR
	function, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		NuclioFunctions(namespace).
		Get(name, metav1.GetOptions{})
	if err != nil {

		// if we didn't find the function, return nothing
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "Failed to get function")
	}

	p.Logger.DebugWith("Completed getting function",
		"name", name,
		"namespace", namespace,
		"function", function)

	return function, nil
}

func (p *Platform) getFunctionInstanceAndConfig(namespace, name string) (*nuclioio.NuclioFunction,
	*functionconfig.ConfigWithStatus, error) {
	functionInstance, err := p.getFunction(namespace, name)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get function")
	}

	// found function instance, return as function config
	if functionInstance != nil {
		initializedFunctionInstance, err := newFunction(p.Logger, p, functionInstance, p.consumer)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to create new function instance")
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

func (p *Platform) enrichAndValidateAPIGatewayName(apiGateway *nuclioio.NuclioAPIGateway) error {
	if apiGateway.Spec.Name == "" {
		apiGateway.Spec.Name = apiGateway.Name
	}
	if apiGateway.Spec.Name != apiGateway.Name {
		return nuclio.NewErrBadRequest("Api gateway metadata.name must match api gateway spec.name")
	}

	if common.StringInSlice(apiGateway.Spec.Name, p.ResolveReservedResourceNames()) {
		return nuclio.NewErrPreconditionFailed(fmt.Sprintf("APIGateway name %s is reserved and cannot be used.",
			apiGateway.Spec.Name))
	}

	return nil
}

func (p *Platform) enrichHTTPTriggersWithServiceType(createFunctionOptions *platform.CreateFunctionOptions) error {

	var err error

	// for backwards compatibility
	serviceType := createFunctionOptions.FunctionConfig.Spec.ServiceType
	if serviceType == "" {
		if serviceType, err = p.getDefaultServiceType(); err != nil {
			return errors.Wrap(err, "Failed getting default service type")
		}
	}

	for triggerName, trigger := range functionconfig.GetTriggersByKind(createFunctionOptions.FunctionConfig.Spec.Triggers, "http") {
		createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName] = p.enrichTriggerWithServiceType(createFunctionOptions,
			trigger,
			serviceType)
	}

	return nil
}

func (p *Platform) validateFunctionHasNoAPIGateways(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	var functionToAPIGateways map[string][]string
	var err error

	// generate function to api gateways mapping
	if functionToAPIGateways, err = p.generateFunctionToAPIGatewaysMapping(deleteFunctionOptions.FunctionConfig.Meta.Namespace); err != nil {
		return errors.Wrap(err, "Failed to get function to api gateways mapping")
	}

	if len(functionToAPIGateways[deleteFunctionOptions.FunctionConfig.Meta.Name]) > 0 {
		return platform.ErrFunctionIsUsedByAPIGateways
	}

	return nil
}

func (p *Platform) enrichTriggerWithServiceType(createFunctionOptions *platform.CreateFunctionOptions,
	trigger functionconfig.Trigger,
	serviceType v1.ServiceType) functionconfig.Trigger {

	if trigger.Attributes == nil {
		trigger.Attributes = map[string]interface{}{}
	}

	if triggerServiceType, serviceTypeExists := trigger.Attributes["serviceType"]; !serviceTypeExists || triggerServiceType == "" {

		p.Logger.DebugWith("Enriching function HTTP trigger with service type",
			"functionName", createFunctionOptions.FunctionConfig.Meta.Name,
			"triggerName", trigger.Name,
			"serviceType", serviceType)
		trigger.Attributes["serviceType"] = serviceType
	}

	return trigger
}

func (p *Platform) getDefaultServiceType() (v1.ServiceType, error) {
	switch configType := p.Config.(type) {
	case *platformconfig.Config:
		return configType.Kube.DefaultServiceType, nil

	// FIXME: see comment in GetScaleToZeroConfiguration
	// for now, if nuctl - return the constant default
	case *config.Configuration:
		nuctlDefaultServiceType := v1.ServiceType(
			common.GetEnvOrDefaultString("NUCTL_DEFAULT_SERVICE_TYPE", ""))

		if nuctlDefaultServiceType == "" {
			nuctlDefaultServiceType = platformconfig.DefaultServiceType
		}

		return nuctlDefaultServiceType, nil
	default:
		return "", errors.New("Not a valid configuration instance")
	}
}
