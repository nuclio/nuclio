package python

import "github.com/nuclio/nuclio/pkg/processor/runtime"

// Configuration is python configuration
type Configuration struct {
	runtime.Configuration

	// what to run
	EntryPoint string

	// a map of environment variables that need to be injected into the
	// process. a nil value indicates to take it from the running process'
	// environment map
	Env map[string]*string
}
