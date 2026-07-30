//go:build test_integration && test_local

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
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

// mergeTestPythonImage mirrors the NUCLIO_PYTHON_BASE_IMAGE_NAME default in types.go.
const mergeTestPythonImage = "gcr.io/iguazio/python:3.11"

// MergeIntegrationTestSuite runs the real merge-authfile container spec against a Docker container,
// verifying both the spec and the embedded script's behavior end to end.
type MergeIntegrationTestSuite struct {
	suite.Suite
	dockerClient dockerclient.Client
}

func (suite *MergeIntegrationTestSuite) SetupSuite() {
	testLogger, err := nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.dockerClient, err = dockerclient.NewShellClient(testLogger, nil)
	suite.Require().NoError(err, "Docker must be reachable to run this suite")
}

// hostScratchDir returns a fresh temp dir under $HOME, not the OS temp dir: Docker-in-a-VM usually
// only bind-mounts $HOME, so a volume outside it silently mounts empty.
func (suite *MergeIntegrationTestSuite) hostScratchDir() string {
	homeDir, err := os.UserHomeDir()
	suite.Require().NoError(err)

	baseDir := filepath.Join(homeDir, ".nuclio-test-tmp")
	suite.Require().NoError(os.MkdirAll(baseDir, 0o755))

	dir, err := os.MkdirTemp(baseDir, "merge-authfile-")
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		_ = os.RemoveAll(dir) // nolint: errcheck
	})
	return dir
}

// runMerge writes secretFiles/tokenFiles to host temp dirs and runs the real merge-authfile container
// spec via docker, returning the resulting authfile contents and the container's output.
func (suite *MergeIntegrationTestSuite) runMerge(secretFiles, tokenFiles map[string]string) (string, string) {
	secretsHostDir := suite.hostScratchDir()
	authHostDir := suite.hostScratchDir()
	scriptHostDir := suite.hostScratchDir()

	for name, contents := range secretFiles {
		suite.Require().NoError(os.WriteFile(filepath.Join(secretsHostDir, name), []byte(contents), 0o644))
	}

	withCloudTokens := len(tokenFiles) > 0
	tokensHostDir := ""
	if withCloudTokens {
		tokensHostDir = suite.hostScratchDir()
		for name, contents := range tokenFiles {
			suite.Require().NoError(os.WriteFile(filepath.Join(tokensHostDir, name), []byte(contents), 0o644))
		}
	}

	suite.Require().NoError(os.WriteFile(
		filepath.Join(scriptHostDir, authScriptFileName), []byte(MergeScriptContents()), 0o644))

	secretNames := make([]string, len(secretFiles))
	container, _ := BuildMergeAuthInitContainer(secretNames, "/authdir", withCloudTokens,
		AuthConfig{PythonImage: mergeTestPythonImage})

	volumes := map[string]string{
		authHostDir:    "/authdir",
		secretsHostDir: authSourcesMountPath,
		scriptHostDir:  authScriptMountPath,
	}
	if withCloudTokens {
		volumes[tokensHostDir] = TokenDirVolumeMount().MountPath
	}

	command := strings.Join(append(container.Command, container.Args...), " ")

	var output string
	_, err := suite.dockerClient.RunContainer(mergeTestPythonImage, &dockerclient.RunOptions{
		Command:          command,
		Volumes:          volumes,
		Remove:           true,
		Attach:           true,
		ImageMayNotExist: true,
		Stdout:           &output,
	})
	suite.Require().NoError(err, "container output: %s", output)

	authfileContents, err := os.ReadFile(filepath.Join(authHostDir, "config.json"))
	suite.Require().NoError(err)

	return string(authfileContents), output
}

func (suite *MergeIntegrationTestSuite) readAuths(authfileContents string) map[string]map[string]string {
	var doc struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(authfileContents), &doc))

	result := map[string]map[string]string{}
	for host, entry := range doc.Auths {
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		suite.Require().NoError(err)
		result[host] = map[string]string{"auth": string(decoded)}
	}
	return result
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesSecretsOnly() {
	authfile, _ := suite.runMerge(map[string]string{
		"0.json": `{"auths":{"registry-a.io":{"auth":"YQ=="}}}`,
		"1.json": `{"auths":{"registry-b.io":{"auth":"Yg=="}}}`,
	}, nil)

	auths := suite.readAuths(authfile)
	suite.Contains(auths, "registry-a.io")
	suite.Contains(auths, "registry-b.io")
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesTokensOnly() {
	authfile, output := suite.runMerge(nil, map[string]string{
		"0.token": "myregistry.azurecr.io\n00000000-0000-0000-0000-000000000000\nsome-token\n",
	})

	suite.NotContains(output, "some-token")

	auths := suite.readAuths(authfile)
	suite.Equal("00000000-0000-0000-0000-000000000000:some-token", auths["myregistry.azurecr.io"]["auth"])
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesTokenWinsOverSecretForSameHost() {
	authfile, _ := suite.runMerge(
		map[string]string{"0.json": `{"auths":{"shared.io":{"auth":"c3RhbGU6c3RhbGU="}}}`}, // stale:stale
		map[string]string{"0.token": "shared.io\nfreshuser\nfreshtoken\n"},
	)

	auths := suite.readAuths(authfile)
	suite.Equal("freshuser:freshtoken", auths["shared.io"]["auth"])
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesTokenOrdering() {
	authfile, _ := suite.runMerge(nil, map[string]string{
		"0.token": "host.io\nuser0\ntoken0\n",
		"1.token": "host.io\nuser1\ntoken1\n",
	})

	auths := suite.readAuths(authfile)
	suite.Equal("user1:token1", auths["host.io"]["auth"])
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesMalformedTokenSkipped() {
	authfile, _ := suite.runMerge(nil, map[string]string{
		"0.token": "onlyonelie",
		"1.token": "host.io\nuser\n\n", // empty token
	})

	auths := suite.readAuths(authfile)
	suite.Empty(auths)
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesPassesThroughNonAuthsKeys() {
	authfile, _ := suite.runMerge(map[string]string{
		"0.json": `{"auths":{"registry-a.io":{"auth":"YQ=="}},"credsStore":"ecr-login"}`,
	}, nil)

	var doc map[string]interface{}
	suite.Require().NoError(json.Unmarshal([]byte(authfile), &doc))
	suite.Equal("ecr-login", doc["credsStore"])
}

func (suite *MergeIntegrationTestSuite) TestMergeAuthFilesMissingSecrets() {
	authfile, _ := suite.runMerge(nil, nil)
	suite.JSONEq(`{"auths":{}}`, authfile)
}

func TestMergeIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(MergeIntegrationTestSuite))
}
