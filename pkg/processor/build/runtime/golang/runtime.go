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

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/golang/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"

	"github.com/nuclio/errors"
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

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (g *golang) GetProcessorDockerfileInfo(runtimeConfig *runtimeconfig.Config, onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{
		BaseImage: "alpine:3.15",
	}

	// if the base image is not default (which is alpine) and is not alpine based, must use the non-alpine onbuild image
	var onbuildImage string
	if g.FunctionConfig.Spec.Build.BaseImage != "" &&
		!strings.Contains(g.FunctionConfig.Spec.Build.BaseImage, "alpine") {

		// use non-alpine based image
		onbuildImage = "%s/nuclio/handler-builder-golang-onbuild:%s-%s"
	} else {

		// use alpine based image
		onbuildImage = "%s/nuclio/handler-builder-golang-onbuild:%s-%s-alpine"
	}

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Image: fmt.Sprintf(onbuildImage, onbuildImageRegistry, g.VersionInfo.Label, g.VersionInfo.Arch),
		Name:  "golang-onbuild",
		Paths: map[string]string{
			"/home/nuclio/bin/processor":  "/usr/local/bin/processor",
			"/home/nuclio/bin/handler.so": "/opt/nuclio/handler.so",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	return &processorDockerfileInfo, nil
}
