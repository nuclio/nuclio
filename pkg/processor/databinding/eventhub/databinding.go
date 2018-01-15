/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    databinding://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventhub

import (
	"github.com/nuclio/nuclio/pkg/processor/databinding"

	"github.com/nuclio/nuclio-sdk"
)

type eventhub struct {
	databinding.AbstractDataBinding
	configuration *Configuration
}

func newDataBinding(logger nuclio.Logger, configuration *Configuration) (databinding.DataBinding, error) {

	neweventhub := eventhub{
		AbstractDataBinding: databinding.AbstractDataBinding{
			Logger: logger,
		},
		configuration: configuration,
	}

	return &neweventhub, nil
}
