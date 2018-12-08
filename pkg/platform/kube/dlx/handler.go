package dlx

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/nuclio/logger"
)

type Handler struct {
	logger          logger.Logger
	handleFunc      func(http.ResponseWriter, *http.Request)
	functionStarter *FunctionStarter
}

func NewHandler(logger logger.Logger, functionStarter *FunctionStarter) (Handler, error) {
	h := Handler{
		logger:          logger,
		functionStarter: functionStarter,
	}
	h.handleFunc = h.handleRequest
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

	h.functionStarter.HandleFunctionStart(headerTarget, responseChannel)
	statusResult := <-responseChannel

	if statusResult.Error != nil {
		h.logger.WarnWith("Failed to forward request to function",
			"function", statusResult.FunctionName,
			"err", statusResult.Error)
		res.WriteHeader(statusResult.Status)
		return
	}

	targeURL, _ := url.Parse(fmt.Sprintf("http://%s:8080", headerTarget))
	proxy := httputil.NewSingleHostReverseProxy(targeURL)
	proxy.ServeHTTP(res, req)
}
