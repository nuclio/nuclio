package runtime

import (
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/pkg/v3ioclient"
)

type Context struct {
	Logger     logger.Logger
	V3ioClient *v3ioclient.V3ioClient
}

func newContext(logger logger.Logger, configuration *Configuration) *Context {
	newContext := &Context{
		Logger: logger,
	}

	// create v3io context if applicable
	for _, dataBinding := range configuration.DataBindings {
		if dataBinding.Class == "v3io" {
			newContext.V3ioClient = v3ioclient.NewV3ioClient(logger, dataBinding.URL)
		}
	}

	return newContext
}
