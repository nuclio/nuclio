package dlx

import (
	"github.com/nuclio/logger"
	"net/http"
)

type Handler struct {
	logger logger.Logger
	requestHandler func(http.ResponseWriter, *http.Request)
	functionStarter *FunctionStarter
}

func NewHandler(logger logger.Logger, functionStarter *FunctionStarter) (Handler, error) {
	h := Handler{
		logger: logger,
		functionStarter: functionStarter,
	}
	h.requestHandler = h.handleRequest
	return h, nil
}

func (h *Handler) handleRequest(res http.ResponseWriter, req *http.Request) {
	responseChannel := make(chan error)
	h.functionStarter.GetOrCreateFunctionSink(req, res, responseChannel)
	err := <- responseChannel
	if err != nil {
		h.logger.Debug("There was an error")
	}
}
