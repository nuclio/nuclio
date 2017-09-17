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
	logger  nuclio.Logger
	options *Options
}

func NewFunctionBuilder(parentLogger nuclio.Logger, options *Options) (*FunctionBuilder, error) {
	newFunctionBuilder := &FunctionBuilder{
		logger:  parentLogger.GetChild("builder").(nuclio.Logger),
		options: options,
	}

	return newFunctionBuilder, nil
}

func (fb *FunctionBuilder) Execute() (string, error) {

	// convert options
	buildOptions := build.Options{
		Verbose:         fb.options.Common.Verbose,
		FunctionName:    fb.options.Common.Identifier,
		FunctionPath:    fb.options.Path,
		OutputType:      fb.options.OutputType,
		OutputName:      fb.options.ImageName,
		OutputVersion:   fb.options.ImageVersion,
		NuclioSourceDir: fb.options.NuclioSourceDir,
		NuclioSourceURL: fb.options.NuclioSourceURL,
		PushRegistry:    fb.options.Registry,
		Runtime:         fb.options.Runtime,
		NoBaseImagePull: fb.options.NoBaseImagesPull,
	}

	// if output name isn't set, use identifier
	if buildOptions.OutputName == "" {
		buildOptions.OutputName = fb.options.Common.Identifier
	}

	// execute a build
	builder, err := build.NewBuilder(fb.logger, &buildOptions)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create builder")
	}

	return builder.Build()
}
