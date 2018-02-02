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

package golang

import (
	"fmt"
	"plugin"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk-go"
)

type pluginHandlerLoader struct {
	abstractHandler
}

func (phl *pluginHandlerLoader) load(configuration *runtime.Configuration) error {

	// try to load via base, if successful we're done
	if err := phl.abstractHandler.load(configuration); err != nil {
		return errors.Wrap(err, "Failed to load handler")
	}

	// base loads defaults in some cases
	if phl.entrypoint != nil {
		return nil
	}

	handlerPlugin, err := plugin.Open(configuration.Spec.Build.Path)
	if err != nil {
		return errors.Wrapf(err, "Can't load plugin at %q", configuration.Spec.Build.Path)
	}

	// parse the handler name
	_, handlerName, err := phl.parseName(configuration.Spec.Handler)
	if err != nil {
		return errors.Wrap(err, "Failed to parse handler name")
	}

	handlerSymbol, err := handlerPlugin.Lookup(handlerName)
	if err != nil {
		return errors.Wrapf(err, "Can't find handler %q in %q",
			handlerName,
			configuration.Spec.Build.Path)
	}

	var ok bool

	phl.entrypoint, ok = handlerSymbol.(func(*nuclio.Context, nuclio.Event) (interface{}, error))
	if !ok {
		return fmt.Errorf("%s:%s is of wrong type - %T",
			configuration.Spec.Build.Path,
			handlerName,
			handlerSymbol)
	}

	contextInitializerSymbol, err := handlerPlugin.Lookup("InitContext")

	// if we can't find it, just carry on - it's not mandatory
	if err != nil {
		return nil
	}

	phl.contextInitializer, ok = contextInitializerSymbol.(func(*nuclio.Context) error)
	if !ok {
		return fmt.Errorf("InitContext is of wrong type - %T", contextInitializerSymbol)
	}

	return nil
}
