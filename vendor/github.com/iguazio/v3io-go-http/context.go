package v3io

import (
	"github.com/valyala/fasthttp"
)

type Context struct {
	logger     Logger
	httpClient *fasthttp.HostClient
	clusterURL string
}

func NewContext(parentLogger Logger, clusterURL string) (*Context, error) {
	newClient := &Context{
		logger: parentLogger.GetChild("v3io").(Logger),
		httpClient: &fasthttp.HostClient{
			Addr: clusterURL,
		},
		clusterURL: clusterURL,
	}

	return newClient, nil
}

func (c *Context) NewSession(username string, password string, label string) (*Session, error) {
	return newSession(c.logger, c, username, password, label)
}

func (c *Context) sendRequest(request *fasthttp.Request, response *fasthttp.Response) error {
	c.logger.DebugWith("Sending request",
		"method", string(request.Header.Method()),
		"uri", string(request.Header.RequestURI()),
		"body", string(request.Body()),
	)

	err := c.httpClient.Do(request, response)
	if err != nil {
		return err
	}

	// log the response
	c.logger.DebugWith("Got response",
		"statusCode", response.Header.StatusCode(),
		"body", string(response.Body()),
	)

	return nil
}
