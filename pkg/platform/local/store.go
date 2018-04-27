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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const (
	volumeName        = "nuclio-local-storage"
	containerName     = "nuclio-local-storage-reader"
	baseDir           = "/etc/nuclio/store"
	functionsDir      = baseDir + "/functions"
	projectsDir       = baseDir + "/projects"
	functionEventsDir = baseDir + "/function-events"
)

type store struct {
	logger       logger.Logger
	dockerClient dockerclient.Client
	platform     platform.Platform
}

func newStore(parentLogger logger.Logger,
	platform platform.Platform,
	dockerClient dockerclient.Client) (*store, error) {
	return &store{
		logger:       parentLogger.GetChild("store"),
		dockerClient: dockerClient,
		platform:     platform,
	}, nil
}

//
// Project
//

func (s *store) createOrUpdateProject(projectConfig *platform.ProjectConfig) error {
	resourcePath := s.getResourcePath(projectsDir, projectConfig.Meta.Namespace, projectConfig.Meta.Name)

	// write the contents to that file name at the appropriate path
	return s.serializeAndWriteFileContents(resourcePath, projectConfig)
}

func (s *store) getProjects(projectMeta *platform.ProjectMeta) ([]platform.Project, error) {
	var projects []platform.Project

	rowHandler := func(row []byte) error {
		newProject := platform.AbstractProject{}

		// unmarshal the row
		if err := json.Unmarshal(row, &newProject.ProjectConfig); err != nil {
			return errors.Wrap(err, "Failed to unmarshal project")
		}

		projects = append(projects, &newProject)

		return nil
	}

	err := s.getResources(projectsDir, projectMeta.Namespace, projectMeta.Name, rowHandler)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	return projects, nil
}

func (s *store) deleteProject(projectMeta *platform.ProjectMeta) error {
	return s.deleteResource(projectsDir, projectMeta.Namespace, projectMeta.Name)
}

//
// Function events
//

func (s *store) createOrUpdateFunctionEvent(functionEventConfig *platform.FunctionEventConfig) error {
	resourcePath := s.getResourcePath(functionEventsDir, functionEventConfig.Meta.Namespace, functionEventConfig.Meta.Name)

	// write the contents to that file name at the appropriate path
	return s.serializeAndWriteFileContents(resourcePath, functionEventConfig)
}

func (s *store) getFunctionEvents(functionEventMeta *platform.FunctionEventMeta) ([]platform.FunctionEvent, error) {
	var functionEvents []platform.FunctionEvent

	// get function filter
	functionName := functionEventMeta.Labels["nuclio.io/function-name"]

	rowHandler := func(row []byte) error {
		newFunctionEvent := platform.AbstractFunctionEvent{}

		// unmarshal the row
		if err := json.Unmarshal(row, &newFunctionEvent.FunctionEventConfig); err != nil {
			return errors.Wrap(err, "Failed to unmarshal function event")
		}

		// if a filter is defined and the event has a function name label which does not match
		// the desired filter, skip
		if functionName != "" &&
			newFunctionEvent.GetConfig().Meta.Labels != nil &&
			functionName != newFunctionEvent.GetConfig().Meta.Labels["nuclio.io/function-name"] {
			return nil
		}

		functionEvents = append(functionEvents, &newFunctionEvent)

		return nil
	}

	err := s.getResources(functionEventsDir, functionEventMeta.Namespace, functionEventMeta.Name, rowHandler)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functionEvents")
	}

	return functionEvents, nil
}

func (s *store) deleteFunctionEvent(functionEventMeta *platform.FunctionEventMeta) error {
	return s.deleteResource(functionEventsDir, functionEventMeta.Namespace, functionEventMeta.Name)
}

//
// Function (used only for the period before there's a docker container to represent the function)
//

func (s *store) createOrUpdateFunction(functionConfig *functionconfig.ConfigWithStatus) error {
	resourcePath := s.getResourcePath(functionsDir, functionConfig.Meta.Namespace, functionConfig.Meta.Name)

	// write the contents to that file name at the appropriate path
	return s.serializeAndWriteFileContents(resourcePath, functionConfig)
}

func (s *store) getFunctions(functionMeta *functionconfig.Meta) ([]platform.Function, error) {
	var functions []platform.Function

	rowHandler := func(row []byte) error {
		configWithStatus := functionconfig.ConfigWithStatus{}

		// unmarshal the row
		if err := json.Unmarshal(row, &configWithStatus); err != nil {
			return errors.Wrap(err, "Failed to unmarshal function")
		}

		newFunction, err := newFunction(s.logger, s.platform, &configWithStatus.Config, &configWithStatus.Status)
		if err != nil {
			return errors.Wrap(err, "Failed to create function")
		}

		functions = append(functions, newFunction)

		return nil
	}

	err := s.getResources(functionsDir, functionMeta.Namespace, functionMeta.Name, rowHandler)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	return functions, nil
}

