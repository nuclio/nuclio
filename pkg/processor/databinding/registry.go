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

package databinding

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/registry"

	"github.com/nuclio/nuclio-sdk"
)

// Creator creates a databinding instance
type Creator interface {

	// Create creates a trigger instance
	Create(nuclio.Logger, string, *functionconfig.DataBinding) (DataBinding, error)
}

type Registry struct {
	registry.Registry
}

// global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("databinding"),
}

func (r *Registry) NewDataBinding(logger nuclio.Logger,
	kind string,
	name string,
	databindingConfiguration *functionconfig.DataBinding) (DataBinding, error) {

	registree, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	return registree.(Creator).Create(logger,
		name,
		databindingConfiguration)
}
