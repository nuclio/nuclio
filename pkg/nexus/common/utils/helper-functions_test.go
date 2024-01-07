package utils

import (
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/url"
	"testing"
)

type HelperSuite struct {
	suite.Suite
}

func (helperSuite *HelperSuite) TestGetEnvironmentHost() {
	hostEnv := GetEnvironmentHost()

	helperSuite.Equal("host.docker.internal", hostEnv)
}

func (helperSuite *HelperSuite) TestTransformRequestToClientRequest() {
	sampleNexusRequest := &http.Request{
		URL: &url.URL{
			Scheme: "http",
			Host:   "localhost:8070",
			Path:   "/api",
		},
		Method: "GET",
		Header: http.Header{
			headers.FunctionName:    []string{"test"},
			headers.ProcessDeadline: []string{"2020-01-01T00:00:00Z"},
		},
	}

	transformedRequest := TransformRequestToClientRequest(sampleNexusRequest)
	helperSuite.Equal("GET", transformedRequest.Method)
	helperSuite.Equal("test", transformedRequest.Header.Get(headers.FunctionName))
	helperSuite.Empty(transformedRequest.Header.Get(headers.ProcessDeadline))
	helperSuite.Equal("http://host.docker.internal:8070/api", transformedRequest.URL.String())
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(HelperSuite))
}
