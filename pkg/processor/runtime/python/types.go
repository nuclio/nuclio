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

package python

import "github.com/nuclio/nuclio/pkg/processor/runtime"

// Configuration is python configuration
type Configuration struct {
	runtime.Configuration

	// What to run (e.g. "module:function")
	Handler string

	// Python version
	PythonVersion string

	// a map of environment variables that need to be injected into the
	// process. a nil value indicates to take it from the running process'
	// environment map
	Env map[string]*string
}
