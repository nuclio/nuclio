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
	"fmt"
	"testing"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type apiGatewayCreateGetAndDeleteTestSuite struct {
	Suite
}

func (suite *apiGatewayCreateGetAndDeleteTestSuite) TestCreateGetAndDelete() {
	suite.ensureRunningOnPlatform("kube")

	numOfAPIGateways := 3
	var apiGatewayNames []string

	for apiGatewayIdx := 0; apiGatewayIdx < numOfAPIGateways; apiGatewayIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), apiGatewayIdx)

		apiGatewayName := "get-test-apigateway" + uniqueSuffix

		// add api gateway name to list
		apiGatewayNames = append(apiGatewayNames, apiGatewayName)

		namedArgs := map[string]string{
			"host":     fmt.Sprintf("some-host-%d", apiGatewayIdx),
			"description":     fmt.Sprintf("some-description-%d", apiGatewayIdx),
			"path":     fmt.Sprintf("some-path-%d", apiGatewayIdx),
			"authentication-mode": "basicAuth",
			"basic-auth-username": "basic-username",
			"basic-auth-password": "basic-password",
			"function": fmt.Sprintf("function-%d", apiGatewayIdx),
			"canary-function": fmt.Sprintf("canary-function-%d", apiGatewayIdx),
			"canary-percentage": "25",
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

func TestAPIGatewayTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(apiGatewayCreateGetAndDeleteTestSuite))
}
