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
