//go:build test_functional && test_kube

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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube/test/kubectlclient"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

// PlatformTestSuite requires
// - minikube >= 1.22.0 (https://minikube.sigs.k8s.io/docs/start/) with a preinstalled cluster. e.g.:
//	  > minikube start --profile nuclio-test --kubernetes-version v1.23.8 --driver docker --addons registry --ports=127.0.0.1:30060:30060
// - helm >= 3.3.0 (https://helm.sh/docs/intro/install/)
type PlatformTestSuite struct {
	suite.Suite
	logger          logger.Logger
	cmdRunner       cmdrunner.CmdRunner
	registryURL     string
	minikubeProfile string
	namespace       string
	backendAPIURL   string
}

func (suite *PlatformTestSuite) SetupSuite() {
	var err error
	common.SetVersionFromEnv()

	suite.logger, err = nucliozap.NewNuclioZapTest("platform-functional-test")
	suite.Require().NoError(err, "Failed to create logger")

	suite.cmdRunner, err = cmdrunner.NewShellRunner(suite.logger)
	suite.Require().NoError(err, "Failed to create shell runner")

	suite.namespace = common.GetEnvOrDefaultString("NUCLIO_TEST_NAMESPACE", "nuclio-test")
	suite.minikubeProfile = common.GetEnvOrDefaultString("NUCLIO_TEST_MINIKUBE_PROFILE", "nuclio-test")

	// assumes that minikube exposed the backend API on port 30060
	suite.backendAPIURL = common.GetEnvOrDefaultString("NUCLIO_TEST_BACKEND_API_URL", "http://localhost:30060/api")
	suite.registryURL = suite.resolveInClusterRegistryURL()
}

func (suite *PlatformTestSuite) SetupTest() {
	suite.installNuclioHelmChart()
}

func (suite *PlatformTestSuite) TearDownTest() {

	// cleanup nuclio via helm
	suite.executeHelm([]string{"delete", "nuclio"}, nil)

	// delete namespace
	suite.executeKubectl([]string{"delete", "namespace", suite.namespace}, nil)
}

func (suite *PlatformTestSuite) TestBuildAndDeployFunctionWithKaniko() {

	// set nuclio to build with kaniko
	suite.executeHelm([]string{"upgrade", "nuclio", "hack/k8s/helm/nuclio", "--wait", "--install", "--reuse-values"},
		map[string]string{
			"set": common.StringMapToString(map[string]string{
				"dashboard.containerBuilderKind":        "kaniko",
				"dashboard.kaniko.insecurePushRegistry": "true",
				"dashboard.kaniko.insecurePullRegistry": "true",
			}),
		})

	// generate function config
	functionConfig := suite.compileFunctionConfig()

	// create function
	suite.createFunction(functionConfig)
	suite.waitForFunctionToBeReady(functionConfig)
}

