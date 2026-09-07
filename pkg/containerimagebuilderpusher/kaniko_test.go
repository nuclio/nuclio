//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package containerimagebuilderpusher

import (
	"context"
	"testing"

	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type KanikoTestSuite struct {
	suite.Suite
	logger logger.Logger
	kaniko *Kaniko
}

func (suite *KanikoTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.T().Setenv("NUCLIO_DASHBOARD_DEPLOYMENT_NAME", "nuclio-dashboard")
	dashboardDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "nuclio-dashboard", Namespace: "default", UID: "dashboard-uid"},
	}

	suite.kaniko = &Kaniko{
		jobRunner: &jobRunner{
			builderName:   KanikoKind,
			logger:        suite.logger,
			kubeClientSet: kube.NewClientWithRetryFromClient(k8sfake.NewClientset(dashboardDeployment)),
			builderConfiguration: &ContainerBuilderConfiguration{
				BusyBoxImage: "busybox:stable",
				AuthConfig: registryhelpers.AuthConfig{
					AWSCLIImage: "amazon/aws-cli:2.17.16",
				},
				Kaniko: KanikoConfig{
					Image:           "gcr.io/kaniko-project/executor:latest",
					ImagePullPolicy: "IfNotPresent",
				},
			},
		},
		awsHelper: &registryhelpers.AWSHelper{},
	}
}

func (suite *KanikoTestSuite) newBuildOptions() *BuildOptions {
	return &BuildOptions{
		Image:       "my-func:latest",
		ContextDir:  "/some/context",
		RegistryURL: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		RepoName:    "my-func",
		DockerfileInfo: &runtime.ProcessorDockerfileInfo{
			DockerfilePath: "/some/context/Dockerfile",
		},
	}
}

func createReposContainer(initContainers []v1.Container) *v1.Container {
	for i := range initContainers {
		if initContainers[i].Name == "create-repos" {
			return &initContainers[i]
		}
	}
	return nil
}

func (suite *KanikoTestSuite) TestConfigureRegistryAuthenticationECRPathSuffixedRegistryURL() {
	buildOptions := suite.newBuildOptions()
	buildOptions.RegistryURL = "934638699319.dkr.ecr.us-east-2.amazonaws.com/iguazio-cloud/qa/vmdev214.lab.iguazeng.com"
	buildOptions.RepoName = "iguazio-cloud/qa/vmdev214.lab.iguazeng.com/evfnivykxu-yxizqvjg-processor"

	jobSpec, err := suite.kaniko.compileJobSpec(context.Background(), "default", buildOptions, "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	createRepos := createReposContainer(podSpec.InitContainers)
	suite.Require().NotNil(createRepos, "expected an ECR create-repos init container for a path-suffixed ECR registry URL")

	command := createRepos.Args[1]
	suite.Contains(command, "aws ecr create-repository --repository-name iguazio-cloud/qa/vmdev214.lab.iguazeng.com/evfnivykxu-yxizqvjg-processor --region us-east-2 --registry-id 934638699319")
	suite.Contains(command, "aws ecr create-repository --repository-name iguazio-cloud/qa/vmdev214.lab.iguazeng.com/evfnivykxu-yxizqvjg-processor/cache --region us-east-2 --registry-id 934638699319")
}

func (suite *KanikoTestSuite) TestConfigureRegistryAuthenticationECRBareHostname() {
	buildOptions := suite.newBuildOptions()

	jobSpec, err := suite.kaniko.compileJobSpec(context.Background(), "default", buildOptions, "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	createRepos := createReposContainer(podSpec.InitContainers)
	suite.Require().NotNil(createRepos, "expected an ECR create-repos init container for a bare ECR hostname")
	suite.Contains(createRepos.Args[1], "--region us-east-1 --registry-id 123456789012")
}

func (suite *KanikoTestSuite) TestConfigureRegistryAuthenticationNonECRHostWithSecret() {
	buildOptions := suite.newBuildOptions()
	buildOptions.RegistryURL = "myregistry.example.com"
	buildOptions.SecretName = "my-registry-secret"

	jobSpec, err := suite.kaniko.compileJobSpec(context.Background(), "default", buildOptions, "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	suite.Nil(createReposContainer(podSpec.InitContainers), "non-ECR host must not get an ECR create-repos init container")

	suite.Require().Len(podSpec.Containers[0].VolumeMounts, 2)
	authMount := podSpec.Containers[0].VolumeMounts[1]
	suite.Equal(registryhelpers.AuthVolumeName, authMount.Name)
}

func (suite *KanikoTestSuite) TestConfigureRegistryAuthenticationNonECRHostWithPathSuffix() {
	buildOptions := suite.newBuildOptions()
	buildOptions.RegistryURL = "registry.example.com/team/project"
	buildOptions.SecretName = "my-registry-secret"

	jobSpec, err := suite.kaniko.compileJobSpec(context.Background(), "default", buildOptions, "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	suite.Nil(createReposContainer(podSpec.InitContainers),
		"a non-AWS host with a path suffix must not be misdetected as ECR")
}

func (suite *KanikoTestSuite) TestNewContainerBuilderConfigurationParsesKanikoPodLabels() {
	for _, testCase := range []struct {
		name      string
		envValue  string
		expected  map[string]string
		expectErr bool
	}{
		{
			name:     "Unset",
			envValue: "",
			expected: nil,
		},
		{
			name:     "SingleLabel",
			envValue: `{"azure.workload.identity/use":"true"}`,
			expected: map[string]string{"azure.workload.identity/use": "true"},
		},
		{
			name:     "MultipleLabels",
			envValue: `{"a":"1","b":"2"}`,
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:      "InvalidJSON",
			envValue:  "not-json",
			expectErr: true,
		},
	} {
		suite.Run(testCase.name, func() {
			if testCase.envValue != "" {
				suite.T().Setenv("NUCLIO_KANIKO_POD_LABELS", testCase.envValue)
			}

			config, err := NewContainerBuilderConfiguration(nil)
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.PodLabels)
		})
	}
}

func TestKanikoTestSuite(t *testing.T) {
	suite.Run(t, new(KanikoTestSuite))
}
