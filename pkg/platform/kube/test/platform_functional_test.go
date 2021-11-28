// +build test_functional
// +build test_kube

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
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nucliozap "github.com/nuclio/zap"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
)

// PlatformTestSuite requires
// - minikube >= 1.22.0 (https://minikube.sigs.k8s.io/docs/start/) with a preinstalled cluster. e.g.:
//			minikube start --profile nuclio-test --kubernetes-version v1.20.11 --driver docker --addons registry --addons ingress
// - helm >= 3.3.0 (https://helm.sh/docs/intro/install/)
type PlatformTestSuite struct {
	suite.Suite
	logger          logger.Logger
	cmdRunner       cmdrunner.CmdRunner
	registryURL     string
	minikubeProfile string
	namespace       string
}

func (suite *PlatformTestSuite) SetupSuite() {
	var err error
	common.SetVersionFromEnv()

	suite.logger, err = nucliozap.NewNuclioZapTest("platform-functional-test")
	suite.Require().NoError(err, "Failed to create logger")

	suite.cmdRunner, err = cmdrunner.NewShellRunner(suite.logger)
	suite.Require().NoError(err, "Failed to create shell runner")

	suite.minikubeProfile = common.GetEnvOrDefaultString("NUCLIO_TEST_MINIKUBE_PROFILE", "nuclio-test")

	suite.registryURL = suite.resolveMinikubeRegistryURL()
}

func (suite *PlatformTestSuite) SetupTest() {
	var err error
	suite.namespace = fmt.Sprintf("test-nuclio-%s",
		common.GenerateRandomString(5, common.SmallLettersAndNumbers))

	renderedHelmValues, err := suite.cmdRunner.Run(nil,
		fmt.Sprintf("cat %s/test/k8s/ci_assets/helm_values.yaml | envsubst", common.GetSourceDir()))
	suite.Require().NoError(err)

	_, err = suite.cmdRunner.Run(&cmdrunner.RunOptions{
		Stdin: &renderedHelmValues.Output,
	}, "helm install --debug --wait --values - nuclio hack/k8s/helm/nuclio")
	suite.Require().NoError(err)
}

func (suite *PlatformTestSuite) TearDownTest() {

	suite.executeHelm([]string{"delete", "nuclio"}, nil)

	// delete all resources
	suite.executeKubectl([]string{"delete", "all"},
		map[string]string{
			"selector": "nuclio.io/app",
		})

	// delete namespace
	suite.executeKubectl([]string{"delete", "namespace", suite.namespace}, nil)
}

func (suite *PlatformTestSuite) TestBuildAndDeployFunctionWithKaniko() {
	suite.executeHelm([]string{"upgrade", "nuclio", "hack/k8s/helm/nuclio", "--install", "--reuse-values"},
		map[string]string{
			"set": common.StringMapToString(map[string]string{
				"dashboard.containerBuilderKind":        "kaniko",
				"dashboard.kaniko.insecurePushRegistry": "true",
				"dashboard.kaniko.insecurePullRegistry": "true",
			}),
		})
	functionConfig := functionconfig.NewConfig()
	functionConfig.Meta.Namespace = suite.namespace
	functionConfig.Meta.Name = "build-deploy-with-kaniko"
	functionConfig.Spec.RunRegistry = suite.registryURL
	functionConfig.Spec.Build.Registry = suite.registryURL
	functionConfig.Spec.Handler = "main:handler"
	functionConfig.Spec.Runtime = "python:3.8"
	functionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  return "hello world"
`))
	functionConfig.Spec.Build.NoBaseImagesPull = true

	suite.executeKubectl([]string{"port-forward", "nuclio-dashboard", "8070:8070"}, map[string]string{})
	encodedFunctionConfig, err := json.Marshal(functionConfig)
	suite.Require().NoError(err)

	_, _, err = common.SendHTTPRequest(nil,
		http.MethodPost,
		"localhost:8070",
		encodedFunctionConfig,
		nil,
		nil,
		http.StatusAccepted)
	suite.Require().NoError(err)

}

func (suite *PlatformTestSuite) executeKubectl(positionalArgs []string,
	namedArgs map[string]string) cmdrunner.RunResult {
	if namedArgs == nil {
		namedArgs = map[string]string{}
	}
	namedArgs["namespace"] = suite.namespace
	results, err := runKubectlCommand(suite.logger, suite.cmdRunner, positionalArgs, namedArgs, runKubectlCommandMinikube, nil)
	suite.Require().NoError(err)
	return results
}

func (suite *PlatformTestSuite) executeHelm(positionalArgs []string,
	namedArgs map[string]string) string {
	positionalArgs = append(positionalArgs, "helm")
	if namedArgs == nil {
		namedArgs = map[string]string{}
	}
	namedArgs["namespace"] = suite.namespace
	results, err := runCommand(suite.logger, suite.cmdRunner, positionalArgs, namedArgs, nil)
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

	results, err := runCommand(suite.logger, suite.cmdRunner, positionalArgs, namedArgs, nil)
	suite.Require().NoError(err)
	return results.Output
}

func (suite *PlatformTestSuite) resolveMinikubeRegistryURL() string {
	minikubeIP := suite.executeMinikube([]string{"ip"}, nil)
	result, err := suite.cmdRunner.Run(nil, "docker port minikube 5000")
	suite.Require().NoError(err)
	return fmt.Sprintf("%s:%s", minikubeIP, strings.TrimSpace(strings.Split(result.Output, ":")[1]))
}

func TestPlatformFunctionalTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(PlatformTestSuite))
}
