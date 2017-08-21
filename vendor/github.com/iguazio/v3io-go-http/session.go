package v3io

import "github.com/valyala/fasthttp"

type Session struct {
	logger     Logger
	client     *Client
	sessionKey string
}

func newSession(parentLogger Logger,
	client *Client,
	username string,
	password string,
	label string) (*Session, error) {
	return &Session{
		logger: parentLogger.GetChild("session").(Logger),
		client: client,
	}, nil
}

func (s *Session) NewContainer(alias string) (*Container, error) {
	return newContainer(s.logger, s, alias)
}

func (s *Session) sendRequest(request *fasthttp.Request, response *fasthttp.Response) error {

	// add session key
	// TODO

	// delegate to client
	return s.client.sendRequest(request, response)
}
