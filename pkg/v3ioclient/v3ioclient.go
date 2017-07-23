package v3ioclient

import (
	"net/http"

	"github.com/nuclio/nuclio-sdk"

	"github.com/iguazio/v3io"
)

// thin wrapper for v3iow
type V3ioClient struct {
	v3io.V3iow
	logger nuclio.Logger
}

func NewV3ioClient(parentLogger nuclio.Logger, url string) *V3ioClient {

	newV3ioClient := &V3ioClient{
		V3iow: v3io.V3iow{
			Url:        url,
			Tr:         &http.Transport{},
			DebugState: true,
		},
		logger: parentLogger.GetChild("v3io").(nuclio.Logger),
	}

	// set logger sink
	newV3ioClient.LogSink = newV3ioClient.logSink

	return newV3ioClient
}

func (vc *V3ioClient) logSink(formatted string) {
	vc.logger.Debug(formatted)
}
