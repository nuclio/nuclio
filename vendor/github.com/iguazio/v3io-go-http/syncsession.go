package v3io

import "github.com/valyala/fasthttp"

type SyncSession struct {
	logger     Logger
	context    *SyncContext
	sessionKey string
}

func newSyncSession(parentLogger Logger,
	context *SyncContext,
	username string,
	password string,
	label string) (*SyncSession, error) {
	return &SyncSession{
		logger:  parentLogger.GetChild("session").(Logger),
		context: context,
	}, nil
}

func (ss *SyncSession) sendRequest(request *fasthttp.Request, response *fasthttp.Response) error {

	// add session key
	// TODO

	// delegate to context
	return ss.context.sendRequest(request, response)
}
