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

type functionEventGetTestSuite struct {
	Suite
}

// leaving in function deploy + teardown so that invoke can be tested too later
func (suite *functionEventGetTestSuite) TestGet() {
	numOfFunctionEvents := 3
	var functionEventNames []string

	for functionEventIdx := 0; functionEventIdx < numOfFunctionEvents; functionEventIdx++ {
		uniqueSuffix := fmt.Sprintf("-%s-%d", xid.New().String(), functionEventIdx)

		functionEventName := "get-test-functionevent" + uniqueSuffix

		// add function event name to list
		functionEventNames = append(functionEventNames, functionEventName)

		namedArgs := map[string]string{
			"function":     fmt.Sprintf("function-%d", functionEventIdx),
			"display-name": fmt.Sprintf("display-name-%d", functionEventIdx),
			"trigger-name": fmt.Sprintf("trigger-name-%d", functionEventIdx),
			"trigger-kind": fmt.Sprintf("trigger-kind-%d", functionEventIdx),
			"body":         fmt.Sprintf("body-%d", functionEventIdx),
		}

		err := suite.ExecuteNutcl([]string{
			"create",
			"functionevent",
			functionEventName,
		}, namedArgs)

		suite.Require().NoError(err)

		// cleanup
		defer func(functionEventName string) {

			// use nutctl to delete the function event when we're done
			suite.ExecuteNutcl([]string{"delete", "fe", functionEventName}, nil)

		}(functionEventName)
	}

	err := suite.ExecuteNutcl([]string{"get", "functionevent"}, nil)
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput(functionEventNames, nil)

	// get all function events for function-2
	err = suite.ExecuteNutcl([]string{"get", "functionevent"}, map[string]string{"function": "function-1"})
	suite.Require().NoError(err)

	// find function names in get result
	suite.findPatternsInOutput([]string{functionEventNames[1]},
		[]string{functionEventNames[0], functionEventNames[2]})

	// delete the second function event
	err = suite.ExecuteNutcl([]string{"delete", "fe", functionEventNames[1]}, nil)
	suite.Require().NoError(err)

	// get again
	err = suite.ExecuteNutcl([]string{"get", "functionevent"}, nil)
	suite.Require().NoError(err)

	// verify second function event deleted
	suite.findPatternsInOutput([]string{
		functionEventNames[0], functionEventNames[2],
	}, []string{
		functionEventNames[1],
	})
}

func TestFunctionEventTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(functionEventGetTestSuite))
}
