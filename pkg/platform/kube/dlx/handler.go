package dlx

import (
	"github.com/nuclio/logger"
	"net/http"
	"net/http/httputil"
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
	responseChannel := make(chan FunctionStatusResult, 1)
	defer close(responseChannel)

	headerTarget := req.Header.Get("X-nuclio-target")
	if headerTarget == "" {
		h.logger.Warn("Must pass X-nuclio-target header value")
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	h.functionStarter.GetOrCreateFunctionSink(headerTarget, responseChannel)
	statusResult := <- responseChannel

	if statusResult.Error != nil {
		h.logger.WarnWith("Failed to forward request to function", "function", statusResult.FunctionName, "err", statusResult.Error)
		res.WriteHeader(statusResult.Status)
		return
	}

	// TODO maybe not needed
	proxy := httputil.NewSingleHostReverseProxy(req.URL)
	proxy.ServeHTTP(res, req)
}
