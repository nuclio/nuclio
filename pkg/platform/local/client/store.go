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

package client

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nuclio "github.com/nuclio/nuclio-sdk-go"
)

const (
	volumeName        = "nuclio-local-storage"
	containerName     = "nuclio-local-storage-reader"
	baseDir           = "/etc/nuclio/store"
	functionsDir      = baseDir + "/functions"
	projectsDir       = baseDir + "/projects"
	functionEventsDir = baseDir + "/function-events"
)

type Store struct {
	logger       logger.Logger
	dockerClient dockerclient.Client
	platform     platform.Platform
	imageName    string
}

func NewStore(parentLogger logger.Logger,
	platform platform.Platform,
	dockerClient dockerclient.Client,
	imageName string) (*Store, error) {
	return &Store{
		logger:       parentLogger.GetChild("store"),
		dockerClient: dockerClient,
		platform:     platform,
		imageName:    imageName,
	}, nil
}

//
// Project
//

func (s *Store) CreateOrUpdateProject(projectConfig *platform.ProjectConfig) error {
	resourcePath := s.getResourcePath(projectsDir, projectConfig.Meta.Namespace, projectConfig.Meta.Name)

	// populate status
	now := time.Now()
	projectConfig.Status.UpdatedAt = &now

	// write the contents to that file name at the appropriate path
	return s.serializeAndWriteFileContents(resourcePath, projectConfig)
}

func (s *Store) GetProjects(projectMeta *platform.ProjectMeta) ([]platform.Project, error) {
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

	if err := s.getResources(projectsDir, projectMeta.Namespace, projectMeta.Name, rowHandler); err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	return projects, nil
}

func (s *Store) DeleteProject(ctx context.Context, projectMeta *platform.ProjectMeta) error {
	functions, err := s.GetProjectFunctions(&platform.GetFunctionsOptions{
		Namespace: projectMeta.Namespace,
		Labels:    fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName, projectMeta.Name),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to get project functions")
	}

	// NOTE: functions delete their related function events
	deleteFunctionsErrGroup, deleteFunctionsErrGroupCtx := errgroup.WithContext(ctx, s.logger)
	for _, function := range functions {
		function := function
		deleteFunctionsErrGroup.Go("Delete function", func() error {
			return s.DeleteFunction(deleteFunctionsErrGroupCtx, &function.GetConfig().Meta)
		})
	}
	if err := deleteFunctionsErrGroup.Wait(); err != nil {
		return errors.Wrap(err, "Failed to delete functions")
	}

	return s.deleteResource(projectsDir, projectMeta.Namespace, projectMeta.Name)
}

//
// Function events
//

func (s *Store) CreateOrUpdateFunctionEvent(functionEventConfig *platform.FunctionEventConfig) error {
	resourcePath := s.getResourcePath(functionEventsDir, functionEventConfig.Meta.Namespace, functionEventConfig.Meta.Name)

	// write the contents to that file name at the appropriate path
	return s.serializeAndWriteFileContents(resourcePath, functionEventConfig)
}

func (s *Store) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	var functionEvents []platform.FunctionEvent

	// get function filter
	functionName := getFunctionEventsOptions.Meta.Labels[common.NuclioResourceLabelKeyFunctionName]
	functionNames := getFunctionEventsOptions.FunctionNames
	if len(functionNames) > 0 {

		// make it easier to find
		sort.Strings(functionNames)
	}

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
			functionName != newFunctionEvent.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyFunctionName] {
			return nil
		}

		if len(functionNames) > 0 {

			idx := sort.SearchStrings(functionNames, newFunctionEvent.GetConfig().Meta.Name)

			// not in list
			if idx == len(functionNames) || functionNames[idx] != newFunctionEvent.GetConfig().Meta.Name {
				return nil
			}
		}

		functionEvents = append(functionEvents, &newFunctionEvent)

		return nil
	}

	if err := s.getResources(functionEventsDir,
		getFunctionEventsOptions.Meta.Namespace,
		getFunctionEventsOptions.Meta.Name,
		rowHandler); err != nil {
		return nil, errors.Wrap(err, "Failed to get function events")
	}

	return functionEvents, nil
}

func (s *Store) DeleteFunctionEvent(functionEventMeta *platform.FunctionEventMeta) error {
	return s.deleteResource(functionEventsDir, functionEventMeta.Namespace, functionEventMeta.Name)
}

//
// Function (used only for the period before there's a docker container to represent the function)
//

func (s *Store) CreateOrUpdateFunction(functionConfig *functionconfig.ConfigWithStatus) error {
	resourcePath := s.getResourcePath(functionsDir, functionConfig.Meta.Namespace, functionConfig.Meta.Name)

	// write the contents to that file name at the appropriate path
	return s.serializeAndWriteFileContents(resourcePath, functionConfig)
}

