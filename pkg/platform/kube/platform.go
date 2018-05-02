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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// NewPlatform instantiates a new kubernetes platform
func NewPlatform(parentLogger logger.Logger, kubeconfigPath string) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := abstract.NewPlatform(parentLogger, newPlatform)
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

	return newPlatform, nil
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) CreateFunction(createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {
	var existingFunctionInstance *nuclioio.Function

	// wrap logger
	logStream, err := abstract.NewLogStream("deployer", nucliozap.InfoLevel, createFunctionOptions.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create log stream")
	}

	// save the log stream for the name
	p.DeployLogStreams[createFunctionOptions.FunctionConfig.Meta.GetUniqueID()] = logStream

	// replace logger
	createFunctionOptions.Logger = logStream.GetLogger()

	reportCreationError := func(creationError error) error {
		createFunctionOptions.Logger.WarnWith("Create function failed failed, setting function status",
			"err", creationError)

		errorStack := bytes.Buffer{}
		errors.PrintErrorStack(&errorStack, creationError, 20)

		// post logs and error
		return p.UpdateFunction(&platform.UpdateFunctionOptions{
			FunctionMeta: &createFunctionOptions.FunctionConfig.Meta,
			FunctionStatus: &functionconfig.Status{
				State:   functionconfig.FunctionStateError,
				Message: errorStack.String(),
			},
		})
	}

	// the builder will may update configuration, so we have to create the function in the platform only after
	// the builder does that
	onAfterConfigUpdated := func(updatedFunctionConfig *functionconfig.Config) error {
		var err error

		createFunctionOptions.Logger.DebugWith("Getting existing function",
			"namespace", updatedFunctionConfig.Meta.Namespace,
			"name", updatedFunctionConfig.Meta.Name)

		existingFunctionInstance, err = p.getFunction(updatedFunctionConfig.Meta.Namespace,
			updatedFunctionConfig.Meta.Name)

		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}

		createFunctionOptions.Logger.DebugWith("Completed getting existing function",
			"found", existingFunctionInstance)

		// create or update the function if existing. FunctionInstance is nil, the function will be created
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

	onAfterBuild := func(buildResult *platform.CreateFunctionBuildResult, buildErr error) (*platform.CreateFunctionResult, error) {
		if buildErr != nil {

			// try to report the error
			reportCreationError(buildErr) // nolint: errcheck

			return nil, buildErr
		}

		createFunctionResult, deployErr := p.deployer.deploy(existingFunctionInstance, createFunctionOptions)
		if deployErr != nil {

			// try to report the error
			reportCreationError(deployErr) // nolint: errcheck

			return nil, deployErr
		}

		return createFunctionResult, nil
	}

	// do the deploy in the abstract base class
	return p.HandleDeployFunction(createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	functions, err := p.getter.get(p.consumer, getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	// iterate over functions and enrich with deploy logs
	for _, function := range functions {

		// enrich with build logs
		if deployLogStream, exists := p.DeployLogStreams[function.GetConfig().Meta.GetUniqueID()]; exists {
			deployLogStream.ReadLogs(nil, &function.GetStatus().Logs)
		}
	}

	return functions, nil
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	return p.updater.update(updateFunctionOptions)
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	return p.deleter.delete(p.consumer, deleteFunctionOptions)
}

func IsInCluster() bool {
	return len(os.Getenv("KUBERNETES_SERVICE_HOST")) != 0 && len(os.Getenv("KUBERNETES_SERVICE_PORT")) != 0
}

func GetKubeconfigPath(platformConfiguration interface{}) string {
	var kubeconfigPath string

	// if kubeconfig is passed in the options, use that
	if platformConfiguration != nil {

		// it might not be a kube configuration
		if _, ok := platformConfiguration.(*Configuration); ok {
			kubeconfigPath = platformConfiguration.(*Configuration).KubeconfigPath
		}
	}

	// do we still not have a kubeconfig path? try environment variable
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
	}

	// still don't? try looking @ home directory
	if kubeconfigPath == "" {
		kubeconfigPath = getKubeconfigFromHomeDir()
	}

	return kubeconfigPath
}

// GetName returns the platform name
func (p *Platform) GetName() string {
	return "kube"
}

// GetNodes returns a slice of nodes currently in the cluster
func (p *Platform) GetNodes() ([]platform.Node, error) {
	var platformNodes []platform.Node

	kubeNodes, err := p.consumer.kubeClientSet.CoreV1().Nodes().List(meta_v1.ListOptions{})
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
	newProject := nuclioio.Project{}
	p.platformProjectToProject(&createProjectOptions.ProjectConfig, &newProject)

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		Projects(createProjectOptions.ProjectConfig.Meta.Namespace).
		Create(&newProject)

	if err != nil {
		return errors.Wrap(err, "Failed to create project")
	}

	return nil
}

// UpdateProject will update a previously existing project
func (p *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	updatedProject := nuclioio.Project{}
	p.platformProjectToProject(&updateProjectOptions.ProjectConfig, &updatedProject)

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		Projects(updateProjectOptions.ProjectConfig.Meta.Namespace).
		Update(&updatedProject)

	if err != nil {
		return errors.Wrap(err, "Failed to update project")
	}

	return nil
}

// DeleteProject will delete a previously existing project
func (p *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Namespace: deleteProjectOptions.Meta.Namespace,
		Labels:    fmt.Sprintf("nuclio.io/project-name=%s", deleteProjectOptions.Meta.Name),
	}

	functions, err := p.GetFunctions(getFunctionsOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to get functions")
	}

	if len(functions) != 0 {
		return fmt.Errorf("Project has %d functions, can't delete", len(functions))
	}

	err = p.consumer.nuclioClientSet.NuclioV1beta1().
		Projects(deleteProjectOptions.Meta.Namespace).
		Delete(deleteProjectOptions.Meta.Name, &meta_v1.DeleteOptions{})

	if err != nil {
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
	var projects []nuclioio.Project

	// if identifier specified, we need to get a single Project
	if getProjectsOptions.Meta.Name != "" {

		// get specific Project CR
		Project, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			Projects(getProjectsOptions.Meta.Namespace).
			Get(getProjectsOptions.Meta.Name, meta_v1.GetOptions{})

		if err != nil {

			// if we didn't find the Project, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformProjects, nil
			}

			return nil, errors.Wrap(err, "Failed to get project")
		}

		projects = append(projects, *Project)

	} else {

		projectInstanceList, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			Projects(getProjectsOptions.Meta.Namespace).
			List(meta_v1.ListOptions{LabelSelector: ""})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list projects")
		}

		// convert []Project to []*Project
		projects = projectInstanceList.Items
	}

	// convert []nuclioio.Project -> Project
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

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (p *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	newFunctionEvent := nuclioio.FunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&createFunctionEventOptions.FunctionEventConfig, &newFunctionEvent)

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		FunctionEvents(createFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Create(&newFunctionEvent)

	if err != nil {
		return errors.Wrap(err, "Failed to create function event")
	}

	return nil
}

// UpdateFunctionEvent will update a previously existing function event
func (p *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	updatedFunctionEvent := nuclioio.FunctionEvent{}
	p.platformFunctionEventToFunctionEvent(&updateFunctionEventOptions.FunctionEventConfig, &updatedFunctionEvent)

	_, err := p.consumer.nuclioClientSet.NuclioV1beta1().
		FunctionEvents(updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace).
		Update(&updatedFunctionEvent)

	if err != nil {
		return errors.Wrap(err, "Failed to update function event")
	}

	return nil
}

// DeleteFunctionEvent will delete a previously existing function event
func (p *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	err := p.consumer.nuclioClientSet.NuclioV1beta1().
		FunctionEvents(deleteFunctionEventOptions.Meta.Namespace).
		Delete(deleteFunctionEventOptions.Meta.Name, &meta_v1.DeleteOptions{})

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
	var functionEvents []nuclioio.FunctionEvent

	// if identifier specified, we need to get a single function event
	if getFunctionEventsOptions.Meta.Name != "" {

		// get specific function event CR
		functionEvent, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			FunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			Get(getFunctionEventsOptions.Meta.Name, meta_v1.GetOptions{})

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
			FunctionEvents(getFunctionEventsOptions.Meta.Namespace).
			List(meta_v1.ListOptions{LabelSelector: labelSelector})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list function events")
		}

		// convert []FunctionEvent to []*FunctionEvent
		functionEvents = functionEventInstanceList.Items
	}

	// convert []nuclioio.FunctionEvent -> FunctionEvent
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

func getKubeconfigFromHomeDir() string {
	homeDir, err := homedir.Dir()
	if err != nil {
		return ""
	}

	homeKubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// if the file exists @ home, use it
	_, err = os.Stat(homeKubeConfigPath)
	if err == nil {
		return homeKubeConfigPath
	}

	return ""
}

func (p *Platform) getFunction(namespace string, name string) (*nuclioio.Function, error) {

	// get specific function CR
	function, err := p.consumer.nuclioClientSet.NuclioV1beta1().Functions(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {

		// if we didn't find the function, return nothing
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "Failed to get function")
	}

	return function, nil
}

func (p *Platform) platformProjectToProject(platformProject *platform.ProjectConfig, project *nuclioio.Project) {
	project.Name = platformProject.Meta.Name
	project.Namespace = platformProject.Meta.Namespace
	project.Labels = platformProject.Meta.Labels
	project.Annotations = platformProject.Meta.Annotations
	project.Spec = platformProject.Spec
}

func (p *Platform) platformFunctionEventToFunctionEvent(platformFunctionEvent *platform.FunctionEventConfig, functionEvent *nuclioio.FunctionEvent) {
	functionEvent.Name = platformFunctionEvent.Meta.Name
	functionEvent.Namespace = platformFunctionEvent.Meta.Namespace
	functionEvent.Labels = platformFunctionEvent.Meta.Labels
	functionEvent.Annotations = platformFunctionEvent.Meta.Annotations
	functionEvent.Spec = platformFunctionEvent.Spec // deep copy instead?
}
