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

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/rs/xid"
)

const (
	volumeName = "nuclio-local-storage"
	baseDir = "/etc/nuclio/store"
	processorConfigDir = baseDir + "/processor-configs"
	projectsDir = baseDir + "/projects"
)

type store struct {
	logger logger.Logger
	dockerClient dockerclient.Client
}

func newStore(parentLogger logger.Logger, dockerClient dockerclient.Client) (*store, error) {
	return &store{
		logger: parentLogger.GetChild("store"),
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

func (s *store) createOrUpdateProject(projectConfig *platform.ProjectConfig) error {

	// serialize the project to json
	serializedProjectConfig, err := json.Marshal(projectConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to serialize project config")
	}

	// write the contents to that file name at the appropriate path path
	return s.writeFileContents(s.projectMetaToPath(&projectConfig.Meta), serializedProjectConfig)
}

func (s *store) deleteProject(projectMeta *platform.ProjectMeta) error {

	// generate a command
	command := fmt.Sprintf(`/bin/rm %s`, s.projectMetaToPath(projectMeta))

	// run in docker, volumizing
	_, err := s.dockerClient.RunContainer("alpine:3.6", &dockerclient.RunOptions{
		Volumes: map[string]string{volumeName: baseDir},
		Remove: true,
		Command: command,
		Attach: true,
	})

	return err
}

func (s *store) getProjects(projectMeta *platform.ProjectMeta) ([]platform.Project, error) {
	var projectPath string
	var commandStdout string

	// if the request is for a single project, get that file
	if projectMeta.Name != "" {
		projectPath = s.projectMetaToPath(projectMeta)
	} else {
		projectPath = path.Join(s.projectMetaToNamespaceDir(projectMeta), "*")
	}

	// generate a command
	command := fmt.Sprintf(`/bin/sh -c "/bin/cat %s"`, projectPath)

	// run in docker, volumizing
	_, err := s.dockerClient.RunContainer("alpine:3.6", &dockerclient.RunOptions{
		Volumes: map[string]string{volumeName: baseDir},
		Remove: true,
		Command: command,
		Stdout: &commandStdout,
		Attach: true,
	})

	var projects []platform.Project

	// iterate over the output line by line
	scanner := bufio.NewScanner(strings.NewReader(commandStdout))
	for scanner.Scan() {
		projectConfig := platform.ProjectConfig{}

		// get row contents
		rowContents := scanner.Text()

		// try to unmarshal
		err = json.Unmarshal([]byte(rowContents), &projectConfig)
		if err != nil {
			s.logger.DebugWith("Ignoring project", "contents", rowContents)
			continue
		}

		// create an abstract project
		newAbstractProject, err := platform.NewAbstractProject(s.logger, nil, projectConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create abstract project")
		}

		// add to projects
		projects = append(projects, newAbstractProject)
	}

	return projects, nil
}

func (s *store) writeFileContents(filePath string, contents []byte) error {
	s.logger.DebugWith("Writing file contents", "path", filePath, "contents", contents)

	// get the file dir
	fileDir := path.Dir(filePath)

	// generate a command
	command := fmt.Sprintf(`/bin/sh -c "mkdir -p %s && /bin/printenv NUCLIO_CONTENTS > %s"`, fileDir, filePath)

	// run in docker, volumizing
	_, err := s.dockerClient.RunContainer("alpine:3.6", &dockerclient.RunOptions{
		Volumes: map[string]string{volumeName: baseDir},
		Remove: true,
		Command: command,
		Env: map[string]string{"NUCLIO_CONTENTS": string(contents)},
		Attach: true,
	})

	return err
}

func (s *store) projectMetaToPath(projectMeta *platform.ProjectMeta) string {
	return path.Join(s.projectMetaToNamespaceDir(projectMeta), projectMeta.Name + ".json")
}

func (s *store) projectMetaToNamespaceDir(projectMeta *platform.ProjectMeta) string {
	return path.Join(projectsDir, projectMeta.Namespace)
}