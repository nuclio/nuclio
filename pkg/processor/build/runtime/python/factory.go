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

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type factory struct {}

func (f *factory) Create(logger nuclio.Logger, configuration runtime.Configuration) (runtime.Runtime, error) {
	return &python{
		AbstractRuntime: runtime.AbstractRuntime{
			Logger: logger,
			Configuration: configuration,
		},
	}, nil
}

func init() {
	runtime.RuntimeRegistrySingleton.Register("python", &factory{})
}
