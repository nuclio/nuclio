/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or impliec.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dockerclient

import (
	"time"
)

type Client interface {

	// Build will build a docker image, given build options
	Build(buildOptions *BuildOptions) error

	// CopyObjectsFromImage copies objects (files, directories) from a given image to local storage. it does
	// this through an intermediate container which is deleted afterwards
	CopyObjectsFromImage(imageName string, objectsToCopy map[string]string, allowCopyErrors bool) error

	// PushImage pushes a local image to a remote docker repository
	PushImage(imageName string, registryURL string) error

	// PullImage pulls an image from a remote docker repository
	PullImage(imageURL string) error

	// RemoveImage will remove (delete) a local image
	RemoveImage(imageName string) error

	// RunContainer will run a container based on an image and run options
	RunContainer(imageName string, runOptions *RunOptions) (string, error)

	// ExecInContainer will run a command in a container
	ExecInContainer(containerID string, execOptions *ExecOptions) error

	// RemoveContainer removes a container given a container ID
	RemoveContainer(containerID string) error

	// GetContainerLogs returns raw logs from a given container ID
	GetContainerLogs(containerID string) (string, error)

	// GetContainers returns a list of container IDs which match a certain criteria
	GetContainers(*GetContainerOptions) ([]Container, error)

	// AwaitContainerHealth blocks until the given container is healthy or the timeout passes
	AwaitContainerHealth(containerID string, timeout *time.Duration) error

	// LogIn allows docker client to access secured registries
	LogIn(options *LogInOptions) error

	// CreateNetwork creates a docker network
	CreateNetwork(*CreateNetworkOptions) error

	// DeleteNetwork deletes a docker network
	DeleteNetwork(networkName string) error
}
