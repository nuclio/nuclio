package runtime

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/v3ioclient"
)

func newContext(logger nuclio.Logger, configuration *Configuration) *nuclio.Context {
	newContext := &nuclio.Context{
		Logger: logger,
	}

	// create v3io context if applicable
	for _, dataBinding := range configuration.DataBindings {
		if dataBinding.Class == "v3io" {
			newContext.DataBinding = v3ioclient.NewV3ioClient(logger, dataBinding.URL)
		}
	}

	return newContext
}
