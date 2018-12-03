package proxier

import (
	"fmt"
	"github.com/nuclio/logger"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Handler struct {
	Logger    *logger.Logger
	Transport http.RoundTripper
}

type statusCapture struct {
	http.ResponseWriter
	statusCode int
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	capture := &statusCapture{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", r.Endpoint.FQDN, r.Endpoint.Port),
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = h.Transport

	proxy.ServeHTTP(capture, r)
}