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

package platform

import (
	"io"
)

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type Platform interface {

	// Build will locally build a processor image and return its name (or the error)
	BuildFunction(buildOptions *BuildOptions) (*BuildResult, error)

	// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
	DeployFunction(deployOptions *DeployOptions) (*DeployResult, error)

	// UpdateOptions will update a previously deployed function
	UpdateFunction(updateOptions *UpdateOptions) error

	// DeleteFunction will delete a previously deployed function
	DeleteFunction(deleteOptions *DeleteOptions) error

	// InvokeFunction will invoke a previously deployed function
	InvokeFunction(invokeOptions *InvokeOptions, writer io.Writer) error

	// InvokeFunction will invoke a previously deployed function
	GetFunctions(getOptions *GetOptions) ([]Function, error)

	// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
	GetDeployRequiresRegistry() bool

	// GetName returns the platform name
	GetName() string

	// GetNodes returns a slice of nodes currently in the cluster
	GetNodes() ([]Node, error)
}
