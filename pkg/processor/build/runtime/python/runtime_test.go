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

package python

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/version"

	"github.com/stretchr/testify/suite"
)

type PythonTestSuite struct {
	suite.Suite
}

func (suite *PythonTestSuite) TestBaseImageName() {

	for _, params := range []struct {
		runtimeVersion    string
		baseImageName     string
		label             string
		arch              string
		expectedBaseImage string
	}{
		{
			"",
			"",
			"latest",
			"amd64",
			"nuclio/processor-py3.6-alpine:latest-amd64",
		},
		{
			"2.7",
			"",
			"latest",
			"amd64",
			"nuclio/processor-py2.7-alpine:latest-amd64",
		},
		{
			"2.7",
			"jessie",
			"latest",
			"amd64",
			"nuclio/processor-py2.7-jessie:latest-amd64",
		},
		{
			"",
			"jessie",
			"latest",
			"amd64",
			"nuclio/processor-py3.6-jessie:latest-amd64",
		},
		{
			"",
			"",
			"label",
			"arch",
			"nuclio/processor-py3.6-alpine:label-arch",
		},
		{
			"3.1",
			"",
			"label",
			"arch",
			"",
		},
		{
			"",
			"foo",
			"label",
			"arch",
			"",
		},
		{
			"3.6",
			"jessie",
			"latest",
			"amd64",
			"nuclio/processor-py3.6-jessie:latest-amd64",
		},
	} {
		versionInfo := version.Info{
			Label: params.label,
			Arch:  params.arch,
		}

		baseImageName, err := getBaseImageName(&versionInfo, params.runtimeVersion, params.baseImageName)

		if params.expectedBaseImage == "" {
			suite.Require().Error(err)
		} else {
			suite.Require().Equal(params.expectedBaseImage, baseImageName)
			suite.Require().NoError(err)
		}
	}
}

func TestWriterTestSuite(t *testing.T) {
	suite.Run(t, new(PythonTestSuite))
}
