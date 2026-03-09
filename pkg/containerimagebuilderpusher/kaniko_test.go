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
	"testing"

	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KanikoTestSuite struct {
	suite.Suite
	kaniko *Kaniko
}

func (suite *KanikoTestSuite) SetupTest() {
	suite.kaniko = &Kaniko{
		builderConfiguration: &ContainerBuilderConfiguration{
			BusyBoxImage:          "busybox:stable",
			KanikoImagePullPolicy: "IfNotPresent",
		},
	}
}

func (suite *KanikoTestSuite) TestResolveAWSRegistryId() {
	for _, testCase := range []struct {
		name        string
		registryURL string
		expected    string
	}{
		{
			name:        "StandardECRURL",
			registryURL: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expected:    "123456789012",
		},
		{
			name:        "DifferentRegion",
			registryURL: "987654321098.dkr.ecr.eu-west-1.amazonaws.com",
			expected:    "987654321098",
		},
		{
			name:        "APRegion",
			registryURL: "111222333444.dkr.ecr.ap-southeast-1.amazonaws.com",
			expected:    "111222333444",
		},
	} {
		suite.Run(testCase.name, func() {
			result := suite.kaniko.resolveAWSRegistryId(testCase.registryURL)
			suite.Require().Equal(testCase.expected, result)
		})
	}
}

func (suite *KanikoTestSuite) TestResolveAWSRegionFromECR() {
	for _, testCase := range []struct {
		name        string
		registryURL string
		expected    string
	}{
		{
			name:        "USEast1",
			registryURL: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expected:    "us-east-1",
		},
		{
			name:        "EUWest1",
			registryURL: "987654321098.dkr.ecr.eu-west-1.amazonaws.com",
			expected:    "eu-west-1",
		},
		{
			name:        "APSoutheast1",
			registryURL: "111222333444.dkr.ecr.ap-southeast-1.amazonaws.com",
			expected:    "ap-southeast-1",
		},
	} {
		suite.Run(testCase.name, func() {
			result := suite.kaniko.resolveAWSRegionFromECR(testCase.registryURL)
			suite.Require().Equal(testCase.expected, result)
		})
	}
}

func (suite *KanikoTestSuite) TestConfigureSecretVolumeMount() {
	for _, testCase := range []struct {
		name                       string
		registryURL                string
		secretName                 string
		registryProviderSecretName string
		expectedInitContainerCount int
		expectedVolumeCount        int
		expectedVolumeMountCount   int
		verifyFunc                 func(podSpec v1.PodSpec)
	}{
		{
			name:                       "ACRWithSecret",
			registryURL:                "myregistry.azurecr.io",
			registryProviderSecretName: "acr-credentials",
			expectedInitContainerCount: 3,
			expectedVolumeCount:        2,
			expectedVolumeMountCount:   2,
			verifyFunc: func(podSpec v1.PodSpec) {
				// Verify emptyDir volume for docker config
				suite.Require().Equal("docker-config", podSpec.Volumes[1].Name)
				suite.Require().NotNil(podSpec.Volumes[1].EmptyDir)

				// Verify setup-acr-config init container with credHelpers
				acrInitContainer := podSpec.InitContainers[2]
				suite.Require().Equal("setup-acr-config", acrInitContainer.Name)
				suite.Require().Contains(acrInitContainer.Args[1], `{"credHelpers":{"myregistry.azurecr.io":"acr-env"}}`)
				suite.Require().Equal("/kaniko/.docker", acrInitContainer.VolumeMounts[0].MountPath)

				// Verify docker config mount on kaniko executor
				suite.Require().Equal("/kaniko/.docker", podSpec.Containers[0].VolumeMounts[1].MountPath)

				// Verify envFrom with secret reference
				suite.Require().Len(podSpec.Containers[0].EnvFrom, 1)
				suite.Require().Equal("acr-credentials", podSpec.Containers[0].EnvFrom[0].SecretRef.Name)
			},
		},
		{
			name:                       "ACRWithoutSecret",
			registryURL:                "myregistry.azurecr.io",
			registryProviderSecretName: "",
			expectedInitContainerCount: 3,
			expectedVolumeCount:        2,
			expectedVolumeMountCount:   2,
			verifyFunc: func(podSpec v1.PodSpec) {
				// Verify init container and volume are still created
				suite.Require().Equal("setup-acr-config", podSpec.InitContainers[2].Name)
				suite.Require().Contains(podSpec.InitContainers[2].Args[1], `"acr-env"`)

				// Verify no envFrom (managed identity fallback)
				suite.Require().Empty(podSpec.Containers[0].EnvFrom)
			},
		},
		{
			name:                       "DockerRegistrySecret",
			registryURL:                "docker.io",
			secretName:                 "my-docker-creds",
			registryProviderSecretName: "",
			expectedInitContainerCount: 2,
			expectedVolumeCount:        2,
			expectedVolumeMountCount:   2,
			verifyFunc: func(podSpec v1.PodSpec) {
				// Verify secret volume with dockerconfigjson mapping
				suite.Require().NotNil(podSpec.Volumes[1].Secret)
				suite.Require().Equal("my-docker-creds", podSpec.Volumes[1].Secret.SecretName)
				suite.Require().Equal(".dockerconfigjson", podSpec.Volumes[1].VolumeSource.Secret.Items[0].Key)
				suite.Require().Equal("config.json", podSpec.Volumes[1].VolumeSource.Secret.Items[0].Path)

				// Verify read-only mount
				suite.Require().True(podSpec.Containers[0].VolumeMounts[1].ReadOnly)
			},
		},
		{
			name:                       "NoRegistrySecret",
			registryURL:                "docker.io",
			secretName:                 "",
			registryProviderSecretName: "",
			expectedInitContainerCount: 2,
			expectedVolumeCount:        1,
			expectedVolumeMountCount:   1,
			verifyFunc:                 nil,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.kaniko.builderConfiguration.RegistryProviderSecretName = testCase.registryProviderSecretName
			buildOptions := &BuildOptions{
				RegistryURL: testCase.registryURL,
				SecretName:  testCase.secretName,
			}
			jobSpec := suite.createBaseJobSpec()

			suite.kaniko.configureSecretVolumeMount(buildOptions, jobSpec)

			podSpec := jobSpec.Spec.Template.Spec
			suite.Require().Len(podSpec.InitContainers, testCase.expectedInitContainerCount)
			suite.Require().Len(podSpec.Volumes, testCase.expectedVolumeCount)
			suite.Require().Len(podSpec.Containers[0].VolumeMounts, testCase.expectedVolumeMountCount)

			if testCase.verifyFunc != nil {
				testCase.verifyFunc(podSpec)
			}
		})
	}
}

func (suite *KanikoTestSuite) createBaseJobSpec() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "test-ns",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "kaniko-executor",
							Image: "gcr.io/kaniko-project/executor:latest",
							VolumeMounts: []v1.VolumeMount{
								{Name: "tmp", MountPath: "/tmp"},
							},
						},
					},
					InitContainers: []v1.Container{
						{Name: "fetch-bundle", Image: "busybox:stable"},
						{Name: "extract-bundle", Image: "busybox:stable"},
					},
					Volumes: []v1.Volume{
						{
							Name: "tmp",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func TestKanikoTestSuite(t *testing.T) {
	suite.Run(t, new(KanikoTestSuite))
}
