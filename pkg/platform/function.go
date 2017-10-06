package platform

type Function interface {

	// Initialize instructs the function to load the fields specified by "fields". Some function implementations
	// are lazy-load - this ensures that the fields are populated properly. if "fields" is nil, all fields
	// are loaded
	Initialize([]string) error

	// GetNamespace returns the namespace of the function, if its part of a namespace
	GetNamespace() string

	// GetName returns the name of the function
	GetName() string

	// GetVersion returns the version of the function
	GetVersion() string

	// GetState returns the state of the function
	GetState() string

	// GetHTTPPort returns the port of the HTTP event source
	GetHTTPPort() int

	// GetLabels returns the function labels
	GetLabels() map[string]string

	// GetReplicas returns the current # of replicas and the configured # of replicas
	GetReplicas() (int, int)
}
