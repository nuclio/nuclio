package shell

import "github.com/nuclio/nuclio/pkg/processor/runtime"

type Configuration struct {
	runtime.Configuration

	// what to run
	ScriptPath string
	ScriptArgs []string

	// a map of environment variables that need to be injected into the shell process. a nil value
	// indicates to take it from the running process' environment map
	Env map[string]*string
}
