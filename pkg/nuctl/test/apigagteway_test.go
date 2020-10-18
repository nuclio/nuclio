/*
Copyright 2017 The Nuclio Authors.

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

package test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"

	"github.com/nuclio/errors"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type apiGatewayCreateGetAndDeleteTestSuite struct {
	Suite
}

func (suite *apiGatewayCreateGetAndDeleteTestSuite) TestCreateGetAndDelete() {
	suite.ensureRunningOnPlatform("kube")

	numOfAPIGateways := 3

	for apiGatewayIdx := 0; apiGatewayIdx < numOfAPIGateways; apiGatewayIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), apiGatewayIdx)

		apiGatewayName := "get-test-apigateway" + uniqueSuffix

		namedArgs := map[string]string{
			"host":                fmt.Sprintf("some-host-%d", apiGatewayIdx),
			"description":         fmt.Sprintf("some-description-%d", apiGatewayIdx),
			"path":                fmt.Sprintf("some-path-%d", apiGatewayIdx),
			"authentication-mode": "basicAuth",
			"basic-auth-username": "basic-username",
			"basic-auth-password": "basic-password",
			"function":            fmt.Sprintf("function-%d", apiGatewayIdx),
			"canary-function":     fmt.Sprintf("canary-function-%d", apiGatewayIdx),
			"canary-percentage":   "25",
		}

		err := suite.ExecuteNuctl([]string{
			"create",
			"apigateway",
			apiGatewayName,
		}, namedArgs)

		suite.Require().NoError(err)

		err = suite.ExecuteNuctl([]string{"get", "apigateway", apiGatewayName, "-o", "yaml"}, nil)
		suite.Require().NoError(err)

		// get all named args values - make sure they're all in the output
		var namedArgsValues []string
		for _, namedArgValue := range namedArgs {
			namedArgsValues = append(namedArgsValues, namedArgValue)
		}
		suite.findPatternsInOutput(namedArgsValues, nil)

		// delete api gateway
		err = suite.ExecuteNuctl([]string{"delete", "apigateway", apiGatewayName}, nil) // nolint: errcheck
		suite.Require().NoError(err)

		// validate deletion
		err = suite.ExecuteNuctl([]string{"get", "apigateway", apiGatewayName, "-o", "yaml"}, nil)
		suite.Require().EqualError(err, "No api gateways found")
	}
}

type apiGatewayInvokeTestSuite struct {
	Suite
}

func (suite *apiGatewayInvokeTestSuite) TestInvokeAuthenticationModeBasicAuth() {
	suite.testInvoke(ingress.AuthenticationModeBasicAuth)
}

func (suite *apiGatewayInvokeTestSuite) TestInvokeAuthenticationModeNone() {
	suite.testInvoke(ingress.AuthenticationModeNone)
}

func (suite *apiGatewayInvokeTestSuite) testInvoke(authenticationMode ingress.AuthenticationMode) {
	suite.ensureRunningOnPlatform("kube")

	functionName := suite.deployFunction()

	// use nutctl to delete the function when we're done
	defer suite.ExecuteNuctl([]string{"delete", "fu", functionName}, nil) // nolint: errcheck

	uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), 1)

	apiGatewayName := "get-test-apigateway" + uniqueSuffix

	apiGatewayHost := suite.getAPIGatewayDefaultHost()
	apiGatewayPath := "/some-path"
	basicAuthUsername := "basic-username"
	basicAuthPassword := "basic-password"
	namedArgs := map[string]string{
		"host":                apiGatewayHost,
		"path":                apiGatewayPath,
		"description":         "some-desc-1",
		"function":            functionName,
		"authentication-mode": string(authenticationMode),
	}

	// fill basic auth args depending on authentication mode
	if authenticationMode == ingress.AuthenticationModeBasicAuth {
		namedArgs["basic-auth-username"] = basicAuthUsername
		namedArgs["basic-auth-password"] = basicAuthPassword
	}

	err := suite.ExecuteNuctl([]string{
		"create",
		"apigateway",
		apiGatewayName,
	}, namedArgs)
	suite.Require().NoError(err)

	defer suite.ExecuteNuctl([]string{"delete", "apigateway", apiGatewayName}, nil) // nolint: errcheck

	expectedResponseBody := "+gnirts siht esrever-"
	apiGatewayURL := fmt.Sprintf("http://%s%s", apiGatewayHost, apiGatewayPath)
	request, err := http.NewRequest("POST", apiGatewayURL, bytes.NewBuffer([]byte("-reverse this string+")))
	suite.Require().NoError(err, "Failed to create new request")

	// prepare request
	request.Header.Set("Content-Type", "application/text")
	if authenticationMode == ingress.AuthenticationModeBasicAuth {
		request.SetBasicAuth(basicAuthUsername, basicAuthPassword)
	}

	// invoke the api-gateway URL to make sure it works (we get the expected function response)
	err = common.RetryUntilSuccessful(20*time.Second, 1*time.Second, func() bool {
		responseBody, err := suite.invokeHTTPRequest(request, &expectedResponseBody)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResponseBody, responseBody)
		return true
	})
	suite.Require().NoError(err)

	if authenticationMode != ingress.AuthenticationModeNone {

		// now try with bad credentials
		if authenticationMode == ingress.AuthenticationModeBasicAuth {
			request.SetBasicAuth(basicAuthUsername, "bad-credentials")
		}

		_, err := suite.invokeHTTPRequest(request, nil)
		suite.Require().Error(err)

	}
}

func (suite *apiGatewayInvokeTestSuite) getAPIGatewayDefaultHost() string {
	defaultTestAPIGatewayHost := common.GetEnvOrDefaultString("NUCTL_TEST_DEFAULT_APIGATEWAY_HOST", "")
	if defaultTestAPIGatewayHost != "" {
		return defaultTestAPIGatewayHost
	}

	// select host address according to system's kubernetes runner (minikube / docker-for-mac)
	if common.GetEnvOrDefaultString("MINIKUBE_HOME", "") != "" {
		return "host.minikube.internal"
	}

	return "kubernetes.docker.internal"
}

func (suite *apiGatewayInvokeTestSuite) deployFunction() string {
	uniqueSuffix := "-" + xid.New().String()
	functionName := "deploy-reverser" + uniqueSuffix
	imageName := "nuclio/processor-" + functionName

	namedArgs := map[string]string{
		"path":    path.Join(suite.GetFunctionsDir(), "common", "reverser", "golang"),
		"runtime": "golang",
		"handler": "main:Reverse",
	}

	err := suite.ExecuteNuctl([]string{"deploy", functionName, "--verbose", "--no-pull"}, namedArgs)

	suite.Require().NoError(err)

	// make sure to clean up after the test
	defer suite.dockerClient.RemoveImage(imageName) // nolint: errcheck

	// try a few times to invoke, until it succeeds
	err = suite.RetryExecuteNuctlUntilSuccessful([]string{"invoke", functionName},
		map[string]string{
			"method": "POST",
			"body":   "-reverse this string+",
			"via":    "external-ip",
		},
		false)
	suite.Require().NoError(err)

	// make sure reverser worked
	suite.Require().Contains(suite.outputBuffer.String(), "+gnirts siht esrever-")
	suite.outputBuffer.Reset()

	return functionName
}

func (suite *apiGatewayInvokeTestSuite) invokeHTTPRequest(request *http.Request,
	expectedBody *string) (string, error) {
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		suite.logger.WarnWith("Failed invoking HTTP request",
			"requestURL", request.URL,
			"requestMethod", request.Method,
			"err", err)
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		suite.logger.WarnWith("Failed while reading response body",
			"requestURL", request.URL,
			"requestMethod", request.Method,
			"err", err)
		return "", err
	}

	if expectedBody != nil {
		if string(body) != *expectedBody {
			suite.logger.WarnWith("Got unexpected response from api gateway",
				"requestURL", request.URL,
				"requestMethod", request.Method,
				"body", string(body))
			return string(body), errors.Errorf("Unexpected body response. received: %s expected %s",
				body, *expectedBody)
		}
	}

	suite.logger.DebugWith("Got expected response", "body", body)
	return string(body), nil
}

func TestAPIGatewayTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(apiGatewayCreateGetAndDeleteTestSuite))
	suite.Run(t, new(apiGatewayInvokeTestSuite))
}
