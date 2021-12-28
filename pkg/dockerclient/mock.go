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

package dockerclient

import (
	"context"
	"io"
	"time"

	"github.com/stretchr/testify/mock"
)

//
// Custom resource client mock
//

type MockDockerClient struct {
	mock.Mock
}

func NewMockDockerClient() *MockDockerClient {
	return &MockDockerClient{}
}

// Build will build a docker image, given build options
func (mdc *MockDockerClient) Build(buildOptions *BuildOptions) error {
	return nil
}

// CopyObjectsFromImage copies objects (files, directories) from a given image to local storage. it does
// this through an intermediate container which is deleted afterwards
func (mdc *MockDockerClient) CopyObjectsFromImage(imageName string, objectsToCopy map[string]string, allowCopyErrors bool) error {
	return nil
}

// PushImage pushes a local image to a remote docker repository
func (mdc *MockDockerClient) PushImage(imageName string, registryURL string) error {
	return nil
}

// PullImage pulls an image from a remote docker repository
func (mdc *MockDockerClient) PullImage(imageURL string) error {
	return nil
}

// RemoveImage will remove (delete) a local image
func (mdc *MockDockerClient) RemoveImage(imageName string) error {
	return nil
}

// RunContainer will run a container based on an image and run options
func (mdc *MockDockerClient) RunContainer(imageName string, runOptions *RunOptions) (string, error) {
	return "", nil
}

// ExecInContainer will run a command in a container
func (mdc *MockDockerClient) ExecInContainer(containerID string, execOptions *ExecOptions) error {
	return nil
}

// RemoveContainer removes a container given a container ID
func (mdc *MockDockerClient) RemoveContainer(containerID string) error {
	return nil
}

// StopContainer stops a container given a container ID
func (mdc *MockDockerClient) StopContainer(containerID string) error {
	return nil
}

// StartContainer stops a container given a container ID
func (mdc *MockDockerClient) StartContainer(containerID string) error {
	return nil
}

// GetContainerLogs returns raw logs from a given container ID
func (mdc *MockDockerClient) GetContainerLogs(containerID string) (string, error) {
	return "", nil
}

// GetContainers returns a list of container IDs which match a certain criteria
func (mdc *MockDockerClient) GetContainers(options *GetContainerOptions) ([]Container, error) {
	return nil, nil
}

func (mdc *MockDockerClient) GetContainerPort(container *Container, boundPort int) (int, error) {
	return boundPort, nil

}

// GetContainerEvents returns a list of container events which occurred within a time range
func (mdc *MockDockerClient) GetContainerEvents(containerName string, since string, until string) ([]string, error) {
	return nil, nil
}

// AwaitContainerHealth blocks until the given container is healthy or the timeout passes
func (mdc *MockDockerClient) AwaitContainerHealth(containerID string, timeout *time.Duration) error {
	return nil
}

// LogIn allows docker client to access secured registries
func (mdc *MockDockerClient) LogIn(options *LogInOptions) error {
	args := mdc.Called(options)
	return args.Error(0)
}

// CreateNetwork creates a docker network
func (mdc *MockDockerClient) CreateNetwork(options *CreateNetworkOptions) error {
	args := mdc.Called(options)
	return args.Error(0)
}

// DeleteNetwork deletes a docker network
func (mdc *MockDockerClient) DeleteNetwork(networkName string) error {
	args := mdc.Called(networkName)
	return args.Error(0)
}

// CreateVolume creates a docker volume
func (mdc *MockDockerClient) CreateVolume(options *CreateVolumeOptions) error {
	args := mdc.Called(options)
	return args.Error(0)
}

// DeleteVolume deletes a docker volume
func (mdc *MockDockerClient) DeleteVolume(volumeName string) error {
	args := mdc.Called(volumeName)
	return args.Error(0)
}

// Save saves a docker image in path
func (mdc *MockDockerClient) Save(imageName string, outPath string) error {
	args := mdc.Called(imageName, outPath)
	return args.Error(0)
}

// Load loads a docker image from path
func (mdc *MockDockerClient) Load(inPath string) error {
	args := mdc.Called(inPath)
	return args.Error(0)
}

func (mdc *MockDockerClient) GetVersion(quiet bool) (string, error) {
	args := mdc.Called(quiet)
	return args.String(0), args.Error(1)
}

func (mdc *MockDockerClient) GetContainerIPAddresses(containerID string) ([]string, error) {
	args := mdc.Called(containerID)
	return args.Get(0).([]string), args.Error(1)
}

func (mdc *MockDockerClient) GetContainerLogStream(ctx context.Context,
	containerID string,
	logOptions *ContainerLogsOptions) (io.ReadCloser, error) {
	args := mdc.Called(ctx, containerID, logOptions)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
