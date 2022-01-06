package middleware

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/nuclio/logger"
)

type ContextKey int

const (
	iguazioContextHeaderName = "igz-ctx"

	IguazioContextKey ContextKey = iota

	IguazioHeaderPrefix = "x-igz"
)

// RequestID is a middleware that injects a request ID into the context of each
// request. It first tries to see if it received an Iguazio context ID and use it, alternatively fallback
// to server framework defaults
func RequestID(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if requestID := r.Header.Get(iguazioContextHeaderName); requestID != "" {

			// for logging purposes
			ctx = context.WithValue(ctx, middleware.RequestIDKey, requestID)

			// for usability
			ctx = context.WithValue(ctx, IguazioContextKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// use framework defaults
		middleware.
			RequestID(next).
			ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// AlignRequestIDKeyToZapLogger transform server framework request ID to Nuclio Zap's logger context value for
// a unique request ID
func AlignRequestIDKeyToZapLogger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if requestID := ctx.Value(middleware.RequestIDKey); requestID != nil {

			// TODO: make logger bind context and log it
			ctx = context.WithValue(ctx, "RequestID", requestID) // nolint: staticcheck
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// RequestResponseLogger logs handled requests
func RequestResponseLogger(logger logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, request *http.Request) {
			responseBodyBuffer := bytes.Buffer{}

			// create a response wrapper so we can access stuff
			responseWrapper := middleware.NewWrapResponseWriter(w, request.ProtoMajor)
			responseWrapper.Tee(&responseBodyBuffer)

			// take start time
			requestStartTime := time.Now()

			// get request body
			requestBody, _ := ioutil.ReadAll(request.Body)

			// restore body for further processing
			request.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))

			// when request processing is done, log the request / response
			defer func() {
				logVars := []interface{}{
					"requestMethod", request.Method,
					"requestPath", request.URL,
					"requestHeaders", request.Header,
					"requestBody", string(requestBody),
					"responseStatus", responseWrapper.Status(),
					"responseTime", time.Since(requestStartTime),
				}

				// response body is too spammy
				if !common.StringSliceContainsStringPrefix([]string{
					"/api/functions",
					"/api/function_templates",
				}, strings.TrimSuffix(request.URL.Path, "/")) {
					logVars = append(logVars, "responseBody", responseBodyBuffer.String())
				}

				logger.DebugWithCtx(request.Context(), "Handled request", logVars...)
			}()

			// call next middleware
			next.ServeHTTP(responseWrapper, request)
		}

		return http.HandlerFunc(fn)
	}
}

// ModifyIguazioRequestHeaderPrefix removes 'igz' from incoming request headers
func ModifyIguazioRequestHeaderPrefix(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		newHeaderMap := http.Header{}
		for headerName, headerValues := range r.Header {
			newHeaderName := headerName
			if strings.HasPrefix(strings.ToLower(headerName), IguazioHeaderPrefix) {
				newHeaderName = "x" + strings.TrimPrefix(strings.ToLower(headerName), IguazioHeaderPrefix)
			}
			newHeaderMap[newHeaderName] = headerValues
		}
		r.Header = newHeaderMap

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}
