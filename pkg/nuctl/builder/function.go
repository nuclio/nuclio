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
	"github.com/nuclio/nuclio/pkg/build"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
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

func (fb *FunctionBuilder) Execute() error {

	// convert options
	buildOptions := build.Options{
		Verbose:         fb.options.Common.Verbose,
		FunctionPath:    fb.options.Path,
		OutputType:      fb.options.OutputType,
		OutputName:      fb.options.ImageName,
		Version:         fb.options.ImageVersion,
		NuclioSourceDir: fb.options.NuclioSourceDir,
		NuclioSourceURL: fb.options.NuclioSourceURL,
		PushRegistry:    fb.options.Registry,
	}

	// if output name isn't set, use identifier
	if buildOptions.OutputName == "" {
		buildOptions.OutputName = fb.options.Common.Identifier
	}

	// execute a build
	if err := build.NewBuilder(fb.logger, &buildOptions).Build(); err != nil {
		return errors.Wrap(err, "Failed to build")
	}

	return nil
}
