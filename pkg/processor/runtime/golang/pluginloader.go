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

	"github.com/nuclio/nuclio-sdk"
)

type pluginHandlerLoader struct{}

func (phl *pluginHandlerLoader) load(path string, handlerName string) (func(*nuclio.Context, nuclio.Event) (interface{}, error), error) {

	handlerPlugin, err := plugin.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't load plugin at %q", path)
	}

	handlerSymbol, err := handlerPlugin.Lookup(handlerName)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't find handler %q in %q",
			handlerName,
			path)
	}

	typedHandlerSymbol, ok := handlerSymbol.(func(*nuclio.Context, nuclio.Event) (interface{}, error))
	if !ok {
		return nil, fmt.Errorf("%s:%s is from wrong type - %T",
			path,
			handlerName,
			handlerSymbol)
	}

	return typedHandlerSymbol, nil
}
