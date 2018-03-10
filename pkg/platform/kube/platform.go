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
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/logger"
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

	// the builder will first create or update
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

		return nil
	}

	onAfterBuild := func(buildResult *platform.CreateFunctionBuildResult, buildErr error) (*platform.CreateFunctionResult, error) {

		if buildErr != nil {
			createFunctionOptions.Logger.WarnWith("Build failed, setting function status", "err", buildErr)

			errorStack := bytes.Buffer{}
			errors.PrintErrorStack(&errorStack, buildErr, 20)

			// post logs and error
			p.UpdateFunction(&platform.UpdateFunctionOptions{
				FunctionMeta: &buildResult.UpdatedFunctionConfig.Meta,
				FunctionStatus: &functionconfig.Status{
					State:   functionconfig.FunctionStateError,
					Message: errorStack.String(),
				},
			})
		}

		return p.deployer.deploy(existingFunctionInstance, createFunctionOptions)
	}

	// do the deploy in the abstract base class
	return p.HandleDeployFunction(createFunctionOptions, onAfterConfigUpdated, onAfterBuild)
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	return p.getter.get(p.consumer, getFunctionsOptions)
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

// CreateProject will deploy a processor image to the platform (optionally building it, if source is provided)
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

// UpdateProject will update a previously deployed function
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

// DeleteProject will delete a previously deployed function
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

// GetProjects will invoke a previously deployed Project
func (p *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	var platformProjects []platform.Project
	var Projects []nuclioio.Project

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

			return nil, errors.Wrap(err, "Failed to get Project")
		}

		Projects = append(Projects, *Project)

	} else {

		ProjectInstanceList, err := p.consumer.nuclioClientSet.NuclioV1beta1().
			Projects(getProjectsOptions.Meta.Namespace).
			List(meta_v1.ListOptions{LabelSelector: ""})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list Projects")
		}

		// convert []Project to []*Project
		Projects = ProjectInstanceList.Items
	}

	// convert []nuclioio.Project -> Project
	for ProjectInstanceIndex := 0; ProjectInstanceIndex < len(Projects); ProjectInstanceIndex++ {
		ProjectInstance := Projects[ProjectInstanceIndex]

		newProject, err := platform.NewAbstractProject(p.Logger,
			p,
			platform.ProjectConfig{
				Meta: platform.ProjectMeta{
					Name:        ProjectInstance.Name,
					Namespace:   ProjectInstance.Namespace,
					Labels:      ProjectInstance.Labels,
					Annotations: ProjectInstance.Annotations,
				},
				Spec: platform.ProjectSpec{
					DisplayName: ProjectInstance.Spec.DisplayName,
					Description: ProjectInstance.Spec.Description,
				},
			})

		if err != nil {
			return nil, err
		}

		platformProjects = append(platformProjects, newProject)
	}

	// render it
	return platformProjects, nil
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
	project.Spec.DisplayName = platformProject.Spec.DisplayName
	project.Spec.Description = platformProject.Spec.Description
}
