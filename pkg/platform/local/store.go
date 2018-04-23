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

package local

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
	"github.com/rs/xid"
)

const (
	volumeName         = "nuclio-local-storage"
	containerName      = "nuclio-local-storage-reader"
	baseDir            = "/etc/nuclio/store"
	processorConfigDir = baseDir + "/processor-configs"
	projectsDir        = baseDir + "/projects"
	functionEventsDir  = baseDir + "/function-events"
)

type store struct {
	logger       logger.Logger
	dockerClient dockerclient.Client
}

func newStore(parentLogger logger.Logger, dockerClient dockerclient.Client) (*store, error) {
	return &store{
		logger:       parentLogger.GetChild("store"),
		dockerClient: dockerClient,
	}, nil
}

func (s *store) createProcessorConfigFile(contents []byte) (string, error) {

	// generate random file name
	processorConfigFileName := "processor-config-" + xid.New().String()

	// write the contents to that file name at the appropriate path path
	processorConfigFilePath := path.Join(processorConfigDir, processorConfigFileName)

	err := s.writeFileContents(processorConfigFilePath, contents)
	if err != nil {
		return "", err
	}

	return processorConfigFilePath, err
}

func (s *store) createOrUpdateResource(resourceConfig interface{}) error {

	// verify resource type
	if err := s.verifyResourceConfig(resourceConfig); err != nil {
		return errors.Wrap(err, "Invalid resource type")
	}

	// serialize the resource to json
	serializedResourceConfig, err := json.Marshal(resourceConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to serialize resource config")
	}

	resourceMeta := s.resourceConfigToMeta(resourceConfig)

	// write the contents to that file name at the appropriate path
	return s.writeFileContents(s.resourceMetaToPath(resourceMeta), serializedResourceConfig)
}

func (s *store) deleteResource(resourceMeta interface{}) error {

	// verify resource type
	if err := s.verifyResourceMeta(resourceMeta); err != nil {
		return errors.Wrap(err, "Invalid resource type")
	}

	// generate a command
	command := fmt.Sprintf(`/bin/rm %s`, s.resourceMetaToPath(resourceMeta))

	// run in docker, volumizing
	_, err := s.dockerClient.RunContainer("alpine:3.6", &dockerclient.RunOptions{
		Volumes:          map[string]string{volumeName: baseDir},
		Remove:           true,
		Command:          command,
		Attach:           true,
		ImageMayNotExist: true,
	})

	return err
}

func (s *store) getProjects(projectMeta *platform.ProjectMeta) ([]platform.Project, error) {
	resources, err := s.getResources(projectMeta)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	var projects []platform.Project

	for _, resource := range resources {
		project, ok := resource.(*platform.AbstractProject)
		if !ok {
			return nil, errors.New("Failed to type assert resource to project")
		}

		projects = append(projects, project)
	}

	return projects, nil
}

func (s *store) getFunctionEvents(functionEventMeta *platform.FunctionEventMeta) ([]platform.FunctionEvent, error) {
	resources, err := s.getResources(functionEventMeta)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function events")
	}

	var functionEvents []platform.FunctionEvent

	// get function filter
	functionName := functionEventMeta.Labels["nuclio.io/function-name"]

	for _, resource := range resources {
		functionEvent, ok := resource.(*platform.AbstractFunctionEvent)
		if !ok {
			return nil, errors.New("Failed to type assert resource to function event")
		}

		// if a filter is defined and the event has a function name label which does not match
		// the desired filter, skip
		if functionName != "" &&
			functionEvent.GetConfig().Meta.Labels != nil &&
			functionName != functionEvent.GetConfig().Meta.Labels["nuclio.io/function-name"] {
			continue
		}

		functionEvents = append(functionEvents, functionEvent)
	}

	return functionEvents, nil
}

func (s *store) getResources(resourceMeta interface{}) ([]interface{}, error) {

	// verify resource type
	if err := s.verifyResourceMeta(resourceMeta); err != nil {
		return nil, errors.Wrap(err, "Invalid resource type")
	}

	var resourcePath string
	var commandStdout, commandStderr string

	resourceName := s.resourceMetaToName(resourceMeta)

	// if the request is for a single resource, get that file
	if resourceName != "" {
		resourcePath = s.resourceMetaToPath(resourceMeta)
	} else {
		resourcePath = path.Join(s.resourceMetaToNamespaceDir(resourceMeta), "*")
	}

	// execute a cat within a container called `containerName`. if it fails because the container doesn't exist,
	// try to run the container. if it fails because it's already created, run exec again (could be that multiple
	// calls to getResources occurred at the same time). Repeat this 3 times
	for attemptIdx := 0; attemptIdx < 3; attemptIdx++ {
		commandStdout = ""
		commandStderr = ""

		err := s.dockerClient.ExecInContainer(containerName, &dockerclient.ExecOptions{
			Command: fmt.Sprintf(`/bin/sh -c "/bin/cat %s"`, resourcePath),
			Stdout:  &commandStdout,
			Stderr:  &commandStderr,
		})

		// if command succeeded, we're done. commandStdout holds the content of the requested file
		if err == nil {
			break
		}

		// if there was an error
		// and it wasn't because the file wasn't created yet
		// and it wasn't because the container doesn't exist
		// return the error
		if err != nil &&
			!strings.Contains(err.Error(), "No such file or directory") &&
			!strings.Contains(err.Error(), "No such container") {
			return nil, errors.Wrap(err, "Failed to execute cat command")
		}

		// run a container that simply volumizes the volume with the storage and sleeps for 6 hours
		_, err = s.dockerClient.RunContainer("alpine:3.6", &dockerclient.RunOptions{
			Volumes:          map[string]string{volumeName: baseDir},
			Remove:           true,
			Command:          `/bin/sh -c "/bin/sleep 6h"`,
			Stdout:           &commandStdout,
			ImageMayNotExist: true,
			ContainerName:    containerName,
		})

		// if we failed and the error is not that it already exists, return the error
		if err != nil &&
			!strings.Contains(err.Error(), "is already in use by container") {
			return nil, errors.Wrap(err, "Failed to run container with storage volume")
		}
	}

	var resources []interface{}

	// iterate over the output line by line
	scanner := bufio.NewScanner(strings.NewReader(commandStdout))
	for scanner.Scan() {

		// get row contents
		rowContents := scanner.Text()

		// try to unmarshal into an empty config of the resource's type
		resourceConfig, err := s.unmarshalResourceConfig(resourceMeta, rowContents)
		if err != nil {
			continue
		}

		// create an abstract resource
		newAbstractResource, err := s.resourceMetaToNewAbstractResource(resourceMeta, resourceConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create abstract resource")
		}

		// add to resources
		resources = append(resources, newAbstractResource)
	}

	return resources, nil
}