func (s *Store) GetProjectFunctions(getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	var functions []platform.Function

	// get project filter
	projectName := common.StringToStringMap(getFunctionsOptions.Labels, "=")[common.NuclioResourceLabelKeyProjectName]

	// get all the functions in the store. these functions represent both functions that are deployed
	// and functions that failed to build
	localStoreFunctions, err := s.GetFunctions(&functionconfig.Meta{
		Name:      getFunctionsOptions.Name,
		Namespace: getFunctionsOptions.Namespace,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to read functions from local store")
	}

	// filter by project name
	for _, localStoreFunction := range localStoreFunctions {
		if projectName != "" && localStoreFunction.GetConfig().Meta.Labels[common.NuclioResourceLabelKeyProjectName] != projectName {
			continue
		}
		functions = append(functions, localStoreFunction)
	}

	return functions, nil
}

func (s *Store) GetFunctions(functionMeta *functionconfig.Meta) ([]platform.Function, error) {
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

	if err := s.getResources(functionsDir, functionMeta.Namespace, functionMeta.Name, rowHandler); err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	return functions, nil
}

func (s *Store) DeleteFunction(ctx context.Context, functionMeta *functionconfig.Meta) error {
	functionEvents, err := s.GetFunctionEvents(&platform.GetFunctionEventsOptions{
		Meta: platform.FunctionEventMeta{
			Namespace: functionMeta.Namespace,
			Labels: map[string]string{
				common.NuclioResourceLabelKeyFunctionName: functionMeta.Name,
			},
		},
	})
	if err != nil {
		return errors.Wrap(err, "Failed to get function events")
	}

	deleteFunctionEventsErrGroup, _ := errgroup.WithContext(ctx, s.logger)
	for _, functionEvent := range functionEvents {
		functionEvent := functionEvent
		deleteFunctionEventsErrGroup.Go("Delete function event", func() error {
			return s.DeleteFunctionEvent(&functionEvent.GetConfig().Meta)
		})
	}

	if err := deleteFunctionEventsErrGroup.Wait(); err != nil {
		s.logger.WarnWithCtx(ctx, "Failed to delete function events, deleting function anyway",
			"err", err)
		return errors.Wrap(err, "Failed to delete function events")
	}

	return s.deleteResource(functionsDir, functionMeta.Namespace, functionMeta.Name)
}

//
// Implementation
//

func (s *Store) serializeAndWriteFileContents(resourcePath string, resourceConfig interface{}) error {

	// serialize the resource to json
	serializedResourceConfig, err := json.Marshal(resourceConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to serialize resource config")
	}

	return s.writeFileContents(resourcePath, serializedResourceConfig)
}

func (s *Store) getResourcePath(resourceDir string, resourceNamespace string, resourceName string) string {
	resourcePath := path.Join(s.getResourceNamespaceDir(resourceDir, resourceNamespace), resourceName+".json")
	// try to check if a function name has a space in it
	if strings.Contains(resourcePath, " ") {
		// we can get there only if a function was deployed with a wrong name in Nuclio 1.11.24 and earlier.
		// if a function name had a space in it, then we created a file called <first_word> instead of the correct name
		resourcePath = strings.Fields(resourcePath)[0]
	}
	return resourcePath
}

func (s *Store) getResourceNamespaceDir(resourceDir string, resourceNamespace string) string {
	return path.Join(resourceDir, resourceNamespace)
}

func (s *Store) getResources(resourceDir string,
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

		// if the error indicates that there's no such file that means nothing was created yet
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

func (s *Store) writeFileContents(filePath string, contents []byte) error {
	s.logger.DebugWith("Writing file contents", "path", filePath, "contents", string(contents))

	tempFile, err := os.CreateTemp(".", "nuclio-contents-temp-file-*")
	if err != nil {
		return errors.Wrap(err, "Error creating temporary file")
	}

	// remove the temporary file at the end
	defer os.Remove(tempFile.Name()) // nolint: errcheck

	// encode contents to base64 and add a newline at the end
	// newline at the end is needed to be able to parse files one be one
	// when doing `cat /functions/*`
	encodedContents := base64.StdEncoding.EncodeToString(contents) + "\n"

	// write content to the temporary file
	if _, err = tempFile.WriteString(encodedContents); err != nil {
		tempFile.Close() // nolint: errcheck
		return errors.Wrap(err, "Failed writing to temporary file")
	}

	// not using defer to ensure that we have closed the file before copying it
	if err = tempFile.Close(); err != nil {
		return errors.Wrap(err, "Failed closing temporary file")
	}

	// copy temporary file content to container
	return s.dockerClient.CopyObjectsToContainer(containerName,
		map[string]string{
			tempFile.Name(): filePath,
		})

}

func (s *Store) runCommand(env map[string]string, format string, args ...interface{}) (string, string, error) {
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

		// if container doesn't exist return the error
		if !strings.Contains(err.Error(), "No such container") {
			return "", "", errors.Wrapf(err, "Failed to execute command: %s", command)
		}

		// run a container that simply volumizes the volume with the storage and sleeps for 6 hours
		// using alpine mirrored to gcr.io/iguazio for stability
		if _, err := s.dockerClient.RunContainer(s.imageName, &dockerclient.RunOptions{
			Volumes:          map[string]string{volumeName: baseDir},
			Remove:           true,
			Command:          `/bin/sh -c "/bin/sleep 6h"`,
			Stdout:           &commandStdout,
			ImageMayNotExist: true,
			ContainerName:    containerName,
		}); err != nil &&
			!strings.Contains(err.Error(), "is already in use by container") {

			// if we failed and the error is not that it already exists, return the error

			return "", "", errors.Wrap(err, "Failed to run container with storage volume")
		}
	}

	return commandStdout, commandStderr, nil
}

func (s *Store) deleteResource(resourceDir string, resourceNamespace string, resourceName string) error {
	resourcePath := s.getResourcePath(resourceDir, resourceNamespace, resourceName)

	// stat the file
	if _, _, err := s.runCommand(nil, "/bin/stat %s", resourcePath); err != nil {
		return nuclio.ErrNotFound
	}

	// remove the file
	_, _, err := s.runCommand(nil, "/bin/rm %s", resourcePath)

	// if the error indicates that there's no such file - that means nothing was created yet
	if cause := errors.Cause(err); cause != nil && strings.Contains(cause.Error(), "No such file") {
		return nuclio.NewErrNotFound(fmt.Sprintf("Could not find resource %s", resourceName))
	}

	return err
}
