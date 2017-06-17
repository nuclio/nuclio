package runtime

import (
	"net/http"

	"github.com/iguazio/v3io"

	"github.com/nuclio/nuclio/pkg/logger"
)

type Context struct {
	Logger     logger.Logger
	V3ioClient *v3io.V3iow
}

func newContext(logger logger.Logger, configuration *Configuration) *Context {
	newContext := &Context{
		Logger: logger,
	}

	// create v3io context if applicable
	for _, dataBinding := range configuration.DataBindings {
		if dataBinding.Class == "v3io" {
			newContext.V3ioClient = &v3io.V3iow{
				Url:        dataBinding.URL,
				Tr:         &http.Transport{},
				DebugState: false,
			}
		}
	}

	return newContext
}
