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

package dockercreds

import (
	"io/ioutil"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
)

// DockerCreds initializes docker client credentials
type DockerCreds struct {
	logger       nuclio.Logger
	dockerClient dockerclient.Client
}

func NewDockerCreds(parentLogger nuclio.Logger,
	dockerClient dockerclient.Client) (*DockerCreds, error) {

	return &DockerCreds{
		logger:       parentLogger.GetChild("loginner"),
		dockerClient: dockerClient,
	}, nil
}

func (dc *DockerCreds) LoadFromDir(keyDir string) error {
	dockerKeyFileInfos, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return errors.Wrap(err, "Failed to read docker key directory")
	}

	for _, dockerKeyFileInfo := range dockerKeyFileInfos {
		if dockerKeyFileInfo.IsDir() {
			continue
		}

		dockerKeyFileName := dockerKeyFileInfo.Name()
		dockerKeyFilePath := path.Join(keyDir, dockerKeyFileName)

		password, err := ioutil.ReadFile(dockerKeyFilePath)
		if err != nil {
			dc.logger.WarnWith("Failed to read docker key file",
				"err", err.Error(),
				"path", dockerKeyFileInfo.Name())

			continue
		}

		// get the URL and username
		username, url, err := dc.getUserAndURLFromKeyPath(dockerKeyFileName)
		if err != nil {
			dc.logger.WarnWith("Failed to get user / url from path",
				"err", err.Error(),
				"path", dockerKeyFileInfo.Name())

			continue
		}

		dc.logger.InfoWith("Logging in to registry",
			"path", dockerKeyFilePath,
			"username", username,
			"passwordLen", len(password),
			"url", url)

		// try to login
		err = dc.dockerClient.LogIn(&dockerclient.LogInOptions{
			Username: username,
			Password: string(password),
			URL:      "https://" + url,
		})

		if err != nil {
			dc.logger.WarnWith("Failed to log in to docker", "err", err.Error())
			continue
		}
	}

	return nil
}

func (dc *DockerCreds) getUserAndURLFromKeyPath(keyPath string) (string, string, error) {
	dockerKeyBase := path.Base(keyPath)
	dockerKeyExt := path.Ext(dockerKeyBase)

	// get just the file name
	dockerKeyFileWithoutExt := dockerKeyBase[:len(dockerKeyBase)-len(dockerKeyExt)]

	// expect user@url in the ext
	userAndURL := strings.Split(dockerKeyFileWithoutExt, "---")
	if len(userAndURL) != 2 {
		return "", "", errors.New("Expected file name to contain user---url")
	}

	if len(userAndURL[0]) == 0 {
		return "", "", errors.New("Username is empty")
	}

	if len(userAndURL[1]) == 0 {
		return "", "", errors.New("URL is empty")
	}

	// return the user and URL
	return userAndURL[0], userAndURL[1], nil
}
