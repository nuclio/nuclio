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
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/platform"
)

type FunctionBuilder struct {
	logger           nuclio.Logger
	platformInstance platform.Platform
}

func NewFunctionBuilder(parentLogger nuclio.Logger, platformInstance platform.Platform) (*FunctionBuilder, error) {
	newFunctionBuilder := &FunctionBuilder{
		logger:           parentLogger.GetChild("builder").(nuclio.Logger),
		platformInstance: platformInstance,
	}

	return newFunctionBuilder, nil
}

func (fb *FunctionBuilder) Build(buildOptions *platform.BuildOptions) (string, error) {
	return fb.platformInstance.BuildFunction(buildOptions)
}
