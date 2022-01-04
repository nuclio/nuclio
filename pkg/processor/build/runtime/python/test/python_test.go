//go:build test_integration && test_local

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
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
	runtime string
	ctx     context.Context
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.ctx = context.Background()

	suite.TestSuite.RuntimeSuite = suite
	suite.TestSuite.ArchivePattern = "python"
	suite.Runtime = suite.runtime
}

func (suite *TestSuite) TestBuildWithBuildArgs() {
	createFunctionOptions := suite.GetDeployOptions("func-with-build-args",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "common", "empty", "python"))
	createFunctionOptions.FunctionConfig.Spec.Handler = "empty:handler"

	// Configure custom pypi repository
	pypiRepositoryURL := "https://test.pypi.org/simple"
	runtimePlatformConfigurationCopy := suite.PlatformConfiguration.Runtime
	suite.PlatformConfiguration.Runtime = &runtimeconfig.Config{
		Python: &runtimeconfig.Python{
			BuildArgs: map[string]string{
				"PIP_INDEX_URL": pypiRepositoryURL,
			},
		},
	}
	defer func() {

		// HACK - reset runtime platform configuration
		// to avoid platform configuration effecting following tests
		// NOTE: on >= 1.6.0 platform configuration would be re-initiated per test case and not per suite.
		suite.PlatformConfiguration.Runtime = runtimePlatformConfigurationCopy
	}()

	// Try to deploy some non-existing package.
	// The deployment will fail but if custom PyPI configuration is successful
	// we should see "Looking in indexes: XXX" message in the logs
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{"pip install non-existing-package"}
	suite.PopulateDeployOptions(createFunctionOptions)
	_, err := suite.Platform.CreateFunction(suite.ctx, createFunctionOptions)
	suite.Assert().NotNil(err)

	// delete leftovers
	defer suite.Platform.DeleteFunction(suite.ctx, &platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: createFunctionOptions.FunctionConfig,
	})
	stackTrace := errors.GetErrorStackString(err, 10)
	suite.Assert().Contains(stackTrace, fmt.Sprintf("Looking in indexes: %s", pypiRepositoryURL))
}

func (suite *TestSuite) TestBuildWithBuildArgsExtended() {
	createFunctionOptions := suite.GetDeployOptions("func-with-build-args-extended",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "common", "empty", "python"))
	createFunctionOptions.FunctionConfig.Spec.Handler = "empty:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{"pip install adbuzdugan"}

	// Create a copy of function options since it's modified during deployment
	createFunctionOptionsOriginal := *createFunctionOptions

	// Sanity, verify deployment attempt without custom pypi repository fails
	suite.DeployFunctionAndExpectError(createFunctionOptions, "Failed to deploy function")

	// Configure custom pypi repository and re-deploy (should succeed)
	runtimePlatformConfigurationCopy := suite.PlatformConfiguration.Runtime
	suite.PlatformConfiguration.Runtime = &runtimeconfig.Config{
		Python: &runtimeconfig.Python{
			BuildArgs: map[string]string{
				"PIP_INDEX_URL": "https://test.pypi.org/simple",
			},
		},
	}

	defer func() {

		// HACK - reset runtime platform configuration
		// to avoid platform configuration effecting following tests
		// NOTE: on >= 1.6.0 platform configuration would be re-initiated per test case and not per suite.
		suite.PlatformConfiguration.Runtime = runtimePlatformConfigurationCopy
	}()

	expectedStatusCode := http.StatusOK
	suite.DeployFunctionAndRequest(&createFunctionOptionsOriginal,
		&httpsuite.Request{
			RequestMethod:              "POST",
			ExpectedResponseStatusCode: &expectedStatusCode,
		})
}

