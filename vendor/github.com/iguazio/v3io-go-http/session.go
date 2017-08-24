package v3io

import "github.com/valyala/fasthttp"

type Session struct {
	logger     Logger
	context    *Context
	sessionKey string
}

func newSession(parentLogger Logger,
	context *Context,
	username string,
	password string,
	label string) (*Session, error) {
	return &Session{
		logger:  parentLogger.GetChild("session").(Logger),
		context: context,
	}, nil
}

func (s *Session) NewContainer(alias string) (*Container, error) {
	return newContainer(s.logger, s, alias)
}

func (s *Session) sendRequest(request *fasthttp.Request, response *fasthttp.Response) error {

	// add session key
	// TODO

	// delegate to context
	return s.context.sendRequest(request, response)
}
