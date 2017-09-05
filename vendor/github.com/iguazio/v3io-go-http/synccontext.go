package v3io

import (
	"github.com/valyala/fasthttp"
)

type SyncContext struct {
	logger     Logger
	httpClient *fasthttp.HostClient
	clusterURL string
}

func newSyncContext(parentLogger Logger, clusterURL string) (*SyncContext, error) {
	newSyncContext := &SyncContext{
		logger: parentLogger.GetChild("v3io").(Logger),
		httpClient: &fasthttp.HostClient{
			Addr: clusterURL,
		},
		clusterURL: clusterURL,
	}

	return newSyncContext, nil
}

func (sc *SyncContext) sendRequest(request *fasthttp.Request, response *fasthttp.Response) error {
	sc.logger.DebugWith("Sending request",
		"method", string(request.Header.Method()),
		"uri", string(request.Header.RequestURI()),
		// "headers", string(request.Header.Header()),
		"body", string(request.Body()),
	)

	err := sc.httpClient.Do(request, response)
	if err != nil {
		return err
	}

	// log the response
	sc.logger.DebugWith("Got response",
		"statusCode", response.Header.StatusCode(),
		"body", string(response.Body()),
	)

	return nil
}
