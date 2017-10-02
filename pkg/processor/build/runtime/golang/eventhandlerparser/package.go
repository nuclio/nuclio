package eventhandlerparser

import (
	"go/importer"

	"github.com/nuclio/nuclio-sdk"
)

const (
	handlerSignature = "func(context *github.com/nuclio/nuclio-sdk.Context, event github.com/nuclio/nuclio-sdk.Event) (interface{}, error)"
)

// PackageHandlerParser parsers event handlers in a package
type PackageHandlerParser struct {
	logger nuclio.Logger
}

// NewPackageHandlerParser returns new EventHandlerParser
func NewPackageHandlerParser(logger nuclio.Logger) *PackageHandlerParser {
	return &PackageHandlerParser{logger}
}

func (p *PackageHandlerParser) ParseEventHandlers(packageName string) ([]string, []string, error) {
	pkg, err := importer.Default().Import(packageName)
	if err != nil {
		return nil, nil, err
	}

	var handlers []string

	for _, name := range pkg.Scope().Names() {
		if pkg.Scope().Lookup(name).Type().String() == handlerSignature {
			handlers = append(handlers, name)
		}
	}

	return []string{packageName}, handlers, nil
}
