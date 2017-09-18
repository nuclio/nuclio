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

package builder

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build"

	"github.com/nuclio/nuclio-sdk"
)

type FunctionBuilder struct {
	logger nuclio.Logger
}

func NewFunctionBuilder(parentLogger nuclio.Logger) (*FunctionBuilder, error) {
	newFunctionBuilder := &FunctionBuilder{
		logger: parentLogger.GetChild("builder").(nuclio.Logger),
	}

	return newFunctionBuilder, nil
}

func (fb *FunctionBuilder) Execute(options *Options) (string, error) {

	// convert options
	buildOptions := build.Options{
		Verbose:         options.Common.Verbose,
		FunctionName:    options.Common.Identifier,
		FunctionPath:    options.Path,
		OutputType:      options.OutputType,
		OutputName:      options.ImageName,
		OutputVersion:   options.ImageVersion,
		NuclioSourceDir: options.NuclioSourceDir,
		NuclioSourceURL: options.NuclioSourceURL,
		PushRegistry:    options.Registry,
		Runtime:         options.Runtime,
		NoBaseImagePull: options.NoBaseImagesPull,
	}

	// if output name isn't set, use identifier
	if buildOptions.OutputName == "" {
		buildOptions.OutputName = options.Common.Identifier
	}

	// execute a build
	builder, err := build.NewBuilder(options.Common.GetLogger(fb.logger), &buildOptions)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create builder")
	}

	return builder.Build()
}