func (suite *PlatformTestSuite) compileFunctionConfig() *functionconfig.Config {
	functionConfig := functionconfig.NewConfig()
	functionConfig.Meta.Namespace = suite.namespace
	functionConfig.Meta.Name = "test-func" + xid.New().String()
	functionConfig.Spec.RunRegistry = suite.registryURL
	functionConfig.Spec.Build.Registry = suite.registryURL
	functionConfig.Spec.Handler = "main:handler"
	functionConfig.Spec.Runtime = "python:3.8"
	functionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  return "hello world"
`))
	functionConfig.Spec.Build.NoBaseImagesPull = true
	return functionConfig
}

func (suite *PlatformTestSuite) executeKubectl(positionalArgs []string,
	namedArgs map[string]string) cmdrunner.RunResult {
	if namedArgs == nil {
		namedArgs = map[string]string{}
	}

	if _, found := namedArgs["namespace"]; !found {
		namedArgs["namespace"] = suite.namespace
	}
	runOptions := kubectlclient.NewRunOptions(kubectlclient.WithMinikubeKubectlCommandRunner(suite.minikubeProfile))
	results, err := kubectlclient.RunKubectlCommand(suite.cmdRunner, positionalArgs, namedArgs, runOptions)
	suite.Require().NoError(err)
	return results
}

func (suite *PlatformTestSuite) executeHelm(positionalArgs []string,
	namedArgs map[string]string) string {

	positionalArgs = append([]string{"helm"}, positionalArgs...)
	if namedArgs == nil {
		namedArgs = map[string]string{}
	}
	if _, found := namedArgs["namespace"]; !found {
		namedArgs["namespace"] = suite.namespace
	}

	nuclioSourceDir := common.GetSourceDir()
	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &nuclioSourceDir,
	}
	results, err := suite.cmdRunner.RunWithPositionalAndNamedArguments(runOptions, positionalArgs, namedArgs)
	suite.Require().NoError(err)
	return results.Output
}

func (suite *PlatformTestSuite) executeMinikube(positionalArgs []string,
	namedArgs map[string]string) string {

	if len(positionalArgs) == 0 {
		positionalArgs = []string{"minikube"}
	} else {
		positionalArgs = append([]string{"minikube"}, positionalArgs...)
	}

	if namedArgs == nil {
		namedArgs = map[string]string{}
	}

	// auto infer if set
	if suite.namespace != "" {
		namedArgs["namespace"] = suite.namespace
	}

	if suite.minikubeProfile != "" {
		namedArgs["profile"] = suite.minikubeProfile
	}

	results, err := suite.cmdRunner.RunWithPositionalAndNamedArguments(nil, positionalArgs, namedArgs)
	suite.Require().NoError(err)
	return results.Output
}

func (suite *PlatformTestSuite) resolveInClusterRegistryURL() string {

	// resolve the service ip and not using the service "registry.kube-system.svc.cluster.local" as
	// https://github.com/kubernetes/minikube/issues/2162 is still open
	results := suite.executeKubectl([]string{"get", "service/registry"}, map[string]string{
		"output":    "jsonpath='{.spec.clusterIP}'",
		"namespace": "kube-system",
	})
	return results.Output

	// For Host
	//// returns 127.0.0.1:<host-port>
	//result, err := suite.cmdRunner.Run(nil, fmt.Sprintf("docker port %s 5000", suite.minikubeProfile))
	//suite.Require().NoError(err)
	//return fmt.Sprintf("host.minikube.internal:%s", strings.TrimSpace(strings.Split(result.Output, ":")[1]))
}

func (suite *PlatformTestSuite) installNuclioHelmChart() {
	renderedHelmValues, err := suite.cmdRunner.Run(&cmdrunner.RunOptions{
		Env: map[string]string{
			"NUCLIO_LABEL": common.GetEnvOrDefaultString("NUCLIO_LABEL", "unstable"),
			"REPO":         common.GetEnvOrDefaultString("REPO", "quay.io"),
			"REPO_NAME":    common.GetEnvOrDefaultString("REPO_NAME", "nuclio"),
			"PULL_POLICY":  common.GetEnvOrDefaultString("PULL_POLICY", "IfNotPresent"),
		},
	},
		fmt.Sprintf("cat %s/test/k8s/ci_assets/helm_values.yaml | envsubst", common.GetSourceDir()))
	suite.Require().NoError(err)

	nuclioSourceDir := common.GetSourceDir()

	_, err = suite.cmdRunner.Run(&cmdrunner.RunOptions{
		WorkingDir: &nuclioSourceDir,
		Stdin:      &renderedHelmValues.Output,
	}, fmt.Sprintf("helm "+
		"--namespace %s "+
		"install "+
		"--create-namespace "+
		"--debug "+
		"--wait "+
		"--set dashboard.nodePort=30060 "+
		"--values "+
		"- "+
		"nuclio hack/k8s/helm/nuclio", suite.namespace))
	suite.Require().NoError(err)
}

func (suite *PlatformTestSuite) waitForFunctionToBeReady(functionConfig *functionconfig.Config) {
	var function *functionconfig.ConfigWithStatus
	err := common.RetryUntilSuccessful(10*time.Minute, 10*time.Second,
		func() bool {
			var err error
			function, err = suite.getFunction(functionConfig.Meta.Name)
			if nuclioError, ok := err.(nuclio.WithStatusCode); ok {
				if nuclioError.StatusCode() >= 500 {

					// bail here, not expected server error
					suite.Require().Error(err)
				}

				suite.logger.WarnWith("Function not ready yet",
					"function", functionConfig.Meta.Name,
					"error", err)
				return false
			}

			suite.logger.DebugWith("Waiting for function to be ready",
				"functionState", function.Status.State,
				"function", functionConfig.Meta.Name)
			return functionconfig.FunctionStateProvisioned(function.Status.State)
		})

	suite.Require().Equal(functionconfig.FunctionStateReady, function.Status.State)
	suite.Require().NoError(err)
}

func (suite *PlatformTestSuite) createFunction(functionConfig *functionconfig.Config) {
	encodedFunctionConfig, err := json.Marshal(functionConfig)
	suite.Require().NoError(err)

	_, _, err = common.SendHTTPRequest(nil,
		http.MethodPost,
		suite.backendAPIURL+"/functions",
		encodedFunctionConfig,
		nil,
		nil,
		http.StatusAccepted)
	suite.Require().NoError(err, "Failed to create function")
}

func (suite *PlatformTestSuite) getFunction(functionName string) (*functionconfig.ConfigWithStatus, error) {
	responseBody, response, err := common.SendHTTPRequest(nil,
		http.MethodGet,
		fmt.Sprintf("%s/functions/%s", suite.backendAPIURL, functionName),
		nil,
		nil,
		nil,
		0)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	if response.StatusCode != http.StatusOK {
		return nil, nuclio.GetByStatusCode(response.StatusCode)("Failed to get function")
	}

	functionConfig := &functionconfig.ConfigWithStatus{}
	if err := json.Unmarshal(responseBody, functionConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal function")
	}
	return functionConfig, nil

}

func TestPlatformFunctionalTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(PlatformTestSuite))
}
