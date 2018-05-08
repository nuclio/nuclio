/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the Licensg.
*/

package golang

import (
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/golang/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
)

type golang struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImage returns the image name of the default processor base image
func (g *golang) GetProcessorBaseImage() (string, error) {
	return "", nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (g *golang) DetectFunctionHandlers(functionPath string) ([]string, error) {
	parser := eventhandlerparser.NewEventHandlerParser(g.Logger)

	packages, handlers, err := parser.ParseEventHandlers(functionPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't find handlers in %q", functionPath)
	}

	g.Logger.DebugWith("Parsed event handlers", "packages", packages, "handlers", handlers)

	if len(handlers) != 1 {
		return nil, errors.Wrapf(err, "Expected one handler, found %d", len(handlers))
	}

	if len(packages) != 1 {
		return nil, errors.Wrapf(err, "Expected one package, found %d", len(packages))
	}

	return handlers[:1], nil
}

// OnAfterStagingDirCreated prepares anything it may need in that directory
// towards building a functioning processor,
func (g *golang) OnAfterStagingDirCreated(stagingDir string) error {

	// copy the function source into the appropriate location
	if err := g.createUserFunctionPath(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to create user function path")
	}

	return nil
}

// GetName returns the name of the runtime, including version if applicable
func (g *golang) GetName() string {
	return "golang"
}

func (g *golang) createUserFunctionPath(stagingDir string) error {

	userFunctionDirInStaging := filepath.Join(stagingDir, "handler")
	g.Logger.DebugWith("Creating user function path", "path", userFunctionDirInStaging)

	if err := os.MkdirAll(userFunctionDirInStaging, 0755); err != nil {
		return errors.Wrapf(err, "Failed to create user function path in staging at %s", userFunctionDirInStaging)
	}

	copyFrom := g.FunctionConfig.Spec.Build.Path

	// copy the function (file / dir) to the stagind dir
	g.Logger.DebugWith("Copying user function", "from", copyFrom, "to", userFunctionDirInStaging)
	err := util.CopyTo(g.FunctionConfig.Spec.Build.Path, userFunctionDirInStaging)

	if err != nil {
		return errors.Wrapf(err, "Error copying from %s to %s", copyFrom, userFunctionDirInStaging)
	}

	return nil
}
