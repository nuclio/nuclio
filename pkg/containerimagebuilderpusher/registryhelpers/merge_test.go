//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package registryhelpers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

// MergeTestSuite covers the container/volume spec built by BuildMergeAuthInitContainer; the script's
// merging behavior is covered by MergeIntegrationTestSuite, which runs the real container.
type MergeTestSuite struct {
	suite.Suite
}

func (suite *MergeTestSuite) TestBuildMergeAuthInitContainerImageAndCommand() {
	container, _ := BuildMergeAuthInitContainer(
		[]string{"secret-a", "secret-b"},
		"/tmp/registry-auth",
		true,
		AuthConfig{PythonImage: "python:3.11", PythonImagePullPolicy: "Always"})

	suite.Equal("merge-authfile", container.Name)
	suite.Equal("python:3.11", container.Image)
	suite.Equal("Always", string(container.ImagePullPolicy))
	suite.Equal([]string{"python3", authScriptMountPath + "/" + authScriptFileName}, container.Command)
}

func (suite *MergeTestSuite) TestBuildMergeAuthInitContainerArgsWithCloudTokens() {
	container, _ := BuildMergeAuthInitContainer(
		[]string{"secret-a"},
		"/tmp/registry-auth",
		true,
		AuthConfig{})

	suite.Equal([]string{
		fmt.Sprintf("--secrets=%s", authSourcesMountPath),
		"--target=/tmp/registry-auth/config.json",
		fmt.Sprintf("--cloud-tokens=%s", TokenDirVolumeMount().MountPath),
	}, container.Args)
}

func (suite *MergeTestSuite) TestBuildMergeAuthInitContainerArgsWithoutCloudTokens() {
	container, _ := BuildMergeAuthInitContainer(
		[]string{"secret-a"},
		"/tmp/registry-auth",
		false,
		AuthConfig{})

	suite.Equal([]string{
		fmt.Sprintf("--secrets=%s", authSourcesMountPath),
		"--target=/tmp/registry-auth/config.json",
	}, container.Args)
}

func (suite *MergeTestSuite) TestBuildMergeAuthInitContainerVolumesAndMounts() {
	container, volumes := BuildMergeAuthInitContainer(
		[]string{"secret-a", "secret-b"},
		"/tmp/registry-auth",
		true,
		AuthConfig{})

	// volumes: projected secrets + script configmap
	suite.Require().Len(volumes, 2)

	sourcesVolume := volumes[0]
	suite.Equal(authSourcesVolumeName, sourcesVolume.Name)
	suite.Require().NotNil(sourcesVolume.Projected)
	suite.Require().Len(sourcesVolume.Projected.Sources, 2)
	suite.Equal("secret-a", sourcesVolume.Projected.Sources[0].Secret.Name)
	suite.Equal("0.json", sourcesVolume.Projected.Sources[0].Secret.Items[0].Path)
	suite.Equal("secret-b", sourcesVolume.Projected.Sources[1].Secret.Name)
	suite.Equal("1.json", sourcesVolume.Projected.Sources[1].Secret.Items[0].Path)

	scriptVolume := volumes[1]
	suite.Equal(authScriptVolumeName, scriptVolume.Name)
	suite.Require().NotNil(scriptVolume.ConfigMap)
	suite.Equal(MergeScriptConfigMapName, scriptVolume.ConfigMap.Name)

	// mounts: authfile (writable), sources (ro), script (ro), tokens (since withCloudTokens=true)
	suite.Require().Len(container.VolumeMounts, 4)
	suite.Equal(AuthVolumeName, container.VolumeMounts[0].Name)
	suite.Equal("/tmp/registry-auth", container.VolumeMounts[0].MountPath)
	suite.False(container.VolumeMounts[0].ReadOnly)

	suite.Equal(authSourcesVolumeName, container.VolumeMounts[1].Name)
	suite.True(container.VolumeMounts[1].ReadOnly)

	suite.Equal(authScriptVolumeName, container.VolumeMounts[2].Name)
	suite.True(container.VolumeMounts[2].ReadOnly)

	suite.Equal(TokenDirVolumeMount().Name, container.VolumeMounts[3].Name)
}

func (suite *MergeTestSuite) TestBuildMergeAuthInitContainerNoTokenMountWithoutCloudTokens() {
	container, _ := BuildMergeAuthInitContainer(
		[]string{"secret-a"},
		"/tmp/registry-auth",
		false,
		AuthConfig{})

	suite.Require().Len(container.VolumeMounts, 3)
	for _, mount := range container.VolumeMounts {
		suite.NotEqual(TokenDirVolumeMount().Name, mount.Name)
	}
}

func (suite *MergeTestSuite) TestMergeScriptContentsIsTheEmbeddedScript() {
	suite.Contains(MergeScriptContents(), "def merge_auth_files")
}

func TestMergeTestSuite(t *testing.T) {
	suite.Run(t, new(MergeTestSuite))
}
