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
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/golang/eventhandlerparser"
)

type golang struct {
	*runtime.AbstractRuntime
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

// GetName returns the name of the runtime, including version if applicable
func (g *golang) GetName() string {
	return "golang"
}

// GetBuildArgs return arguments passed to image builder
func (g *golang) GetBuildArgs() (map[string]string, error) {

	// call inherited
	buildArgs, err := g.AbstractRuntime.GetBuildArgs()
	if err != nil {
		return nil, err
	}

	// if the base image is default (which is alpine) and is not alpine based, must use the non-alpine onbuild image
	if g.FunctionConfig.Spec.Build.BaseImage != "" && !strings.Contains(g.FunctionConfig.Spec.Build.BaseImage, "alpine") {

		// set tag and arch
		onbuildImage := fmt.Sprintf("nuclio/handler-builder-golang-onbuild:%s-%s",
			buildArgs["NUCLIO_TAG"],
			buildArgs["NUCLIO_ARCH"])

		// set the onbuild image
		buildArgs["NUCLIO_ONBUILD_IMAGE"] = onbuildImage
	}

	return buildArgs, nil
}

// GetProcessorDockerfilePath returns the path of the appropriate Dockerfile, with which we'll build
// the processor image
func (g *golang) GetProcessorDockerfilePath(stagingDir string) string {
	return ""
}
