/*
Copyright 2026 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	httptrigger "github.com/nuclio/nuclio/pkg/processor/trigger/http"
	"golang.org/x/sync/errgroup"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// server fronts a function or the DLX.
type server struct {
	logger     logger.Logger
	httpServer *http.Server
}

// newServer wraps an HTTP server around a ready handler bound to listenAddress.
func newServer(parentLogger logger.Logger, listenAddress string, handler http.Handler) *server {
	return &server{
		logger: parentLogger,
		httpServer: &http.Server{
			Addr:    listenAddress,
			Handler: handler,
		},
	}
}

// newHandler builds the HTTP handler for the given mode: reverseProxy authenticates each request and
// forwards it to the processor; authOnly serves the DLX /auth endpoint.
func newHandler(parentLogger logger.Logger,
	mode auth.ProxyMode,
	upstreamURL string,
	authenticator authproxy.Authenticator) (http.Handler, error) {

	switch mode {
	case auth.ProxyModeReverseProxy:
		handler, err := newReverseProxyHandler(parentLogger, upstreamURL, authenticator)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create reverse-proxy handler")
		}
		return handler, nil
	case auth.ProxyModeAuthOnly:
		return newAuthOnlyHandler(authenticator), nil
	default:
		return nil, errors.Errorf("Unknown auth-proxy mode: %s", mode)
	}
}

// resolveListenAddresses returns the addresses the given mode listens on: reverseProxy is exposed to the
// cluster on every one of listenPorts (e.g. all of a function's published ports); authOnly is reachable
// only from within the pod (loopback), and only ever has a single listen port (see config.go:validatePorts).
func resolveListenAddresses(mode auth.ProxyMode, listenPorts []int) ([]string, error) {
	listenAddresses := make([]string, 0, len(listenPorts))
	for _, listenPort := range listenPorts {
		switch mode {
		case auth.ProxyModeReverseProxy:
			listenAddresses = append(listenAddresses, fmt.Sprintf(":%d", listenPort))
		case auth.ProxyModeAuthOnly:
			listenAddresses = append(listenAddresses, fmt.Sprintf("127.0.0.1:%d", listenPort))
		default:
			return nil, errors.Errorf("Unknown auth-proxy mode: %s", mode)
		}
	}
	return listenAddresses, nil
}

func (s *server) start() error {
	s.logger.InfoWith("Starting auth-proxy", "listenAddress", s.httpServer.Addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return errors.Wrap(err, "Auth-proxy server failed to start")
	}

	return nil
}

// newReverseProxyHandler serves the running-function topology: it authenticates each request and, on
// success, reverse-proxies it to the processor over loopback. The readiness probe is allowlisted.
func newReverseProxyHandler(parentLogger logger.Logger,
	upstreamURL string,
	authenticator authproxy.Authenticator) (http.Handler, error) {

	target, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse upstream URL")
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	reverseProxy.ErrorHandler = func(responseWriter http.ResponseWriter, request *http.Request, err error) {
		parentLogger.WarnWithCtx(request.Context(),
			"Failed to proxy request to upstream",
			"err", err.Error())
		responseWriter.WriteHeader(http.StatusBadGateway)
	}

	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {

		// readiness probe is allowlisted (unauthenticated) so the kubelet can reach the processor
		if request.URL.Path == httptrigger.InternalHealthPath {
			reverseProxy.ServeHTTP(responseWriter, request)
			return
		}

		// on reject the authenticator already wrote the response
		if !authenticator.Authenticate(responseWriter, request) {
			return
		}

		reverseProxy.ServeHTTP(responseWriter, request)
	}), nil
}

// newAuthOnlyHandler serves the DLX /auth decision endpoint. Only /auth is routed; any other path 404s.
// It returns 200 when authorized; on reject the authenticator already wrote the 401/302/403 response.
func newAuthOnlyHandler(authenticator authproxy.Authenticator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(responseWriter http.ResponseWriter, request *http.Request) {
		if authenticator.Authenticate(responseWriter, request) {
			responseWriter.WriteHeader(http.StatusOK)
		}
	})
	return mux
}

// startServers builds and starts one server per listen address, all serving the same handler
// concurrently, so a single auth-proxy process fronts every listen address (e.g. all of a function's ports)
func startServers(ctx context.Context, parentLogger logger.Logger, listenAddresses []string, handler http.Handler) error {
	errorWG, groupCtx := errgroup.WithContext(ctx)
	servers := make([]*server, 0, len(listenAddresses))
	for _, listenAddress := range listenAddresses {
		listenerServer := newServer(parentLogger, listenAddress, handler)
		servers = append(servers, listenerServer)
		errorWG.Go(listenerServer.start)
	}

	stop := context.AfterFunc(groupCtx, func() {
		closeServers(parentLogger, servers)
	})
	defer stop()

	return errorWG.Wait()
}

func closeServers(parentLogger logger.Logger, servers []*server) {
	for _, listenerServer := range servers {
		if err := listenerServer.httpServer.Close(); err != nil && err != http.ErrServerClosed {
			parentLogger.WarnWith("Failed to close auth-proxy listener",
				"listenAddress", listenerServer.httpServer.Addr,
				"err", err.Error())
		}
	}
}