func (s *store) writeFileContents(filePath string, contents []byte) error {
	s.logger.DebugWith("Writing file contents", "path", filePath, "contents", contents)

	// get the file dir
	fileDir := path.Dir(filePath)

	// generate a command
	command := fmt.Sprintf(`/bin/sh -c "mkdir -p %s && /bin/printenv NUCLIO_CONTENTS > %s"`, fileDir, filePath)

	// run in docker, volumizing
	_, err := s.dockerClient.RunContainer("alpine:3.6", &dockerclient.RunOptions{
		Volumes:          map[string]string{volumeName: baseDir},
		Remove:           true,
		Command:          command,
		Env:              map[string]string{"NUCLIO_CONTENTS": string(contents)},
		Attach:           true,
		ImageMayNotExist: true,
	})

	return err
}

func (s *store) verifyResourceConfig(resourceConfig interface{}) error {
	switch resourceType := resourceConfig.(type) {
	case *platform.ProjectConfig, *platform.FunctionEventConfig:
		return nil
	default:
		return errors.Errorf("unsupported type: %T", resourceType)
	}
}

func (s *store) verifyResourceMeta(resourceMeta interface{}) error {
	switch resourceType := resourceMeta.(type) {
	case *platform.ProjectMeta, *platform.FunctionEventMeta:
		return nil
	default:
		return errors.Errorf("unsupported type: %T", resourceType)
	}
}

func (s *store) resourceConfigToMeta(resourceConfig interface{}) interface{} {
	switch resourceConfig := resourceConfig.(type) {
	case *platform.ProjectConfig:
		return &resourceConfig.Meta
	case *platform.FunctionEventConfig:
		return &resourceConfig.Meta
	}

	return nil
}

func (s *store) resourceMetaToName(resourceMeta interface{}) string {
	switch resourceMeta := resourceMeta.(type) {
	case *platform.ProjectMeta:
		return resourceMeta.Name
	case *platform.FunctionEventMeta:
		return resourceMeta.Name
	}

	return ""
}

func (s *store) unmarshalResourceConfig(resourceMeta interface{}, marshalledConfig string) (interface{}, error) {
	switch resourceMeta.(type) {
	case *platform.ProjectMeta:
		projectConfig := platform.ProjectConfig{}

		// try to unmarshal
		err := json.Unmarshal([]byte(marshalledConfig), &projectConfig)
		if err != nil {
			s.logger.DebugWith("Ignoring project", "contents", marshalledConfig)
			return nil, errors.Wrap(err, "Failed to unmarshal project")
		}

		return projectConfig, nil
	case *platform.FunctionEventMeta:
		functionEventConfig := platform.FunctionEventConfig{}

		// try to unmarshal
		err := json.Unmarshal([]byte(marshalledConfig), &functionEventConfig)
		if err != nil {
			s.logger.DebugWith("Ignoring function event", "contents", marshalledConfig)
			return nil, errors.Wrap(err, "Failed to unmarshal function event")
		}

		return functionEventConfig, nil
	}

	return nil, nil
}

func (s *store) resourceMetaToNewAbstractResource(resourceMeta interface{}, resourceConfig interface{}) (interface{}, error) {
	switch resourceMeta.(type) {
	case *platform.ProjectMeta:
		projectConfig := resourceConfig.(platform.ProjectConfig)

		newAbstractProject, err := platform.NewAbstractProject(s.logger, nil, projectConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create new abstract project")
		}

		return newAbstractProject, nil

	case *platform.FunctionEventMeta:
		functionEventConfig := resourceConfig.(platform.FunctionEventConfig)

		newAbstractFunctionEvent, err := platform.NewAbstractFunctionEvent(s.logger, nil, functionEventConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create new abstract function event")
		}

		return newAbstractFunctionEvent, nil
	}

	return nil, nil
}

func (s *store) resourceMetaToPath(resourceMeta interface{}) string {
	return path.Join(s.resourceMetaToNamespaceDir(resourceMeta), s.resourceMetaToName(resourceMeta)+".json")
}

func (s *store) resourceMetaToNamespaceDir(resourceMeta interface{}) string {
	switch resourceMeta := resourceMeta.(type) {
	case *platform.ProjectMeta:
		return path.Join(projectsDir, resourceMeta.Namespace)
	case *platform.FunctionEventMeta:
		return path.Join(functionEventsDir, resourceMeta.Namespace)
	}

	return ""
}