func (s *store) deleteFunction(functionMeta *functionconfig.Meta) error {
	return s.deleteResource(functionsDir, functionMeta.Namespace, functionMeta.Name)
}

//
// Implementation
//

func (s *store) serializeAndWriteFileContents(resourcePath string, resourceConfig interface{}) error {

	// serialize the resource to json
	serializedResourceConfig, err := json.Marshal(resourceConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to serialize resource config")
	}

	return s.writeFileContents(resourcePath, serializedResourceConfig)
}

func (s *store) getResourcePath(resourceDir string, resourceNamespace string, resourceName string) string {
	return path.Join(s.getResourceNamespaceDir(resourceDir, resourceNamespace), resourceName+".json")
}

func (s *store) getResourceNamespaceDir(resourceDir string, resourceNamespace string) string {
	return path.Join(resourceDir, resourceNamespace)
}

func (s *store) getResources(resourceDir string,
	resourceNamespace string,
	resourceName string,
	rowHandler func([]byte) error) error {

	var commandStdout, resourcePath string

	// if the request is for a single resource, get that file
	if resourceName != "" {
		resourcePath = s.getResourcePath(resourceDir, resourceNamespace, resourceName)
	} else {
		resourcePath = path.Join(s.getResourceNamespaceDir(resourceDir, resourceNamespace), "*")
	}

	commandStdout, _, err := s.runCommand(nil, `/bin/sh -c "/bin/cat %s"`, resourcePath)
	if err != nil {

		// if there error indicates that there's no such file - that means nothing was created yet
		cause := errors.Cause(err)
		if cause != nil && strings.Contains(cause.Error(), "No such file") {
			return nil
		}

		return errors.Wrap(err, "Failed to run cat command")
	}

	// iterate over the output line by line
	scanner := bufio.NewScanner(strings.NewReader(commandStdout))

	// set a 2MB scan buffer (this is the max resource size)
	maxResourceLen := 2 * 1024 * 1024
	readBuffer := make([]byte, maxResourceLen)
	scanner.Buffer(readBuffer, maxResourceLen)

	for scanner.Scan() {

		// decode contents from base64
		decodedRow, err := base64.StdEncoding.DecodeString(scanner.Text())
		if err != nil {
			return errors.Wrap(err, "Row contains invalid base64")
		}

		// get row contents
		if err := rowHandler(decodedRow); err != nil {
			return errors.Wrap(err, "Row handler returned error")
		}
	}

	return nil
}

func (s *store) writeFileContents(filePath string, contents []byte) error {
	s.logger.DebugWith("Writing file contents", "path", filePath, "contents", string(contents))

	// get the file dir
	fileDir := path.Dir(filePath)

	// set NUCLIO_CONTENTS as base64 encoded value
	env := map[string]string{"NUCLIO_CONTENTS": base64.StdEncoding.EncodeToString(contents)}

	// generate a command
	_, _, err := s.runCommand(env,
		`/bin/sh -c "mkdir -p %s && /bin/printenv NUCLIO_CONTENTS > %s"`,
		fileDir,
		filePath)

	return err
}

func (s *store) runCommand(env map[string]string, format string, args ...interface{}) (string, string, error) {
	var commandStdout, commandStderr string

	// format the command to a string
	command := fmt.Sprintf(format, args...)

	// execute a command within a container called `containerName`. if it fails because the container doesn't exist,
	// try to run the container. if it fails because it's already created, run exec again (could be that multiple
	// calls to getResources occurred at the same time). Repeat this 3 times
	for attemptIdx := 0; attemptIdx < 3; attemptIdx++ {
		commandStdout = ""
		commandStderr = ""

		err := s.dockerClient.ExecInContainer(containerName, &dockerclient.ExecOptions{
			Command: command,
			Stdout:  &commandStdout,
			Stderr:  &commandStderr,
			Env:     env,
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
			!strings.Contains(err.Error(), "No such container") {
			return "", "", errors.Wrapf(err, "Failed to execute command: %s", command)
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
			return "", "", errors.Wrap(err, "Failed to run container with storage volume")
		}
	}

	return commandStdout, commandStderr, nil
}

func (s *store) deleteResource(resourceDir string, resourceNamespace string, resourceName string) error {
	resourcePath := s.getResourcePath(resourceDir, resourceNamespace, resourceName)

	// remove the file
	_, _, err := s.runCommand(nil, "/bin/rm %s", resourcePath)

	// if there error indicates that there's no such file - that means nothing was created yet
	cause := errors.Cause(err)
	if cause != nil && strings.Contains(cause.Error(), "No such file") {
		return nuclio.NewErrNotFound(fmt.Sprintf("Could not find resource %s", resourceName))
	}

	return err
}
