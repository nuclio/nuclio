/*
Copyright 2018 The Nuclio Authors.

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

package http

import (
	"context"
	"net"
	nethttp "net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	httpcors "github.com/nuclio/nuclio/pkg/processor/trigger/http/cors"

	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type TestSuite struct {
	processorsuite.TestSuite
	trigger             http
	fastDummyHttpServer *fasthttputil.InmemoryListener
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
	suite.trigger = http{
		AbstractTrigger: trigger.AbstractTrigger{
			Logger:          suite.Logger,
			WorkerAllocator: nil,
		},
		events: []Event{},
		configuration: &Configuration{
			Configuration: trigger.Configuration{
				Trigger: functionconfig.Trigger{
					URL: ":0", // let OS pick a port for me
				},
				ID: "999",
			},
			ReadBufferSize: DefaultReadBufferSize,
			CORS:           *httpcors.NewCORS(),
		},
	}
	suite.fastDummyHttpServer = fasthttputil.NewInmemoryListener()
	go func() {
		err := fasthttp.Serve(suite.fastDummyHttpServer, suite.trigger.handleRequest())
		if err != nil {
			suite.Require().NoError(err, "Failed to serve")
		}
	}()
}

func (suite *TestSuite) TearDownSuite() {
	suite.fastDummyHttpServer.Close() // nolint: errcheck
}

func (suite *TestSuite) TestCORS() {
	client := suite.getClient()
	for _, testCase := range []struct {
		CORSAllowOrigin            string
		RequestOrigin              string
		RequestMethod              string
		RequestHeaders             []string
		ExpectedResponseStatusCode int
		ExpectedResponseHeaders    map[string]string
	}{

		// happy flow
		{
			CORSAllowOrigin: "foo.bar",
			RequestOrigin:   "foo.bar",
			RequestMethod:   "GET",
			RequestHeaders: []string{
				"X-Nuclio-log-level",
			},
			ExpectedResponseStatusCode: fasthttp.StatusOK,
			ExpectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "foo.bar",
				"Access-Control-Allow-Methods": "HEAD, GET, POST, PUT, DELETE, OPTIONS",
				"Access-Control-Max-Age":       "-1",
				"Access-Control-Allow-Headers": "Accept, Content-Length, Content-Type, X-nuclio-log-level",
			},
		},

		// invalid origin
		{
			CORSAllowOrigin:            "foo.bar",
			RequestOrigin:              "baz.bar",
			RequestMethod:              "GET",
			ExpectedResponseStatusCode: fasthttp.StatusBadRequest,
		},

		// invalid request header
		{
			RequestOrigin: "foo.bar",
			RequestMethod: "GET",
			RequestHeaders: []string{
				"Not-supported-header",
			},
			ExpectedResponseStatusCode: fasthttp.StatusBadRequest,
		},

		// invalid request method
		{
			RequestOrigin:              "foo.bar",
			RequestMethod:              "ABC",
			ExpectedResponseStatusCode: fasthttp.StatusBadRequest,
		},
	} {
		suite.Logger.DebugWith("Testing CORS", "testCase", testCase)

		// set cors configuration
		cors := httpcors.NewCORS()
		if testCase.CORSAllowOrigin != "" {
			cors.AllowOrigin = testCase.CORSAllowOrigin
		}
		suite.trigger.configuration.CORS = *cors

		// create request, use OPTIONS to trigger preflight flow
		request, err := nethttp.NewRequest(fasthttp.MethodOptions, "http://foo.bar/", nil)
		suite.NoError(err, "Failed to create new request")

		// set preflight required headers
		request.Header.Set("Origin", testCase.RequestOrigin)
		request.Header.Set("Access-Control-Request-Method", testCase.RequestMethod)
		for _, headerName := range testCase.RequestHeaders {
			request.Header.Add("Access-Control-Request-Headers", headerName)
		}

		// do request
		response, err := client.Do(request)
		suite.Require().NoError(err, "Failed to do request")
		suite.Logger.DebugWith("Received response",
			"headers", response.Header,
			"statusCode", response.StatusCode)

		// check for response status code
		suite.Equal(testCase.ExpectedResponseStatusCode, response.StatusCode)

		// check for response headers
		for headerName, headerValue := range testCase.ExpectedResponseHeaders {
			suite.Equal(response.Header.Get(headerName), headerValue)
		}
	}

}

func (suite *TestSuite) getClient() *nethttp.Client {
	return &nethttp.Client{
		Transport: &nethttp.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return suite.fastDummyHttpServer.Dial()
			},
		},
	}
}

func TestHTTPSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
