package v3io_client

import (
	"net/http"

	"github.com/iguazio/v3io"

	"github.com/nuclio/nuclio/pkg/logger"
)

// thin wrapper for v3iow
type V3ioClient struct {
	v3io.V3iow
	logger logger.Logger
}

func NewV3ioClient(logger logger.Logger, url string) *V3ioClient {

	newV3ioClient := &V3ioClient{
		V3iow: v3io.V3iow{
			Url:        url,
			Tr:         &http.Transport{},
			DebugState: true,
		},
		logger: logger.GetChild("v3io"),
	}

	// set logger sink
	newV3ioClient.LogSink = newV3ioClient.logSink

	return newV3ioClient
}

func (vc *V3ioClient) logSink(formatted string) {
	vc.logger.Debug(formatted)
}
