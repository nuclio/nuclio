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