func (suite *TestSuite) TestBuildWithPipCAPath() {
	caCertContents, _, err := common.SendHTTPRequest(nil,
		http.MethodGet,
		"http://curl.haxx.se/ca/cacert.pem",
		nil,
		nil,
		nil,
		http.StatusOK)
	suite.Require().NoError(err, "Failed to obtain curl cacert")

	invalidCACertContents := `-----BEGIN CERTIFICATE-----
MIIDdTCCAl2gAwIBAgILBAAAAAABFUtaw5QwDQYJKoZIhvcNAQEFBQAwVzELMAkGA1UEBhMCQkUx
GTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYTc2ExEDAOBgNVBAsTB1Jvb3QgQ0ExGzAZBgNVBAMTEkds
b2JhbFNpZ24gUm9vdCBDQTAeFw05ODA5mDExMjAwMDBaFw0yODAxMjgxMjAwMDBaMFcxCzAJBgNV
BAYTAkJFMRkwFwYDVQQKExBHbG9IYWxTaWduIG52LXNhMRAwDgYDVQQLEwdSb290IENBMRswGQYD
VQQDExJHbG9iYWxTaWduIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDa
DuaZjc6j40+Kfvvxi4Mla+pIH/EqsLmVEQS98GPR4mdmzxzdzxtIK+6NiY6arymAZavpxy0Sy6sc
THAHoT0KMM0VjU/43dSMUBUc71DuxC73/OlS8pF94G3VNTCOXkNz8kHp1Wrjsok6Vjk4bwY8iGlb
Kk3Fp1S4bInMm/k8yunarilSPJJ4ltbcdG6TRGHRjcdGsnUOhugZitVtbNV4FpWi6cgKOOvyJBNP
c1STE4U6G7weNLWLBYy5d4ux2x8gkasJU26Qzns3dLlwR5EiUWMWea6xrkEmCMgZK9FGqkjWZCrX
gzT/LCrBbBlDSgeF59N89iFo7+ryUp9/k5DPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNV
HRMBAf8EBTADAQH/MB0GA1UdDgQWBBRge2YaRQ2XyonarilEzTSo//z9SzANBgkqhkiG9w0BAQUF
AAOCAQnarilnfE920I2/7LqivjTFKDK1fPxsnCwrvQmeU79rXqoRSLblCKOzyj1hTdNGCbM+w6Dj
Y1Ub8rrvrTnhQ7k4o+YviiY776BQVvnGCv04zcQLcFGUlnarilNflNUVyRRBnMRddWQVDf9VMOyG
j/8N7yy5Y0b2qvzfvGn9LhJIZJrglfCm7ymPAbEVtQwdpf5pLGkkeB6zpxxxYu7KyJesF12KwvhH
hm4qxFYxldBniYUr+WymXUadDKqC5JlR3XC321Y9YeRq4VzW9v493kHMB65jUr9TU/Qr6cf9tveC
X4XSQRjbgbMEHMUfppIBvFSDJ3gyICh3WZlXi/EjJKSZp4A==
-----END CERTIFICATE-----`

	validCAFile := suite.writeFile("nuclio-curl-ca-cert", caCertContents)
	invalidCAFile := suite.writeFile("nuclio-curl-ca-cert-invalid", []byte(invalidCACertContents))

	// remove leftovers
	defer os.Remove(validCAFile.Name())
	defer os.Remove(invalidCAFile.Name())

	createFunctionOptions := suite.GetDeployOptions("pio-ca-file",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "common", "empty", "python"))
	createFunctionOptions.FunctionConfig.Spec.Handler = "empty:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{"pip install linkchecker"}

	// Create a copy of function options since it's modified during deployment
	createFunctionOptionsOriginal := *createFunctionOptions

	// Configure pip to work with a custom invalid ca (should fail)
	runtimePlatformConfigurationCopy := suite.PlatformConfiguration.Runtime
	suite.PlatformConfiguration.Runtime = &runtimeconfig.Config{
		Python: &runtimeconfig.Python{
			PipCAPath: invalidCAFile.Name(),
		},
	}

	defer func() {

		// HACK - reset runtime platform configuration
		// to avoid platform configuration effecting following tests
		// NOTE: on >= 1.6.0 platform configuration would be re-initiated per test case and not per suite.
		suite.PlatformConfiguration.Runtime = runtimePlatformConfigurationCopy
	}()

	// Sanity, verify deployment attempt with invalid ca fails
	suite.DeployFunctionAndExpectError(createFunctionOptions, "Failed to deploy function")

	// Configure pip to work with a custom ca (should succeed)
	suite.PlatformConfiguration.Runtime = &runtimeconfig.Config{
		Python: &runtimeconfig.Python{
			PipCAPath: validCAFile.Name(),
		},
	}

	expectedStatusCode := http.StatusOK
	suite.DeployFunctionAndRequest(&createFunctionOptionsOriginal,
		&httpsuite.Request{
			RequestMethod:              "POST",
			ExpectedResponseStatusCode: &expectedStatusCode,
		})

}

func (suite *TestSuite) GetFunctionInfo(functionName string) buildsuite.FunctionInfo {
	functionInfo := buildsuite.FunctionInfo{
		Runtime: suite.runtime,
	}

	switch functionName {

	case "reverser":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "reverser", "python", "reverser.py"}
		functionInfo.Handler = "reverser:handler"

	case "json-parser-with-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-function-config", "python"}

	case "json-parser-with-inline-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-inline-function-config", "python", "parser.py"}

	case "invalid-inline-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "invalid-inline-config", "python", "parser.py"}

	case "long-initialization":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "long-initialization", "python", "sleepy.py"}

	case "context-init-fail":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "context-init-fail", "python", "contextinitfail.py"}

	default:
		suite.Logger.InfoWith("Test skipped", "functionName", functionName)

		functionInfo.Skip = true
	}

	return functionInfo
}

func (suite *TestSuite) writeFile(filenamePattern string, contents []byte) *os.File {
	tmpFile, err := ioutil.TempFile("", filenamePattern)
	suite.Require().NoError(err)
	suite.Require().NoError(tmpFile.Close())

	err = ioutil.WriteFile(tmpFile.Name(), contents, 0644)
	suite.Require().NoError(err)
	return tmpFile
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	for _, testCase := range []struct {
		runtimeName string
	}{
		{runtimeName: "python:3.6"},
		{runtimeName: "python:3.7"},
		{runtimeName: "python:3.8"},
		{runtimeName: "python:3.9"},
	} {
		t.Run(testCase.runtimeName, func(t *testing.T) {
			testSuite := new(TestSuite)
			testSuite.runtime = testCase.runtimeName
			suite.Run(t, testSuite)
		})
	}
}
