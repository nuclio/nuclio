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

package nodejs

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	stagingDir string,
	functionConfig *functionconfig.Config) (runtime.Runtime, error) {

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, stagingDir, functionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}

	return &nodejs{
		AbstractRuntime: abstractRuntime,
	}, nil
}

func init() {
	runtime.RuntimeRegistrySingleton.Register("nodejs", &factory{})
}
